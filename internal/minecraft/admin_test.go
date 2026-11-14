package minecraft

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type adminRoute struct {
	method, path, body string
	// what the backend should see
	backendPath, backendBody string
}

var adminRoutes = []adminRoute{
	{http.MethodGet, "/api/minecraft/admin/players", "", "/server/allowlist", ""},
	{http.MethodPost, "/api/minecraft/admin/players", `{"name":"Steve_1","edition":"java","operation":"add","world":"creative","x":1}`,
		"/server/allowlist", `{"operation":"add","edition":"java","name":"Steve_1"}`},
	{http.MethodGet, "/api/minecraft/admin/archives?world=creative", "", "/server/archives", ""},
	{http.MethodPost, "/api/minecraft/admin/restore", `{"archive":"20261016T031500Z.tar.gz","world":"creative","force":true}`,
		"/server/restore", `{"world":"creative","archive":"20261016T031500Z.tar.gz"}`},
	{http.MethodGet, "/api/minecraft/admin/operators", "", "/server/operators", ""},
	{http.MethodPost, "/api/minecraft/admin/operators", `{"world":"creative","operation":"remove","edition":"bedrock","name":"Blocky Kid 7"}`,
		"/server/operators", `{"world":"creative","operation":"remove","edition":"bedrock","name":"Blocky Kid 7"}`},
}

func adminCall(t *testing.T, base string, r adminRoute) (*http.Response, string) {
	t.Helper()
	if r.method == http.MethodPost {
		return post(t, base+r.path, r.body)
	}
	return call(t, r.method, base+r.path, nil)
}

// Only minecraft-admin reaches the players, archives and operators routes.
func TestOnlyAdminsReachTheAdminRoutes(t *testing.T) {
	for _, c := range []struct {
		name   string
		grants []string
		status int
		code   string
	}{
		{"signed out", nil, http.StatusUnauthorized, "signed_out"},
		{"launcher only", []string{Grant}, http.StatusForbidden, "no_access"},
	} {
		site, calls := modsSite(t, c.grants, modsBackend)
		for _, r := range adminRoutes {
			res, body := adminCall(t, site.URL, r)
			if res.StatusCode != c.status || code(t, body) != c.code {
				t.Errorf("%s, %s %s: %d %s", c.name, r.method, r.path, res.StatusCode, body)
			}
		}
		if len(*calls) != 0 {
			t.Errorf("%s: backend was asked: %+v", c.name, *calls)
		}
	}
}

// Each route asks the backend its own question with a body the site built,
// names no Member, and passes the answer on.
func TestAdminRoutesReachTheBackend(t *testing.T) {
	const answer = `{"players":[{"edition":"java","name":"Steve_1"}]}`
	var query string
	site, calls := modsSite(t, []string{AdminGrant}, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
	})
	for i, r := range adminRoutes {
		res, body := adminCall(t, site.URL, r)
		if res.StatusCode != http.StatusOK || body != answer {
			t.Fatalf("%s %s: %d %s", r.method, r.path, res.StatusCode, body)
		}
		got := (*calls)[i]
		if got.method != r.method || got.path != r.backendPath || got.body != r.backendBody || got.member != "" {
			t.Errorf("%s %s: backend saw %+v", r.method, r.path, got)
		}
		if r.backendPath == "/server/archives" && query != "world=creative" {
			t.Errorf("archives query: %q", query)
		}
	}
}

// A change that isn't one is refused by the site before the backend, and so
// is one from another site.
func TestBadAdminRequestsAreRefusedBeforeTheBackend(t *testing.T) {
	site, calls := modsSite(t, []string{AdminGrant}, modsBackend)
	for path, bodies := range map[string][]string{
		"/api/minecraft/admin/players": {
			``, `[]`,
			`{"operation":"op","edition":"java","name":"Steve"}`,
			`{"operation":"add","edition":"xbox","name":"Steve"}`,
			`{"operation":"add","edition":"java","name":"a b"}`,
			`{"operation":"add","edition":"java","name":"ab"}`,
			`{"operation":"add","edition":"bedrock","name":" padded"}`,
			`{"operation":"add","edition":"bedrock","name":"semi;colon"}`,
			`{"operation":"add","edition":"bedrock","name":"` + strings.Repeat("x", 17) + `"}`,
		},
		"/api/minecraft/admin/restore": {
			`{"world":"creative"}`,
			`{"world":"Creative","archive":"20261016T031500Z.tar.gz"}`,
			`{"world":"creative","archive":"../20261016T031500Z.tar.gz"}`,
			`{"world":"creative","archive":"s3://bucket/20261016T031500Z.tar.gz"}`,
		},
		"/api/minecraft/admin/operators": {
			`{"operation":"add","edition":"java","name":"Steve"}`,
			`{"world":"../x","operation":"add","edition":"java","name":"Steve"}`,
			`{"world":"creative","operation":"add","edition":"java","name":"op @a"}`,
		},
	} {
		for _, body := range bodies {
			res, answer := post(t, site.URL+path, body)
			if res.StatusCode != http.StatusBadRequest || code(t, answer) != "bad_request" {
				t.Errorf("%s %.50q: %d %s, want 400 bad_request", path, body, res.StatusCode, answer)
			}
		}
	}
	for _, q := range []string{"", "?world=", "?world=Creative", "?world=a%26b"} {
		res, answer := call(t, http.MethodGet, site.URL+"/api/minecraft/admin/archives"+q, nil)
		if res.StatusCode != http.StatusBadRequest || code(t, answer) != "bad_request" {
			t.Errorf("archives%s: %d %s", q, res.StatusCode, answer)
		}
	}
	for _, r := range adminRoutes {
		if r.method != http.MethodPost {
			continue
		}
		req, _ := http.NewRequest(http.MethodPost, site.URL+r.path, strings.NewReader(r.body))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("cross-site %s: %d", r.path, res.StatusCode)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("backend was asked: %+v", *calls)
	}
}

// The backend's refusals come through as their reason; its trouble doesn't.
func TestAdminRefusalsComeThrough(t *testing.T) {
	for _, c := range []struct {
		status     int
		answer     string
		wantStatus int
		wantCode   string
	}{
		{http.StatusNotFound, `{"code":"player_not_found"}`, http.StatusNotFound, "player_not_found"},
		{http.StatusConflict, `{"code":"linked_account"}`, http.StatusConflict, "linked_account"},
		{http.StatusBadGateway, `{"code":"allowlist_failed"}`, http.StatusServiceUnavailable, "tool_unavailable"},
	} {
		site, _ := modsSite(t, []string{AdminGrant}, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.answer)
		})
		res, body := post(t, site.URL+"/api/minecraft/admin/players", `{"operation":"add","edition":"java","name":"Steve"}`)
		if res.StatusCode != c.wantStatus || code(t, body) != c.wantCode {
			t.Errorf("%d %s: got %d %s", c.status, c.answer, res.StatusCode, body)
		}
	}
}

// A change to the allowlist or to operators is given the patient client's
// time; everything else keeps the ordinary one's.
func TestSlowChangesGetThePatientClient(t *testing.T) {
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{
		Backend: be.URL,
		Check: func(http.ResponseWriter, *http.Request, ...string) (Access, string, error) {
			return Granted, "craig@example.com", nil
		},
		Client:  &http.Client{Timeout: 20 * time.Millisecond},
		Patient: &http.Client{Timeout: 5 * time.Second},
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r := chi.NewRouter()
	tool.Routes(r)
	site := httptest.NewServer(r)
	t.Cleanup(site.Close)
	for _, c := range []struct {
		path, body string
		want       int
	}{
		{PlayersPath, `{"operation":"add","edition":"java","name":"Steve"}`, http.StatusOK},
		{OperatorsPath, `{"world":"creative","operation":"add","edition":"java","name":"Steve"}`, http.StatusOK},
		{"/api/minecraft/admin/restore", `{"world":"creative","archive":"20261016T031500Z.tar.gz"}`, http.StatusServiceUnavailable},
	} {
		if res, body := post(t, site.URL+c.path, c.body); res.StatusCode != c.want {
			t.Errorf("%s: %d %s, want %d", c.path, res.StatusCode, body, c.want)
		}
	}
}
