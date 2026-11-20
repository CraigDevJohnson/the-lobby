// Package store holds what the site itself remembers: who the Members are,
// which Tools each may use, and their Sign-in sessions. Tool backends keep
// their own data (ADR 0001), except a Member's Let's Plan Soccer calendar:
// their Remembered teams, their Member link's secret, their Google access
// and chosen Google calendar, and their iCloud access, chosen iCloud
// calendar and the games the daily update has added to it, with the time and
// field the league last gave each, are kept here, so the soccer backend
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

// ICloud is a Member's iCloud access for adding games to iCloud Calendar
// (ADR 0012): the Apple Account address, its app-specific password sealed
// with the site's token key, where Apple keeps the account's calendars, and
// the calendar the Member picked, "" until they add or turn the daily update
// on. Daily says the Member turned the daily update on (ADR 0015), and
// Written is the UIDs it has put, or found already there, in CalendarID.
// Times is where the league had each of those games when the daily update
// last looked, by UID, so it can tell a game the league has moved since
// (ADR 0017). A game written before Times was kept has no entry until the
// next run, and a Session-over notice never has one.
type ICloud struct {
	Email      string
	Account    string
	Password   string
	Home       string
	CalendarID string
	Daily      bool
	Written    []string
	Times      map[string]GameTime
}

// GameTime is a game's start and field as the backend's calendar file gives
// them: the start as Boise wall-clock digits, such as 20261121T191500, and
// the field as a person reads it, such as "Field 3".
type GameTime struct {
	Start string `dynamodbav:"start"`
	Field string `dynamodbav:"field"`
}

type Store interface {
	GetMember(ctx context.Context, email string) (Member, error)
	PutMember(ctx context.Context, m Member) error
	// DeleteMember also deletes the Member's Let's Plan Soccer calendar,
	// stops its Member link, and forgets their Google and iCloud access.
	DeleteMember(ctx context.Context, email string) error
	ListMembers(ctx context.Context) ([]Member, error)
	GetSession(ctx context.Context, id string) (Session, error)
	PutSession(ctx context.Context, s Session) error
	DeleteSession(ctx context.Context, id string) error
	// ListSessions is every Sign-in session that hasn't expired, for Lobby
	// Admin's "last here" and its sign-out. A session's ID is its cookie's
	// value, so nothing that reads this may pass the IDs on.
	ListSessions(ctx context.Context) ([]Session, error)
	GetSoccer(ctx context.Context, email string) (Soccer, error)
	PutSoccer(ctx context.Context, s Soccer) error
	// SoccerLink is the Member whose Member link has this secret.
	SoccerLink(ctx context.Context, secret string) (email string, err error)
	PutSoccerLink(ctx context.Context, secret, email string) error
	DeleteSoccerLink(ctx context.Context, secret string) error
	GetGoogle(ctx context.Context, email string) (Google, error)
	PutGoogle(ctx context.Context, g Google) error
	DeleteGoogle(ctx context.Context, email string) error
	GetICloud(ctx context.Context, email string) (ICloud, error)
	PutICloud(ctx context.Context, c ICloud) error
	DeleteICloud(ctx context.Context, email string) error
	// ListICloud is every iCloud connection, for the daily update.
	ListICloud(ctx context.Context) ([]ICloud, error)
	// UpdateICloudDaily sets only Daily, Written and Times, and only while
	// the Member's connection is still the one c was read from, with the
	// daily update still on: its password and calendar are still c's.
	// Otherwise it changes nothing and answers ErrNotFound. So the daily
	// update never brings back the password of a Member who disconnected,
	// never writes an old list onto a new connection or calendar, and never
	// turns back on a switch the Member turned off while it ran.
	UpdateICloudDaily(ctx context.Context, c ICloud) error
}

// NormalizeEmail makes the address the lookup key: lower case, trimmed.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
