package devices

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"mime"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/micromdm/plist"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/profile"
)

// fakeAccess stands in for the /admin application's verifier: each token it
// knows is one adult.
type fakeAccess map[string]string

func (f fakeAccess) Verify(_ context.Context, tok string) (access.Identity, error) {
	if e, ok := f[tok]; ok {
		return access.Identity{Email: e}, nil
	}
	return access.Identity{}, errors.New("bad token")
}

const (
	kidsDoH   = "https://kids1234.cloudflare-gateway.com/dns-query"
	adultsDoH = "https://adults567.cloudflare-gateway.com/dns-query"
)

var testUI = fstest.MapFS{"admin.html": {Data: []byte("<!doctype html>family devices")}}

var routes = []string{PagePath, FilesPath + profile.KidsFileName, FilesPath + profile.AdultFileName, StatusPath}

// testCertificate is a self-signed CA, standing in for the Gateway root.
func testCertificate(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Gateway CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// configured is a handler on a deployed environment with every setting set
// and one adult who may pass.
func configured(t *testing.T) *Handler {
	t.Helper()
	return &Handler{
		Env:    "dev",
		Access: fakeAccess{"good": "adult@example.com"},
		Kids:   profile.Kids{DoHURL: kidsDoH},
		Adult:  profile.Adult{DoHURL: adultsDoH, GatewayRootCA: testCertificate(t)},
		UI:     testUI,
		Log:    slog.Default(),
	}
}

func newServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	h.Routes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path, token string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if token != "" {
		req.Header.Set(access.Header, token)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res, body
}

// code reads the site's own error answer.
func code(t *testing.T, res *http.Response, body []byte) string {
	t.Helper()
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type %q, want application/json for an error answer", ct)
	}
	var e struct{ Code string }
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("error answer %q is not JSON: %v", body, err)
	}
	return e.Code
}

func TestEveryRouteNeedsTheAccessToken(t *testing.T) {
	srv := newServer(t, configured(t))
	for _, path := range routes {
		for name, tok := range map[string]string{"no token": "", "forged token": "forged"} {
			res, body := get(t, srv, path, tok)
			if res.StatusCode != http.StatusForbidden {
				t.Errorf("%s with %s: status %d, want 403", path, name, res.StatusCode)
				continue
			}
			if c := code(t, res, body); c != "forbidden" {
				t.Errorf("%s with %s: code %q", path, name, c)
			}
			if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
				t.Errorf("%s with %s: Cache-Control %q", path, name, cc)
			}
		}
	}
}

func TestThePageServesBehindTheToken(t *testing.T) {
	srv := newServer(t, configured(t))
	res, body := get(t, srv, PagePath, "good")
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "family devices") {
		t.Fatalf("page: %d %q", res.StatusCode, body)
	}
	if ct, cc := res.Header.Get("Content-Type"), res.Header.Get("Cache-Control"); ct != "text/html; charset=utf-8" || cc != "no-store" {
		t.Fatalf("page headers: Content-Type %q, Cache-Control %q", ct, cc)
	}

	// Without the built screens the page says so, and the files still serve.
	h := configured(t)
	h.UI = fstest.MapFS{}
	srv = newServer(t, h)
	if res, _ := get(t, srv, PagePath, "good"); res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("page without screens: %d", res.StatusCode)
	}
	if res, _ := get(t, srv, FilesPath+profile.KidsFileName, "good"); res.StatusCode != http.StatusOK {
		t.Fatalf("file without screens: %d", res.StatusCode)
	}
}

func TestProfilesDownloadWithTheRightHeaders(t *testing.T) {
	srv := newServer(t, configured(t))
	for _, tc := range []struct{ file, identifier, doh string }{
		{profile.KidsFileName, "com.craigdevjohnson.family.kids-ipad", kidsDoH},
		{profile.AdultFileName, "com.craigdevjohnson.family.adult-iphone", adultsDoH},
	} {
		res, body := get(t, srv, FilesPath+tc.file, "good")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d: %s", tc.file, res.StatusCode, body)
		}
		if ct := res.Header.Get("Content-Type"); ct != "application/x-apple-aspen-config" {
			t.Errorf("%s: Content-Type %q", tc.file, ct)
		}
		disposition, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
		if err != nil || disposition != "attachment" || params["filename"] != tc.file {
			t.Errorf("%s: Content-Disposition %q (%v)", tc.file, res.Header.Get("Content-Disposition"), err)
		}
		if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control %q", tc.file, cc)
		}
		if cl := res.Header.Get("Content-Length"); cl != strconv.Itoa(len(body)) {
			t.Errorf("%s: Content-Length %q for %d bytes", tc.file, cl, len(body))
		}
		var top map[string]any
		if err := plist.Unmarshal(body, &top); err != nil {
			t.Fatalf("%s: body is not a plist: %v", tc.file, err)
		}
		if top["PayloadType"] != "Configuration" || top["PayloadIdentifier"] != tc.identifier {
			t.Errorf("%s: PayloadType %v, PayloadIdentifier %v", tc.file, top["PayloadType"], top["PayloadIdentifier"])
		}
		if !strings.Contains(string(body), tc.doh) {
			t.Errorf("%s: does not carry its DNS over HTTPS endpoint", tc.file)
		}
	}

	if res, _ := get(t, srv, FilesPath+"other.mobileconfig", "good"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown profile: status %d, want 404", res.StatusCode)
	}
}

func TestStatusListsBothProfiles(t *testing.T) {
	srv := newServer(t, configured(t))
	res, body := get(t, srv, StatusPath, "good")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}
	var answer struct {
		Profiles []struct{ Device, File, Href string }
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatal(err)
	}
	if len(answer.Profiles) != 2 {
		t.Fatalf("profiles: %+v", answer.Profiles)
	}
	for _, p := range answer.Profiles {
		if p.Href != FilesPath+p.File || (p.Device != "kids" && p.Device != "adult") {
			t.Errorf("profile %+v", p)
		}
		if res, _ := get(t, srv, p.Href, "good"); res.StatusCode != http.StatusOK {
			t.Errorf("%s: the listed link answers %d", p.Href, res.StatusCode)
		}
	}
}

// An unset setting is never papered over with a placeholder: the status and
// that profile's file say not configured, while the page and the other
// profile still serve.
func TestUnconfiguredProfilesAreNeverServed(t *testing.T) {
	h := configured(t)
	h.Adult.GatewayRootCA = ""
	srv := newServer(t, h)

	res, body := get(t, srv, StatusPath, "good")
	if res.StatusCode != http.StatusServiceUnavailable || code(t, res, body) != "not_configured" {
		t.Fatalf("status with the certificate unset: %d %s", res.StatusCode, body)
	}
	res, body = get(t, srv, FilesPath+profile.AdultFileName, "good")
	if res.StatusCode != http.StatusServiceUnavailable || code(t, res, body) != "not_configured" {
		t.Fatalf("adult profile with the certificate unset: %d %s", res.StatusCode, body)
	}
	if res, _ := get(t, srv, FilesPath+profile.KidsFileName, "good"); res.StatusCode != http.StatusOK {
		t.Fatalf("kids profile: %d", res.StatusCode)
	}
	if res, _ := get(t, srv, PagePath, "good"); res.StatusCode != http.StatusOK {
		t.Fatalf("page: %d", res.StatusCode)
	}

	// A setting with a wrong value is not configured either.
	h = configured(t)
	h.Kids.DoHURL = "https://dns.example.com/dns-query"
	srv = newServer(t, h)
	res, body = get(t, srv, FilesPath+profile.KidsFileName, "good")
	if res.StatusCode != http.StatusServiceUnavailable || code(t, res, body) != "not_configured" {
		t.Fatalf("kids profile with a bad endpoint: %d %s", res.StatusCode, body)
	}
}

// Without an Access application, only a developer's computer serves the
// routes; a deployed environment refuses them, token or not.
func TestWithoutAccessOnlyADevelopersComputerServes(t *testing.T) {
	deployed := configured(t)
	deployed.Access = nil
	srv := newServer(t, deployed)
	for _, path := range routes {
		res, body := get(t, srv, path, "good")
		if res.StatusCode != http.StatusServiceUnavailable || code(t, res, body) != "access_not_configured" {
			t.Errorf("%s on dev without Access: %d %s", path, res.StatusCode, body)
		}
	}

	local := configured(t)
	local.Access = nil
	local.Env = "local"
	srv = newServer(t, local)
	for _, path := range routes {
		if res, _ := get(t, srv, path, ""); res.StatusCode != http.StatusOK {
			t.Errorf("%s on a developer's computer: %d", path, res.StatusCode)
		}
	}
}
