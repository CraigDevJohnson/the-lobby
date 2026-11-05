package store

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Memory is the store used on a developer's computer and in tests.
type Memory struct {
	mu       sync.Mutex
	members  map[string]Member
	sessions map[string]Session
	soccer   map[string]Soccer
	links    map[string]string // Member link secret -> email
	google   map[string]Google
}

func NewMemory() *Memory {
	return &Memory{members: map[string]Member{}, sessions: map[string]Session{}, soccer: map[string]Soccer{}, links: map[string]string{}, google: map[string]Google{}}
}

func (m *Memory) GetMember(_ context.Context, email string) (Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.members[NormalizeEmail(email)]
	if !ok {
		return Member{}, ErrNotFound
	}
	return mem, nil
}

func (m *Memory) PutMember(_ context.Context, mem Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem.Email = NormalizeEmail(mem.Email)
	m.members[mem.Email] = mem
	return nil
}

func (m *Memory) DeleteMember(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email = NormalizeEmail(email)
	delete(m.members, email)
	if s, ok := m.soccer[email]; ok && s.Secret != "" {
		delete(m.links, s.Secret)
	}
	delete(m.soccer, email)
	delete(m.google, email)
	return nil
}

func (m *Memory) GetGoogle(_ context.Context, email string) (Google, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.google[NormalizeEmail(email)]
	if !ok {
		return Google{}, ErrNotFound
	}
	return g, nil
}

func (m *Memory) PutGoogle(_ context.Context, g Google) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g.Email = NormalizeEmail(g.Email)
	m.google[g.Email] = g
	return nil
}

func (m *Memory) DeleteGoogle(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.google, NormalizeEmail(email))
	return nil
}

func (m *Memory) ListMembers(_ context.Context) ([]Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Member, 0, len(m.members))
	for _, mem := range m.members {
		out = append(out, mem)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out, nil
}

func (m *Memory) GetSession(_ context.Context, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || !s.ExpiresAt.After(time.Now()) {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (m *Memory) PutSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *Memory) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *Memory) GetSoccer(_ context.Context, email string) (Soccer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.soccer[NormalizeEmail(email)]
	if !ok {
		return Soccer{}, ErrNotFound
	}
	s.Teams = cloneTeams(s.Teams)
	return s, nil
}

func (m *Memory) PutSoccer(_ context.Context, s Soccer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.Email = NormalizeEmail(s.Email)
	s.Teams = cloneTeams(s.Teams)
	m.soccer[s.Email] = s
	return nil
}

func (m *Memory) SoccerLink(_ context.Context, secret string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email, ok := m.links[secret]
	if !ok {
		return "", ErrNotFound
	}
	return email, nil
}

func (m *Memory) PutSoccerLink(_ context.Context, secret, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links[secret] = NormalizeEmail(email)
	return nil
}

func (m *Memory) DeleteSoccerLink(_ context.Context, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.links, secret)
	return nil
}

// cloneTeams copies Remembered teams, so a caller changing what it was
// given or got back cannot change what is stored, as with DynamoDB.
func cloneTeams(teams []RememberedTeam) []RememberedTeam {
	if teams == nil {
		return nil
	}
	out := make([]RememberedTeam, len(teams))
	for i, t := range teams {
		out[i] = RememberedTeam{Name: t.Name, TeamIDs: append([]string(nil), t.TeamIDs...)}
	}
	return out
}
