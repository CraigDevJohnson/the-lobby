package members

import (
	"context"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// StatusPath answers GET with the Site card's facts.
const StatusPath = "/api/lobby/status"

// pingWithin is how long every backend has to answer, well inside the
// site's own 8 seconds for a request.
const pingWithin = 4 * time.Second

// Backend is one Tool's backend as the Site card shows it.
type Backend struct {
	// Tool is the Tool's grant, such as "soccer", which the page names.
	Tool string
	// Configured is whether the backend's address is set.
	Configured bool
	// Ping makes one quick signed request that the backend answers with a
	// 2xx, and says why when it didn't.
	Ping func(context.Context) error
}

// Version is the commit the running server was built from: its hash, when
// that commit was made (go build's vcs.time, the nearest it records to the
// build itself), and whether the checkout had changes that weren't
// committed. Each is empty or false when the build recorded none.
type Version struct {
	Commit   string `json:"commit"`
	Built    string `json:"built"`
	Modified bool   `json:"modified"`
}

// Checked is one backend's answer to the check.
type Checked struct {
	Tool       string `json:"tool"`
	Configured bool   `json:"configured"`
	Answered   bool   `json:"answered"`
}

// Status is the Site card's answer. Backends come in the order the site
// was given them.
type Status struct {
	Environment string    `json:"environment"`
	Version     Version   `json:"version"`
	Calendars   bool      `json:"calendars"`
	Backends    []Checked `json:"backends"`
}

// GET /api/lobby/status: Lobby Admin's Site card (#98), which answers "is
// it the site or the Tool?" without the AWS console or the Actions log. It
// says which environment this is, the version it runs, whether the
// calendars are offered, and whether each Tool's backend is set up and
// answered a quick check just now. Nothing is kept between checks.
func (t *Tool) status(w http.ResponseWriter, r *http.Request) {
	if _, ok := t.allow(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, Status{
		Environment: t.Environment,
		Version:     running(),
		Calendars:   t.Calendars,
		Backends:    t.check(r.Context()),
	})
}

// check asks every backend that is set up at once, each for at most
// pingWithin, so a slow one never holds up the others.
func (t *Tool) check(ctx context.Context) []Checked {
	ctx, cancel := context.WithTimeout(ctx, pingWithin)
	defer cancel()
	out := make([]Checked, len(t.Backends))
	var wg sync.WaitGroup
	for i, b := range t.Backends {
		out[i] = Checked{Tool: b.Tool, Configured: b.Configured}
		if !b.Configured || b.Ping == nil {
			continue
		}
		wg.Go(func() {
			if err := b.Ping(ctx); err != nil {
				t.Log.Warn("lobby admin: backend not answering", "tool", b.Tool, "err", err)
				return
			}
			out[i].Answered = true
		})
	}
	wg.Wait()
	return out
}

// running is the version go build recorded. `task lambda` builds in a git
// checkout, so the Lambda knows its commit; `go run` and the tests record
// none.
func running() Version {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version{}
	}
	return versionOf(info.Settings)
}

func versionOf(settings []debug.BuildSetting) Version {
	var v Version
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			v.Commit = s.Value
		case "vcs.time":
			v.Built = s.Value
		case "vcs.modified":
			v.Modified = s.Value == "true"
		}
	}
	return v
}
