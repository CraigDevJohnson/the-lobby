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
}

func NewMemory() *Memory {
	return &Memory{members: map[string]Member{}, sessions: map[string]Session{}}
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
	delete(m.members, NormalizeEmail(email))
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
