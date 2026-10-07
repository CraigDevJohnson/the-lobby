// Package store holds what the site itself remembers: who the Members are,
// which Tools each may use, and their Sign-in sessions. Tool backends keep
// their own data (ADR 0001).
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

type Store interface {
	GetMember(ctx context.Context, email string) (Member, error)
	PutMember(ctx context.Context, m Member) error
	DeleteMember(ctx context.Context, email string) error
	ListMembers(ctx context.Context) ([]Member, error)
	GetSession(ctx context.Context, id string) (Session, error)
	PutSession(ctx context.Context, s Session) error
	DeleteSession(ctx context.Context, id string) error
}

// NormalizeEmail makes the address the lookup key: lower case, trimmed.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
