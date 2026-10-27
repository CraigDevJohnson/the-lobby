package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
)

// requestLine runs one request through requestLogger and returns the
// attributes of the line it wrote.
func requestLine(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := requestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	h.ServeHTTP(httptest.NewRecorder(), r)
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("request log line %q: %v", buf.String(), err)
	}
	return line
}

// A calendar link is logged as its route: the token stays out of the log,
// and so does the query string (#28).
func TestRequestLogHidesLinkTokens(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/soccer/link/abc.def-ghi.ics?secret=1", nil)
	line := requestLine(t, r)
	if got, want := line["path"], "/soccer/link/{token}"; got != want {
		t.Fatalf("path logged as %q, want %q", got, want)
	}
	for k, v := range line {
		s, _ := v.(string)
		if bytes.Contains([]byte(s), []byte("abc.def")) || bytes.Contains([]byte(s), []byte("secret")) {
			t.Fatalf("attribute %q carries the link's address: %q", k, s)
		}
	}
	if got, want := line["status"], float64(http.StatusNoContent); got != want {
		t.Fatalf("status logged as %v, want %v", got, want)
	}
	if _, ok := line["rid"]; ok {
		t.Fatalf("a request outside API Gateway must log no rid, got %v", line["rid"])
	}
}

// A request that came through API Gateway logs its request id, which is the
// requestId in the API's access log, the log that no longer records paths.
func TestRequestLogCarriesGatewayRequestID(t *testing.T) {
	event := events.APIGatewayV2HTTPRequest{
		RawPath: "/healthz",
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			RequestID: "Kx7tQhXYPHcEJ3w=",
			HTTP:      events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodGet, Path: "/healthz"},
		},
	}
	r, err := (&core.RequestAccessorV2{}).EventToRequestWithContext(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	line := requestLine(t, r)
	if got, want := line["rid"], "Kx7tQhXYPHcEJ3w="; got != want {
		t.Fatalf("rid logged as %v, want %q", got, want)
	}
	if got, want := line["path"], "/healthz"; got != want {
		t.Fatalf("path logged as %v, want %q", got, want)
	}
}
