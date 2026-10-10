package soccer

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/go-chi/chi/v5"
)

// seen is one request the fake backend received.
type seen struct {
	method, path, query, body string
	header                    http.Header
}

// newSite serves the Tool's routes in front of a fake backend.
func newSite(t *testing.T, backend http.HandlerFunc, configure func(*Tool)) (*httptest.Server, *[]seen) {
	t.Helper()
	var calls []seen
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, seen{r.Method, r.URL.Path, r.URL.RawQuery, string(b), r.Header.Clone()})
		backend(w, r)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{Backend: be.URL, BaseURL: "https://lobby.example", Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if configure != nil {
		configure(tool)
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	return site, &calls
}

func do(t *testing.T, method, url, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func errorCode(t *testing.T, body string) string {
	t.Helper()
	var e struct{ Code string }
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("not a JSON error: %q", body)
	}
	return e.Code
}

func TestLookupsAreForwarded(t *testing.T) {
	site, calls := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"teams":[]}`))
	}, nil)

	for _, c := range []struct{ get, wantPath, wantQuery string }{
		{"/api/soccer/teams?q=boise+comets&x=1", "/teams", "q=boise+comets"},
		{"/api/soccer/teams/123456", "/teams/123456", ""},
		{"/api/soccer/divisions", "/divisions", ""},
		{"/api/soccer/divisions/987/teams", "/divisions/987/teams", ""},
	} {
		res, body := do(t, http.MethodGet, site.URL+c.get, "")
		if res.StatusCode != http.StatusOK || body != `{"teams":[]}` {
			t.Fatalf("%s: %d %q", c.get, res.StatusCode, body)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: lookups must not be cached, got %q", c.get, res.Header.Get("Cache-Control"))
		}
		got := (*calls)[len(*calls)-1]
		if got.path != c.wantPath || got.query != c.wantQuery {
			t.Fatalf("%s reached the backend as %s?%s", c.get, got.path, got.query)
		}
	}
}

func TestTeamIDMustBeANumber(t *testing.T) {
	site, calls := newSite(t, func(http.ResponseWriter, *http.Request) {}, nil)
	res, body := do(t, http.MethodGet, site.URL+"/api/soccer/teams/12ab", "")
	if res.StatusCode != http.StatusBadRequest || errorCode(t, body) != "invalid_team_id" {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
	if len(*calls) != 0 {
		t.Fatal("an unusable Team ID reached the backend")
	}
}

// The backend's own errors, including the league being down, come through
// with their status and code so the page can tell them apart.
func TestBackendErrorsComeThrough(t *testing.T) {
	site, _ := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"league_unavailable","message":"x"}`))
	}, nil)
	res, body := do(t, http.MethodGet, site.URL+"/api/soccer/teams/123456", "")
	if res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "league_unavailable" {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
}

func TestUnreachableBackendIsNotALeagueOutage(t *testing.T) {
	for name, configure := range map[string]func(*Tool){
		"not connected": func(tool *Tool) { tool.Backend = "" },
		"not listening": func(tool *Tool) { tool.Backend = "http://127.0.0.1:1" },
	} {
		site, _ := newSite(t, func(http.ResponseWriter, *http.Request) {}, configure)
		res, body := do(t, http.MethodGet, site.URL+"/api/soccer/divisions", "")
		if res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "tool_unavailable" {
			t.Fatalf("%s: %d %q", name, res.StatusCode, body)
		}
		res, _ = do(t, http.MethodGet, site.URL+"/soccer/link/abc.ics", "")
		if res.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s: a calendar app must get 503, not %d, so it keeps its last copy", name, res.StatusCode)
		}
	}
}

// AWS answers 403 when it refuses the site's signature; that is the Tool
// being unavailable, never something to show a Visitor as it is.
func TestRefusedSignatureIsUnavailable(t *testing.T) {
	site, _ := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Message":"Forbidden"}`))
	}, nil)
	res, body := do(t, http.MethodGet, site.URL+"/api/soccer/divisions", "")
	if res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "tool_unavailable" {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
}

func TestDownloadPassesTheChoiceAndTheFile(t *testing.T) {
	site, calls := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="comets.ics"`)
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
	}, nil)
	choice := `{"team_ids":["123456"],"excluded":["g2"],"since":"2000-01-01T00:00:00Z"}`
	res, body := do(t, http.MethodPost, site.URL+"/api/soccer/calendar", choice)
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(body, "BEGIN:VCALENDAR") {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
	if got := res.Header.Get("Content-Disposition"); got != `attachment; filename="comets.ics"` {
		t.Fatalf("Content-Disposition %q", got)
	}
	if got := (*calls)[0]; got.method != http.MethodPost || got.path != "/calendar" || got.body != choice {
		t.Fatalf("backend saw %s %s %q", got.method, got.path, got.body)
	}
}

// A Session link follows every game for the chosen Teams. Whatever the page
// sends besides the Teams is refused, so download exclusions cannot narrow it.
func TestSessionLinkCoversAllGamesForTheTeams(t *testing.T) {
	site, calls := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"dG9rZW4","url":"http://elsewhere/dG9rZW4.ics","teams":[{"id":"123456","name":"Boise Comets"}],"failed":[]}`))
	}, nil)

	res, body := do(t, http.MethodPost, site.URL+"/api/soccer/links", `{"team_ids":["123456","654321"]}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
	var out struct {
		URL   string
		Teams []struct{ Name string }
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if out.URL != "https://lobby.example/soccer/link/dG9rZW4.ics" {
		t.Fatalf("link %q is not on the site's own address", out.URL)
	}
	if len(out.Teams) != 1 || out.Teams[0].Name != "Boise Comets" {
		t.Fatalf("teams %v", out.Teams)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte((*calls)[0].body), &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 || sent["since"] != "2000-01-01T00:00:00Z" {
		t.Fatalf("backend was sent %v; want only the Teams and a since before any Session", sent)
	}

	res, body = do(t, http.MethodPost, site.URL+"/api/soccer/links", `{"team_ids":["123456"],"excluded":["g2"]}`)
	if res.StatusCode != http.StatusBadRequest || errorCode(t, body) != "bad_request" {
		t.Fatalf("a link request carrying exclusions: %d %q", res.StatusCode, body)
	}
	if len(*calls) != 1 {
		t.Fatal("a refused link request reached the backend")
	}
}

func TestSessionLinkFailureComesThrough(t *testing.T) {
	site, _ := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"league_unavailable","message":"x"}`))
	}, nil)
	res, body := do(t, http.MethodPost, site.URL+"/api/soccer/links", `{"team_ids":["123456"]}`)
	if res.StatusCode != http.StatusServiceUnavailable || errorCode(t, body) != "league_unavailable" {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
}

func TestCalendarAppFetchesALink(t *testing.T) {
	status := http.StatusOK
	site, calls := newSite(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "private, max-age=900")
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
		}
	}, nil)

	req, _ := http.NewRequest(http.MethodGet, site.URL+"/soccer/link/dG9rZW4.ics", nil)
	req.Header.Set("User-Agent", "iOS/26.0 dataaccessd/1.0")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(string(b), "BEGIN:VCALENDAR") {
		t.Fatalf("%d %q", res.StatusCode, b)
	}
	if res.Header.Get("Content-Type") != "text/calendar; charset=utf-8" || res.Header.Get("Cache-Control") != "private, max-age=900" {
		t.Fatalf("headers %v", res.Header)
	}
	got := (*calls)[0]
	if got.path != "/links/dG9rZW4.ics" || got.header.Get("User-Agent") != "iOS/26.0 dataaccessd/1.0" {
		t.Fatalf("backend saw %s from %q", got.path, got.header.Get("User-Agent"))
	}

	res, _ = do(t, http.MethodHead, site.URL+"/soccer/link/dG9rZW4.ics", "")
	if res.StatusCode != http.StatusOK || (*calls)[1].method != http.MethodHead {
		t.Fatalf("HEAD: %d, backend saw %s", res.StatusCode, (*calls)[1].method)
	}

	// The league failing must reach the calendar app as a failure.
	status = http.StatusServiceUnavailable
	res, _ = do(t, http.MethodGet, site.URL+"/soccer/link/dG9rZW4.ics", "")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want the backend's 503", res.StatusCode)
	}

	res, body := do(t, http.MethodGet, site.URL+"/soccer/link/not%20a%20token", "")
	if res.StatusCode != http.StatusNotFound || errorCode(t, body) != "invalid_link" {
		t.Fatalf("%d %q", res.StatusCode, body)
	}
}

func TestRequestsToTheBackendAreSigned(t *testing.T) {
	creds := aws.CredentialsProvider(credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", ""))
	site, calls := newSite(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
	}, func(tool *Tool) { tool.Sign = SigV4(creds, "us-west-2") })

	do(t, http.MethodPost, site.URL+"/api/soccer/calendar", `{"team_ids":["123456"]}`)
	auth := (*calls)[0].header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/") || !strings.Contains(auth, "/us-west-2/lambda/aws4_request") {
		t.Fatalf("Authorization %q", auth)
	}
}

// A failed call to the backend is logged without the address it called, so
// a link's token and a search stay out of the logs.
func TestBackendFailuresAreLoggedWithoutTheAddress(t *testing.T) {
	var logged bytes.Buffer
	site, _ := newSite(t, func(http.ResponseWriter, *http.Request) {}, func(tool *Tool) {
		tool.Backend = "http://127.0.0.1:1"
		tool.Log = slog.New(slog.NewTextHandler(&logged, nil))
	})
	do(t, http.MethodGet, site.URL+"/soccer/link/c2VjcmV0dG9rZW4.ics", "")
	do(t, http.MethodGet, site.URL+"/api/soccer/teams?q=privatesearch", "")
	if logged.Len() == 0 {
		t.Fatal("the failures were not logged")
	}
	for _, secret := range []string{"c2VjcmV0dG9rZW4", "privatesearch"} {
		if strings.Contains(logged.String(), secret) {
			t.Fatalf("log carries %q: %s", secret, logged.String())
		}
	}
}
