package minecraft

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
)

// Minecraft Admin's other routes: the players on the allowlist, each
// world's archives and restoring one, and each world's operators. All need
// minecraft-admin, name no Member to the backend, and send it only bodies
// the site builds itself from checked values. The site logs no player name.
//
//	GET  /api/minecraft/admin/players    -> GET  /server/allowlist
//	POST /api/minecraft/admin/players    -> POST /server/allowlist
//	GET  /api/minecraft/admin/archives   -> GET  /server/archives?world=<id>
//	POST /api/minecraft/admin/restore    -> POST /server/restore
//	GET  /api/minecraft/admin/operators  -> GET  /server/operators
//	POST /api/minecraft/admin/operators  -> POST /server/operators

// The backend's own rules for a player's name and an archive's.
var (
	javaName    = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
	archiveName = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z\.tar\.gz$`)
)

func validGamertag(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || len(utf16.Encode([]rune(name))) > 16 {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != ' ' && r != '_' {
			return false
		}
	}
	return true
}

// player is one named player and what to do with them, as the page sends it.
type player struct {
	World     *string `json:"world,omitempty"`
	Operation string  `json:"operation"`
	Edition   string  `json:"edition"`
	Name      string  `json:"name"`
}

func (p player) valid() bool {
	if p.Operation != "add" && p.Operation != "remove" {
		return false
	}
	switch p.Edition {
	case "java":
		return javaName.MatchString(p.Name)
	case "bedrock":
		return validGamertag(p.Name)
	}
	return false
}

// asked reads the page's JSON body, at most 1 KiB, into v.
func asked(r *http.Request, v any) bool {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
	return err == nil && len(raw) <= maxRequestBody && json.Unmarshal(raw, v) == nil
}

// admin answers the request itself unless it may go on: a change must come
// from this site, and the person must hold minecraft-admin.
func (t *Tool) admin(w http.ResponseWriter, r *http.Request, change string) bool {
	if change != "" && !sameSite(w, r, change+" from the Minecraft Admin page") {
		return false
	}
	_, ok := t.allow(w, r, AdminGrant)
	return ok
}

// GET /api/minecraft/admin/players: the allowlist every world shares.
func (t *Tool) players(w http.ResponseWriter, r *http.Request) {
	if t.admin(w, r, "") {
		t.forward(w, r, http.MethodGet, "/server/allowlist", "", nil)
	}
}

// POST /api/minecraft/admin/players: add or remove one player. The backend
// answers 404 player_not_found when no account has the name, and 409
// linked_account for a gamertag linked to a Java account.
func (t *Tool) changePlayer(w http.ResponseWriter, r *http.Request) {
	if !t.admin(w, r, "change the allowlist") {
		return
	}
	var p player
	if !asked(r, &p) || !p.valid() {
		writeError(w, http.StatusBadRequest, "bad_request", "send add or remove, an edition and a player's name")
		return
	}
	p.World = nil
	body, _ := json.Marshal(p)
	t.forward(w, r, http.MethodPost, "/server/allowlist", "", body)
}

// GET /api/minecraft/admin/archives?world=<id>: one world's archives.
func (t *Tool) archives(w http.ResponseWriter, r *http.Request) {
	if !t.admin(w, r, "") {
		return
	}
	id := r.URL.Query().Get("world")
	if !worldID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request", "that isn't a world's id")
		return
	}
	t.forward(w, r, http.MethodGet, "/server/archives?world="+id, "", nil)
}

// POST /api/minecraft/admin/restore: put one archive in place of a stopped
// world, which then starts. The backend answers 409 not_stopped otherwise.
func (t *Tool) restore(w http.ResponseWriter, r *http.Request) {
	if !t.admin(w, r, "restore a world") {
		return
	}
	var a struct {
		World   string `json:"world"`
		Archive string `json:"archive"`
	}
	if !asked(r, &a) || !worldID.MatchString(a.World) || !archiveName.MatchString(a.Archive) {
		writeError(w, http.StatusBadRequest, "bad_request", "send a world and one of its archives")
		return
	}
	body, _ := json.Marshal(a)
	t.forward(w, r, http.MethodPost, "/server/restore", "", body)
}

// GET /api/minecraft/admin/operators: each world's operators.
func (t *Tool) operators(w http.ResponseWriter, r *http.Request) {
	if t.admin(w, r, "") {
		t.forward(w, r, http.MethodGet, "/server/operators", "", nil)
	}
}

// POST /api/minecraft/admin/operators: make a player an operator of one
// world, or stop them being one. The backend answers 409 not_allowlisted
// for a player who isn't on the allowlist.
func (t *Tool) changeOperator(w http.ResponseWriter, r *http.Request) {
	if !t.admin(w, r, "change a world's operators") {
		return
	}
	var p player
	if !asked(r, &p) || !p.valid() || p.World == nil || !worldID.MatchString(*p.World) {
		writeError(w, http.StatusBadRequest, "bad_request", "send a world, add or remove, an edition and a player's name")
		return
	}
	body, _ := json.Marshal(p)
	t.forward(w, r, http.MethodPost, "/server/operators", "", body)
}
