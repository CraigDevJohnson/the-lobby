package minecraft

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/CraigDevJohnson/the-lobby/internal/invite"
)

// The invite's read of the worlds is a signed GET /server that names no
// Member. It keeps each world in the backend's order with its name and the
// addresses it can use, and leaves out what the page would leave out too.
func TestWorldsReadsTheAddressesForAnInvite(t *testing.T) {
	var asked []seen
	signed := false
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, seen{r.Method, r.URL.Path, r.Header.Get(MemberHeader)})
		signed = r.Header.Get("X-Signed") == "yes"
		_, _ = io.WriteString(w, `{"name":"Survival","state":"ready","worlds":[
			{"id":"survival","name":"Survival","state":"ready","connection":{"java":{"address":"survival.mc.example.net"},"bedrock":{"address":"survival.mc.example.net","port":19132}}},
			{"id":"creative","name":" Creative ","connection":{"java":{"address":"creative.mc.example.net","port":25566},"bedrock":{"address":"be.example.net"}}},
			{"id":"adventure","name":"Adventure","connection":{"java":{"address":"has space.example.net"},"bedrock":{"address":"adventure.mc.example.net","port":19133}}},
			{"id":"survival","name":"Second Survival","connection":{"java":{"address":"other.example.net"}}},
			{"id":"Bad Id","name":"Bad","connection":{"java":{"address":"bad.example.net"}}},
			{"id":"nameless","connection":{"java":{"address":"nameless.example.net"}}},
			{"id":"closed","name":"Closed","connection":{}}
		]}`)
	}))
	t.Cleanup(be.Close)
	tool := &Tool{
		Backend: be.URL,
		Sign:    func(r *http.Request, _ []byte) error { r.Header.Set("X-Signed", "yes"); return nil },
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	got, err := tool.Worlds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []invite.World{
		{Name: "Survival", JavaAddress: "survival.mc.example.net", BedrockAddress: "survival.mc.example.net", BedrockPort: 19132},
		{Name: "Creative", JavaAddress: "creative.mc.example.net", JavaPort: 25566},
		{Name: "Adventure", BedrockAddress: "adventure.mc.example.net", BedrockPort: 19133},
	}
	if !slices.Equal(got, want) {
		t.Errorf("worlds:\n%+v\nwant:\n%+v", got, want)
	}
	if len(asked) != 1 || asked[0] != (seen{http.MethodGet, "/server", ""}) || !signed {
		t.Errorf("asked %+v, signed %v", asked, signed)
	}
}

// Anything but a readable 200 is an error, so the invite links to the
// Launcher instead, and with no backend nothing is asked.
func TestWorldsFailsWithoutAReadableAnswer(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
	}{
		{http.StatusServiceUnavailable, `{"code":"unavailable"}`},
		{http.StatusNotFound, `{"code":"no_server"}`},
		{http.StatusOK, `not json`},
	} {
		be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		}))
		tool := &Tool{Backend: be.URL, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
		if worlds, err := tool.Worlds(context.Background()); err == nil {
			t.Errorf("%d %s: %+v without an error", c.status, c.body, worlds)
		}
		be.Close()
	}
	if _, err := (&Tool{}).Worlds(context.Background()); err == nil {
		t.Error("no backend: no error")
	}
}
