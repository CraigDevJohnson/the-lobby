package store

import (
	"context"
	"maps"
	"sort"
	"sync"
	"time"
)

// Memory is the store used on a developer's computer and in tests.
type Memory struct {
	mu       sync.Mutex
	members  map[string]Member
	removing map[string]bool // addresses whose record DeleteMember has marked
	cleanup  error           // what DeleteMember answers once it has marked (FailCleanup)
	sessions map[string]Session
	soccer   map[string]Soccer
	links    map[string]string // Member link secret -> email
	google   map[string]Google
	icloud   map[string]ICloud
}

func NewMemory() *Memory {
	return &Memory{members: map[string]Member{}, removing: map[string]bool{}, sessions: map[string]Session{}, soccer: map[string]Soccer{}, links: map[string]string{}, google: map[string]Google{}, icloud: map[string]ICloud{}}
}

func (m *Memory) GetMember(_ context.Context, email string) (Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email = NormalizeEmail(email)
	mem, ok := m.members[email]
	switch {
	case !ok:
		return Member{}, ErrNotFound
	case m.removing[email]:
		return Member{}, ErrRemoving
	}
	return mem, nil
}

// current reports whether email, normalized, has a Member record that is not
// being removed: the check each write of a Member's calendar records makes,
// under the same lock as the write.
func (m *Memory) current(email string) bool {
	_, ok := m.members[email]
	return ok && !m.removing[email]
}

// PutMember writes a record whatever is there, one being removed included.
// It is not part of Store: tests set a Member up with it.
func (m *Memory) PutMember(_ context.Context, mem Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem.Email = NormalizeEmail(mem.Email)
	m.members[mem.Email] = mem
	delete(m.removing, mem.Email)
	return nil
}

func (m *Memory) CreateMember(_ context.Context, mem Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem.Email = NormalizeEmail(mem.Email)
	if _, ok := m.members[mem.Email]; ok {
		return ErrConflict
	}
	m.members[mem.Email] = mem
	return nil
}

func (m *Memory) ReplaceMember(_ context.Context, mem Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mem.Email = NormalizeEmail(mem.Email)
	if was, ok := m.members[mem.Email]; !ok || m.removing[mem.Email] || was.Generation != mem.Generation {
		return ErrConflict
	}
	m.members[mem.Email] = mem
	return nil
}

func (m *Memory) DeleteMember(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email = NormalizeEmail(email)
	if _, ok := m.members[email]; ok {
		m.removing[email] = true
	}
	if m.cleanup != nil {
		return m.cleanup
	}
	if s, ok := m.soccer[email]; ok && s.Secret != "" {
		delete(m.links, s.Secret)
	}
	delete(m.soccer, email)
	delete(m.google, email)
	delete(m.icloud, email)
	delete(m.members, email)
	delete(m.removing, email)
	return nil
}

// FailCleanup makes DeleteMember stop with err once it has marked the
// Member's record and before it deletes anything, as DynamoDB failing
// part-way does. Nil lets a removal finish again. It is not part of Store:
// tests leave a removal unfinished with it.
func (m *Memory) FailCleanup(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanup = err
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
	if !m.current(g.Email) {
		return ErrNotFound
	}
	m.google[g.Email] = g
	return nil
}

func (m *Memory) UpdateGoogleCalendar(_ context.Context, current Google, calendarID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(current.Email)
	live, ok := m.google[email]
	if !ok || live.ConnectionID != current.ConnectionID || live.Token != current.Token || live.CalendarID != current.CalendarID {
		return ErrNotFound
	}
	live.CalendarID = calendarID
	m.google[email] = live
	return nil
}

func (m *Memory) DeleteGoogleConnection(_ context.Context, current Google) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(current.Email)
	live, ok := m.google[email]
	if !ok || live.ConnectionID != current.ConnectionID || live.Token != current.Token {
		return ErrNotFound
	}
	delete(m.google, email)
	return nil
}

func (m *Memory) GetICloud(_ context.Context, email string) (ICloud, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.icloud[NormalizeEmail(email)]
	if !ok {
		return ICloud{}, ErrNotFound
	}
	c.Written, c.Times = append([]string(nil), c.Written...), maps.Clone(c.Times)
	return c, nil
}

func (m *Memory) PutICloud(_ context.Context, c ICloud) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.Email = NormalizeEmail(c.Email)
	if !m.current(c.Email) {
		return ErrNotFound
	}
	c.Written, c.Times = append([]string(nil), c.Written...), maps.Clone(c.Times)
	m.icloud[c.Email] = c
	return nil
}

func (m *Memory) UpdateICloudCalendar(_ context.Context, current ICloud, calendarID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(current.Email)
	live, ok := m.icloud[email]
	if !ok || live.ConnectionID != current.ConnectionID || live.Password != current.Password || live.CalendarID != current.CalendarID {
		return ErrNotFound
	}
	if live.CalendarID != calendarID {
		live.CalendarID, live.Written, live.Times = calendarID, nil, nil
	}
	m.icloud[email] = live
	return nil
}

func (m *Memory) UpdateICloudDailySwitch(_ context.Context, current ICloud, on bool, calendarID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(current.Email)
	live, ok := m.icloud[email]
	if !ok || live.ConnectionID != current.ConnectionID || live.Password != current.Password || live.CalendarID != current.CalendarID || live.Daily != current.Daily {
		return ErrNotFound
	}
	live.Daily = on
	if on && live.CalendarID != calendarID {
		live.CalendarID, live.Written, live.Times = calendarID, nil, nil
	}
	m.icloud[email] = live
	return nil
}

func (m *Memory) DeleteICloudConnection(_ context.Context, current ICloud) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(current.Email)
	live, ok := m.icloud[email]
	if !ok || live.ConnectionID != current.ConnectionID || live.Password != current.Password {
		return ErrNotFound
	}
	delete(m.icloud, email)
	return nil
}

func (m *Memory) ListICloud(_ context.Context) ([]ICloud, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ICloud, 0, len(m.icloud))
	for _, c := range m.icloud {
		c.Written, c.Times = append([]string(nil), c.Written...), maps.Clone(c.Times)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out, nil
}

func (m *Memory) UpdateICloudDaily(_ context.Context, c ICloud) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	email := NormalizeEmail(c.Email)
	was, ok := m.icloud[email]
	if !ok || !was.Daily || was.ConnectionID != c.ConnectionID || was.Password != c.Password || was.CalendarID != c.CalendarID {
		return ErrNotFound
	}
	was.Daily, was.Written, was.Times = c.Daily, append([]string(nil), c.Written...), maps.Clone(c.Times)
	m.icloud[email] = was
	return nil
}

func (m *Memory) DeleteICloud(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.icloud, NormalizeEmail(email))
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

func (m *Memory) RenewSession(_ context.Context, id string, currentExpiresAt, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return ErrNotFound
	}
	if !s.ExpiresAt.Equal(currentExpiresAt) {
		if s.ExpiresAt.After(currentExpiresAt) {
			return nil
		}
		return ErrNotFound
	}
	s.ExpiresAt = expiresAt
	m.sessions[id] = s
	return nil
}

func (m *Memory) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *Memory) ListSessions(_ context.Context) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	out := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.ExpiresAt.After(now) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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
	if !m.current(s.Email) {
		return ErrNotFound
	}
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
	email = NormalizeEmail(email)
	if !m.current(email) {
		return ErrNotFound
	}
	m.links[secret] = email
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
