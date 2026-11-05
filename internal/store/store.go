// Package store holds what the site itself remembers: who the Members are,
// which Tools each may use, and their Sign-in sessions. Tool backends keep
// their own data (ADR 0001), except a Member's Let's Plan Soccer calendar:
// their Remembered teams, their Member link's secret, and their Google
// access and chosen Google calendar are kept here, so the soccer backend
// stores nothing (ADR 0011).
package store

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

type Member struct {
	Email   string
	Tools   []string
	AddedAt time.Time
}

type Session struct {
	ID        string
	Email     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Soccer is a Member's Let's Plan Soccer calendar: the Teams it follows and
// the secret in its Member link's address, "" until the first Team.
type Soccer struct {
	Email  string
	Teams  []RememberedTeam
	Secret string
}

// RememberedTeam is a Team a Member's calendar follows from one Session to
// the next: its name and its Team IDs, newest first, at most two, since a
// Team ID belongs to one Session.
type RememberedTeam struct {
	Name    string   `json:"name" dynamodbav:"name"`
	TeamIDs []string `json:"team_ids" dynamodbav:"team_ids"`
}

// Google is a Member's Google access for adding games to Google Calendar:
// the Google account it belongs to, its refresh token sealed with the
// site's token key (so the table alone never gives it away), and the
// calendar the Member picked, "primary" until they pick another.
type Google struct {
	Email      string
	Account    string
	Token      string
	CalendarID string
}

type Store interface {
	GetMember(ctx context.Context, email string) (Member, error)
	PutMember(ctx context.Context, m Member) error
	// DeleteMember also deletes the Member's Let's Plan Soccer calendar,
	// stops its Member link, and forgets their Google access.
	DeleteMember(ctx context.Context, email string) error
	ListMembers(ctx context.Context) ([]Member, error)
	GetSession(ctx context.Context, id string) (Session, error)
	PutSession(ctx context.Context, s Session) error
	DeleteSession(ctx context.Context, id string) error
	GetSoccer(ctx context.Context, email string) (Soccer, error)
	PutSoccer(ctx context.Context, s Soccer) error
	// SoccerLink is the Member whose Member link has this secret.
	SoccerLink(ctx context.Context, secret string) (email string, err error)
	PutSoccerLink(ctx context.Context, secret, email string) error
	DeleteSoccerLink(ctx context.Context, secret string) error
	GetGoogle(ctx context.Context, email string) (Google, error)
	PutGoogle(ctx context.Context, g Google) error
	DeleteGoogle(ctx context.Context, email string) error
}

// NormalizeEmail makes the address the lookup key: lower case, trimmed.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
