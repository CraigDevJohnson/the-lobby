// Package store holds what the site itself remembers: who the Members are,
// which Tools each may use, and their Sign-in sessions. Tool backends keep
// their own data (ADR 0001), except a Member's Let's Plan Soccer calendar:
// their Remembered teams, their Member link's secret, their Google and
// iCloud connections, and the games the daily update has added to each
// calendar are kept here, so the soccer backend stores nothing (ADR 0011).
// The family's wish lists are kept here too, one record each (ADR 0019).
package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")

// ErrRemoving is GetMember's answer for an address whose Member is being
// removed: DeleteMember marked the record and has not deleted it yet, because
// deleting their calendar failed part-way. It is an ErrNotFound, so sign-in,
// a Sign-in session, the Tools, a Member link and the daily iCloud update all
// treat the address as nobody's. The Owner's commands tell it apart, to
// remove the Member again and to refuse adding the address until that is done.
var ErrRemoving = fmt.Errorf("the member is being removed: %w", ErrNotFound)

// ErrConflict is a Member write refused because the record is no longer what
// the caller read: the address was added, removed or invited again since.
// A wish list write answers it too, when the list is no longer at the
// revision the editor read, or already exists for a create.
var ErrConflict = errors.New("conflict")

// ErrAccessChanged is a wish list write refused because the editor is no
// longer the Member the request checked: removed, being removed, invited
// again, or without a grant the change needs.
var ErrAccessChanged = errors.New("access changed")

type Member struct {
	Email      string
	Tools      []string
	AddedAt    time.Time
	Generation string
}

type Session struct {
	ID               string
	Email            string
	MemberGeneration string
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

// Soccer is a Member's Let's Plan Soccer calendar: the Teams it follows and
// the secret of any existing Member link. New calendars have no secret.
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
// ConnectionID changes on every connection, even when Google returns the
// same refresh token, so work holding an older snapshot cannot change it.
type Google struct {
	Email        string
	Account      string
	Token        string
	CalendarID   string
	ConnectionID string
	Daily        bool
	Written      []string
	Times        map[string]GameTime
	// DailyRevision binds a scheduled save to its settings and progress snapshot.
	// Connections saved before daily updates start at zero.
	DailyRevision int64
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
// next run, and a Session-over notice never has one. ConnectionID changes
// on every connection so work holding an older snapshot cannot change it.
type ICloud struct {
	Email        string
	Account      string
	Password     string
	Home         string
	CalendarID   string
	ConnectionID string
	Daily        bool
	Written      []string
	Times        map[string]GameTime
}

// GameTime is a game's start and field as the backend's calendar file gives
// them: the start as Boise wall-clock digits, such as 20261121T191500, and
// the field as a person reads it, such as "Field 3".
type GameTime struct {
	Start string `dynamodbav:"start"`
	Field string `dynamodbav:"field"`
}

// The kinds of wish list (ADR 0019).
const (
	WishPersonal = "personal"
	WishManaged  = "managed"
)

// WishList is one of the family's wish lists, kept as one record. A personal
// list belongs to the membership that made it, Owner and OwnerGeneration,
// and each membership has at most one. A managed list belongs to the family
// and has neither. Revision counts its saves from 1, and ID never changes.
type WishList struct {
	ID              string
	Kind            string
	Name            string
	Revision        int64
	Wishes          []Wish
	Owner           string
	OwnerGeneration string
}

// Wish is one gift idea on a wish list. URL and Notes may be empty.
type Wish struct {
	ID    string `dynamodbav:"id"`
	Title string `dynamodbav:"title"`
	URL   string `dynamodbav:"url,omitempty"`
	Notes string `dynamodbav:"notes,omitempty"`
}

// WishEditor is the Member changing a wish list, as the request's access
// check read them, and the grants the change needs. The write holds only
// while their record is still that membership, is not being removed and
// holds every grant in Need.
type WishEditor struct {
	Email      string
	Generation string
	Need       []string
}

type Store interface {
	// GetMember is the Member with this address. An address with no record
	// answers ErrNotFound, and so does one whose Member is being removed, as
	// ErrRemoving: everything that checks access reads the Member here, so
	// this is where a removal that has begun ends their access.
	GetMember(ctx context.Context, email string) (Member, error)
	// CreateMember writes a new Member's record, and only while the address
	// has none, not even one being removed. Otherwise it changes nothing and
	// answers ErrConflict.
	CreateMember(ctx context.Context, m Member) error
	// ReplaceMember writes m over the record it was read from, and only
	// while that record still has m's Generation, which is none for a record
	// made before generations, and is not being removed. A record removed or
	// invited again since the read is left as it is, and the answer is
	// ErrConflict: a late change never brings back a removed Member, replaces
	// the new invitation's Tools and Generation, or undoes a removal's mark.
	ReplaceMember(ctx context.Context, m Member) error
	// DeleteMember first marks the Member's record as being removed, which
	// ends their access at once (GetMember). Then it stops their Member link,
	// deletes their Let's Plan Soccer calendar, forgets their Google and
	// iCloud access, deletes the personal wish list of the membership it
	// marked, and deletes the record last, only while it is still the record
	// it marked. If any of that fails, the marked record stays: the address
	// can't be added again, and the removal can be tried again until it
	// finishes. A removal that runs late, after the address was invited
	// again, leaves the new membership's record and wish list alone.
	DeleteMember(ctx context.Context, email string) error
	// ListMembers is every Member record, one being removed included, so the
	// Owner still sees it and can remove it again.
	ListMembers(ctx context.Context) ([]Member, error)
	GetSession(ctx context.Context, id string) (Session, error)
	PutSession(ctx context.Context, s Session) error
	// RenewSession advances only the expiry of the session while it still has
	// currentExpiresAt. An already-advanced expiry means another renewal won
	// the race and is also success; a missing session answers ErrNotFound.
	RenewSession(ctx context.Context, id string, currentExpiresAt, expiresAt time.Time) error
	DeleteSession(ctx context.Context, id string) error
	// ListSessions is every Sign-in session that hasn't expired, for Lobby
	// Admin's "last here" and its sign-out. A session's ID is its cookie's
	// value, so nothing that reads this may pass the IDs on.
	ListSessions(ctx context.Context) ([]Session, error)
	GetSoccer(ctx context.Context, email string) (Soccer, error)
	// UpdateSoccerTeams replaces the Teams only while both Teams and Secret
	// still match was. An empty snapshot can create the first calendar.
	UpdateSoccerTeams(ctx context.Context, was Soccer, teams []RememberedTeam) error
	// RevokeSoccerLink removes the matching legacy secret and its lookup in one write.
	RevokeSoccerLink(ctx context.Context, was Soccer) error
	// SoccerLink is the Member whose Member link has this secret.
	SoccerLink(ctx context.Context, secret string) (email string, err error)
	GetGoogle(ctx context.Context, email string) (Google, error)
	// PutGoogle and PutICloud require an active Member in the same transaction.
	PutGoogle(ctx context.Context, g Google) error
	// A manual copy remembers the destination only while daily updates are off.
	UpdateGoogleCalendar(ctx context.Context, current Google, calendarID string) error
	UpdateGoogleDailySwitch(ctx context.Context, current Google, on bool, calendarID string) error
	ListGoogle(ctx context.Context) ([]Google, error)
	// UpdateGoogleDaily saves Daily, Written and Times only when the same
	// connection, calendar and DailyRevision are still enabled.
	UpdateGoogleDaily(ctx context.Context, g Google) error
	// DeleteGoogleConnection deletes only current, the connection the caller
	// read: one Google refused, or the one an explicit disconnect read before
	// it asked Google to revoke the token. A connection made since is left as
	// it is, and the answer is ErrNotFound.
	DeleteGoogleConnection(ctx context.Context, current Google) error
	GetICloud(ctx context.Context, email string) (ICloud, error)
	PutICloud(ctx context.Context, c ICloud) error
	// A manual copy preserves an active daily destination. Changing the daily
	// destination explicitly starts its event history afresh.
	UpdateICloudCalendar(ctx context.Context, current ICloud, calendarID string) error
	UpdateICloudDailySwitch(ctx context.Context, current ICloud, on bool, calendarID string) error
	// DeleteICloudConnection is the conditional counterpart to
	// DeleteGoogleConnection, for a connection Apple refused. An explicit
	// iCloud disconnect asks Apple nothing first, so it uses DeleteICloud,
	// which forgets whichever connection is live.
	DeleteICloudConnection(ctx context.Context, current ICloud) error
	DeleteICloud(ctx context.Context, email string) error
	// ListICloud is every iCloud connection, for the daily update.
	ListICloud(ctx context.Context) ([]ICloud, error)
	// UpdateICloudDaily sets only Daily, Written and Times, and only while
	// the Member's connection is still the one c was read from, with the
	// daily update still on: its connection ID, password and calendar are
	// still c's.
	// Otherwise it changes nothing and answers ErrNotFound. So the daily
	// update never brings back the password of a Member who disconnected,
	// never writes an old list onto a new connection or calendar, and never
	// turns back on a switch the Member turned off while it ran.
	UpdateICloudDaily(ctx context.Context, c ICloud) error
	// ListWishLists is every list the family sees: each managed list, and
	// the personal list of each Member whose record is there, is not being
	// removed and has the generation the list was made under.
	ListWishLists(ctx context.Context) ([]WishList, error)
	// SaveWishList writes l over the version the editor read, l.Revision,
	// or creates it at revision 0, and answers it as saved, one revision on.
	// A personal list is always by's own. The write holds only while by is
	// still the Member checked, with every grant by.Need (else
	// ErrAccessChanged), and the list still has l's ID and revision, or for
	// a create there is no list there yet (else ErrConflict).
	SaveWishList(ctx context.Context, by WishEditor, l WishList) (WishList, error)
	// DeleteWishList deletes the list l names by Kind, ID and Revision, on
	// the same conditions as SaveWishList.
	DeleteWishList(ctx context.Context, by WishEditor, l WishList) error
}

// wishKey is where a wish list's record is kept: a managed list by its ID,
// and a personal list by its owner's membership, generation first, since a
// generation never holds "#", so the key names one membership. It is
// ErrNotFound for any other kind.
func wishKey(kind, id, email, generation string) (string, error) {
	switch kind {
	case WishManaged:
		return "wish#managed#" + id, nil
	case WishPersonal:
		return personalWishKey(email, generation), nil
	}
	return "", fmt.Errorf("wish list kind: %w", ErrNotFound)
}

func personalWishKey(email, generation string) string {
	return "wish#personal#" + generation + "#" + NormalizeEmail(email)
}

// holds reports whether tools has every grant in need.
func holds(tools, need []string) bool {
	for _, g := range need {
		if !slices.Contains(tools, g) {
			return false
		}
	}
	return true
}

// NormalizeEmail makes the address the lookup key: lower case, trimmed.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NewMember makes a record for a new membership. Generation binds Sign-in
// sessions to this invitation of the address, so removing and later adding
// the same address cannot make an earlier session valid again. Records made
// before generations were introduced use "" and remain compatible until
// that Member is removed.
func NewMember(email string, tools []string, addedAt time.Time) (Member, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Member{}, err
	}
	return Member{
		Email:      NormalizeEmail(email),
		Tools:      tools,
		AddedAt:    addedAt,
		Generation: base64.RawURLEncoding.EncodeToString(raw),
	}, nil
}

// saved is l as a save by by writes it: at the next revision, a personal
// list as by's own, a managed list as nobody's, and never with nil wishes.
func (l WishList) saved(by WishEditor) WishList {
	l.Revision++
	l.Owner, l.OwnerGeneration = "", ""
	if l.Kind == WishPersonal {
		l.Owner, l.OwnerGeneration = NormalizeEmail(by.Email), by.Generation
	}
	l.Wishes = slices.Clone(l.Wishes)
	if l.Wishes == nil {
		l.Wishes = []Wish{}
	}
	return l
}

// clone copies a wish list, so a caller changing what it was given or got
// back cannot change what is stored, as with DynamoDB.
func (l WishList) clone() WishList {
	l.Wishes = slices.Clone(l.Wishes)
	return l
}
