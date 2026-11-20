package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/CraigDevJohnson/the-lobby/internal/app"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
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

// changedAfterRead is a store where something else changes the Members just
// after `site member add` has read one: then runs once, after GetMember.
type changedAfterRead struct {
	*store.Memory
	then func()
}

func (c *changedAfterRead) GetMember(ctx context.Context, email string) (store.Member, error) {
	m, err := c.Memory.GetMember(ctx, email)
	if then := c.then; then != nil {
		c.then = nil
		then()
	}
	return m, err
}

// `site member add` writes only over what it read: a new Member where the
// address had no record, and new Tools over the same invitation, a record
// made before generations included. When the Member was added, removed or
// invited again meanwhile, it writes nothing and says to run it again, so it
// never brings back a removed Member or replaces the new invitation's Tools
// and generation.
func TestMemberAddWritesOnlyOverWhatItRead(t *testing.T) {
	ctx := context.Background()
	const email = "member@example.com"
	add := func(st store.Store, tool string) error {
		return memberCommand(ctx, st, []string{"add", "Member@Example.com", tool})
	}

	st := store.NewMemory()
	if err := add(st, "soccer"); err != nil {
		t.Fatal(err)
	}
	first, err := st.GetMember(ctx, email)
	if err != nil || first.Generation == "" || !slices.Equal(first.Tools, []string{"soccer"}) {
		t.Fatalf("a new Member: %+v, %v", first, err)
	}
	if err := add(st, "minecraft"); err != nil {
		t.Fatal(err)
	}
	if m, _ := st.GetMember(ctx, email); m.Generation != first.Generation || !slices.Equal(m.Tools, []string{"minecraft"}) {
		t.Fatalf("new Tools for the same invitation: %+v", m)
	}
	_ = st.PutMember(ctx, store.Member{Email: email, Tools: []string{"soccer"}})
	if err := add(st, "minecraft"); err != nil {
		t.Fatal(err)
	}
	if m, _ := st.GetMember(ctx, email); m.Generation != "" || !slices.Equal(m.Tools, []string{"minecraft"}) {
		t.Fatalf("new Tools for a Member from before generations: %+v", m)
	}

	theirs := store.Member{Email: email, Tools: []string{"foundry"}, Generation: "theirs"}
	for _, c := range []struct {
		name     string
		before   *store.Member // the record the command reads, nil for none
		reinvite bool          // whether the address is invited again meanwhile, or only removed
	}{
		{"added meanwhile", nil, true},
		{"removed meanwhile", &store.Member{Email: email, Tools: []string{"soccer"}, Generation: "first"}, false},
		{"invited again meanwhile", &store.Member{Email: email, Tools: []string{"soccer"}, Generation: "first"}, true},
		{"from before generations, removed meanwhile", &store.Member{Email: email, Tools: []string{"soccer"}}, false},
		{"from before generations, invited again meanwhile", &store.Member{Email: email, Tools: []string{"soccer"}}, true},
	} {
		st := store.NewMemory()
		if c.before != nil {
			_ = st.PutMember(ctx, *c.before)
		}
		err := add(&changedAfterRead{Memory: st, then: func() {
			_ = st.DeleteMember(ctx, email)
			if c.reinvite {
				_ = st.PutMember(ctx, theirs)
			}
		}}, "minecraft")
		if err == nil || !strings.Contains(err.Error(), "run the command again") {
			t.Errorf("%s: %v, want to be told to run the command again", c.name, err)
		}
		m, err := st.GetMember(ctx, email)
		switch {
		case c.reinvite && (err != nil || m.Generation != "theirs" || !slices.Equal(m.Tools, []string{"foundry"})):
			t.Errorf("%s: the command wrote over the new invitation: %+v, %v", c.name, m, err)
		case !c.reinvite && !errors.Is(err, store.ErrNotFound):
			t.Errorf("%s: the command brought the removed Member back: %+v, %v", c.name, m, err)
		}
	}
}
