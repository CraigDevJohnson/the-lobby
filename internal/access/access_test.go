package access

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// A stand-in for Cloudflare's team domain: serves one signing key and signs
// tokens with it.
type fakeTeam struct {
	srv *httptest.Server
	key *rsa.PrivateKey
}

func newFakeTeam(t *testing.T) *fakeTeam {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}
	mux := http.NewServeMux()
	mux.HandleFunc("/cdn-cgi/access/certs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &fakeTeam{srv: srv, key: key}
}

func (f *fakeTeam) token(t *testing.T, aud, email string, exp time.Time, issuer string) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	if issuer == "" {
		issuer = f.srv.URL
	}
	claims := jwt.Claims{Issuer: issuer, Audience: jwt.Audience{aud}, Expiry: jwt.NewNumericDate(exp), IssuedAt: jwt.NewNumericDate(time.Now())}
	extra := map[string]any{"email": email}
	tok, err := jwt.Signed(signer).Claims(claims).Claims(extra).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestCloudflareVerify(t *testing.T) {
	team := newFakeTeam(t)
	const aud = "app-aud-tag"
	v, err := NewCloudflare(context.Background(), team.srv.URL, aud)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)

	id, err := v.Verify(context.Background(), team.token(t, aud, "Someone@Example.com", future, ""))
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if id.Email != "Someone@Example.com" {
		t.Fatalf("email = %q", id.Email)
	}

	bad := map[string]string{
		"wrong audience": team.token(t, "other-app", "a@b.c", future, ""),
		"expired":        team.token(t, aud, "a@b.c", time.Now().Add(-time.Minute), ""),
		"wrong issuer":   team.token(t, aud, "a@b.c", future, "https://elsewhere.example"),
		"garbage":        "not.a.token",
	}
	for name, tok := range bad {
		if _, err := v.Verify(context.Background(), tok); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestTokenFromRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/signin", nil)
	if TokenFromRequest(r) != "" {
		t.Fatal("expected no token")
	}
	r.AddCookie(&http.Cookie{Name: "CF_Authorization", Value: "from-cookie"})
	if TokenFromRequest(r) != "from-cookie" {
		t.Fatal("cookie not read")
	}
	r.Header.Set(Header, "from-header")
	if TokenFromRequest(r) != "from-header" {
		t.Fatal("header must win over cookie")
	}
}
