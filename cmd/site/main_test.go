package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/CraigDevJohnson/the-lobby/internal/app"
)

// On Lambda the daily rule's event runs the daily iCloud update, an API
// Gateway HTTP request goes to the site, and an event the site doesn't know
// is an error, not an answer that looks like a success.
func TestLambdaTellsTheDailyEventFromARequest(t *testing.T) {
	var daily, served int
	handle := lambdaHandler(&app.Site{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
				served++
			}
			w.WriteHeader(http.StatusNoContent)
		}),
		Daily: func(context.Context) error {
			daily++
			return nil
		},
	})
	ctx := context.Background()

	if out, err := handle(ctx, json.RawMessage(`{"task":"icloud-daily"}`)); err != nil || out != nil || daily != 1 || served != 0 {
		t.Fatalf("the daily event: %v, %v; %d daily runs, %d requests", out, err, daily, served)
	}

	request, _ := json.Marshal(events.APIGatewayV2HTTPRequest{
		Version: "2.0",
		RawPath: "/healthz",
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodGet, Path: "/healthz"},
		},
	})
	out, err := handle(ctx, request)
	if res, ok := out.(events.APIGatewayV2HTTPResponse); err != nil || !ok || res.StatusCode != http.StatusNoContent || served != 1 || daily != 1 {
		t.Fatalf("a request: %v, %v; %d daily runs, %d requests", out, err, daily, served)
	}
	// A request is never the daily event, whatever else it carries.
	var both map[string]any
	_ = json.Unmarshal(request, &both)
	both["task"] = "icloud-daily"
	request, _ = json.Marshal(both)
	if _, err := handle(ctx, request); err != nil || served != 2 || daily != 1 {
		t.Fatalf("a request carrying the task: %v; %d daily runs, %d requests", err, daily, served)
	}

	for _, raw := range []string{`{}`, `{"task":"something-else"}`, `{"source":"aws.events","detail-type":"Scheduled Event"}`, `"icloud-daily"`, `not json`} {
		if out, err := handle(ctx, json.RawMessage(raw)); err == nil || out != nil {
			t.Errorf("%s: %v, %v; want an error", raw, out, err)
		}
	}
	if served != 2 || daily != 1 {
		t.Fatalf("an unknown event ran something: %d daily runs, %d requests", daily, served)
	}
}
