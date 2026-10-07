package origin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequire(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(Header) != "" {
			t.Error("secret header must not reach the handler")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	cases := []struct {
		name, secret, header string
		want                 int
	}{
		{"matching secret", "s3cret", "s3cret", http.StatusNoContent},
		{"wrong secret", "s3cret", "nope", http.StatusForbidden},
		{"missing header", "s3cret", "", http.StatusForbidden},
		{"check disabled", "", "", http.StatusNoContent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.header != "" {
				req.Header.Set(Header, c.header)
			}
			rec := httptest.NewRecorder()
			Require(c.secret)(ok).ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("got %d, want %d", rec.Code, c.want)
			}
		})
	}
}
