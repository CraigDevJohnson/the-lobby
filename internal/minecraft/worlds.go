package minecraft

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CraigDevJohnson/the-lobby/internal/invite"
)

// worldsWait bounds reading the worlds for an invite, which Lobby Admin's
// add waits for before it sends the email.
const worldsWait = 4 * time.Second

// Worlds reads each world's name and addresses from GET /server for a new
// Member's invite email (internal/invite), the same connection values the
// Launcher shows. Nobody is asking for themselves, so it names no Member.
// A world without a usable id, a name or an address to join is left out,
// as is a second entry for the same id. An error means the email links to
// the Launcher instead.
func (t *Tool) Worlds(ctx context.Context) ([]invite.World, error) {
	if t.Backend == "" {
		return nil, errNotConnected
	}
	ctx, cancel := context.WithTimeout(ctx, worldsWait)
	defer cancel()
	res, err := t.do(ctx, http.MethodGet, "/server", "", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("the backend answered " + res.Status)
	}
	type endpoint struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	}
	var status struct {
		Worlds []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Connection struct {
				Java    *endpoint `json:"java"`
				Bedrock *endpoint `json:"bedrock"`
			} `json:"connection"`
		} `json:"worlds"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBody)).Decode(&status); err != nil {
		return nil, err
	}
	var worlds []invite.World
	seen := map[string]bool{}
	for _, w := range status.Worlds {
		if !worldID.MatchString(w.ID) || seen[w.ID] || strings.TrimSpace(w.Name) == "" {
			continue
		}
		seen[w.ID] = true
		out := invite.World{Name: strings.TrimSpace(w.Name)}
		if j := w.Connection.Java; j != nil && usable(j.Address) {
			out.JavaAddress = strings.TrimSpace(j.Address)
			if port(j.Port) {
				out.JavaPort = j.Port
			}
		}
		if b := w.Connection.Bedrock; b != nil && usable(b.Address) && port(b.Port) {
			out.BedrockAddress, out.BedrockPort = strings.TrimSpace(b.Address), b.Port
		}
		if out.JavaAddress != "" || out.BedrockAddress != "" {
			worlds = append(worlds, out)
		}
	}
	return worlds, nil
}

// usable and port are the page's own rules for a connection value
// (web/src/minecraft/logic.ts): an address, trimmed, is one word of at most
// 255 characters, and a port is a TCP port.
func usable(address string) bool {
	a := strings.TrimSpace(address)
	return a != "" && len(a) <= 255 && !strings.ContainsFunc(a, func(r rune) bool { return r <= ' ' })
}

func port(p int) bool { return p >= 1 && p <= 65535 }
