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

// Requests to the soccer backend are signed everywhere but a developer's
// computer unless SOCCER_BACKEND_SIGNING says otherwise: "lambda" signs even
// locally, "off" never signs, and anything else stops the site from starting.
func TestSoccerSigningFollowsTheEnvironmentUnlessTold(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	const backend = "https://backend.example"
	for _, c := range []struct {
		env, signing string
		signed       bool
	}{
		{"local", "", false},
		{"dev", "", true},
		{"prod", "", true},
		{"local", "lambda", true},
		{"dev", "off", false},
		{"prod", "lambda", true},
	} {
		tool, err := newSoccer(context.Background(), Config{Env: c.env, SoccerURL: backend, SoccerSigning: c.signing}, log)
		if err != nil {
			t.Fatalf("%s %q: %v", c.env, c.signing, err)
		}
		if got := tool.Sign != nil; got != c.signed {
			t.Errorf("env %s, SOCCER_BACKEND_SIGNING %q: signed %v, want %v", c.env, c.signing, got, c.signed)
		}
	}
	for _, env := range []string{"local", "prod"} {
		if _, err := newSoccer(context.Background(), Config{Env: env, SoccerURL: backend, SoccerSigning: "Lambda "}, log); err == nil {
			t.Errorf("env %s: an unknown SOCCER_BACKEND_SIGNING was accepted", env)
		}
	}
	// With no backend to call there is nothing to sign, but a bad value is still refused.
	if _, err := newSoccer(context.Background(), Config{Env: "prod", SoccerSigning: "sometimes"}, log); err == nil {
		t.Error("an unknown SOCCER_BACKEND_SIGNING was accepted when no backend is set")
	}
}
