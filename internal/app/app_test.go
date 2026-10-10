package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A calendar link's token stays out of the request log whether or not the
// request matched a route: a wrong method or a longer path is logged as the
// route too.
func TestRequestLogLeavesOutLinkTokens(t *testing.T) {
	var logged bytes.Buffer
	h, err := New(context.Background(), Config{Env: "local", SoccerURL: "http://127.0.0.1:1"}, slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/soccer/link/c2VjcmV0dG9rZW4.ics"},
		{http.MethodPost, "/soccer/link/c2VjcmV0dG9rZW4.ics"},
		{http.MethodGet, "/soccer/link/c2VjcmV0dG9rZW4/extra"},
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
	}
	if n := strings.Count(logged.String(), "path=/soccer/link/{token}"); n != 3 {
		t.Fatalf("%d of 3 link requests were logged as the route: %s", n, logged.String())
	}
	if strings.Contains(logged.String(), "c2VjcmV0dG9rZW4") {
		t.Fatalf("log carries the token: %s", logged.String())
	}
}
