// Package google is the site's side of Add to Google Calendar (#52, ADR
// 0007, ADR 0011): a Member connects their Google account, and when they
// press Add the site writes the games they ticked into a Google calendar
// they pick. The site holds the Member's Google access and makes every
// Google call itself; the soccer backend only builds the event bodies and
// never sees a token.
package google

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/oauth2"
)

// The scopes asked for: who the Google account is, writing events, and
// listing the calendars it can write to. Without the last the main calendar
// is the only choice.
const (
	ScopeEvents       = "https://www.googleapis.com/auth/calendar.events"
	ScopeCalendarList = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
)

var scopes = []string{"openid", "email", ScopeEvents, ScopeCalendarList}

// Settings are the Google client's credentials and the key refresh tokens
// are sealed with. The site reads them from Parameter Store when it first
// needs them, never from its environment.
type Settings struct {
	ClientID     string
	ClientSecret string
	Key          []byte // 32 bytes, for AES-256-GCM
}

var (
	// ErrExpired means Google no longer accepts the Member's access: they
	// revoked it, or it lapsed. The connection is dropped and they connect
	// again.
	ErrExpired = errors.New("google: access expired or revoked")
	// ErrCalendar means the chosen calendar can't be written to: gone, or
	// shared read-only.
	ErrCalendar = errors.New("google: calendar can't be written to")
	// ErrScope means the Member didn't allow listing their calendars.
	ErrScope = errors.New("google: calendar list not allowed")
	// ErrSealed means a sealed token doesn't open with this key: the item
	// was sealed with another key, or changed.
	ErrSealed = errors.New("google: sealed token doesn't open")
)

// Client makes the Google calls. Its addresses are Google's own unless a
// test points them elsewhere.
type Client struct {
	HTTP        *http.Client
	Endpoint    oauth2.Endpoint
	UserInfoURL string
	CalendarURL string
	RevokeURL   string
}

// New is a Client for Google itself.
func New(hc *http.Client) *Client {
	return &Client{
		HTTP:        hc,
		Endpoint:    oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", AuthStyle: oauth2.AuthStyleInParams},
		UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
		CalendarURL: "https://www.googleapis.com/calendar/v3",
		RevokeURL:   "https://oauth2.googleapis.com/revoke",
	}
}

func (c *Client) config(s Settings, redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: s.ClientID, ClientSecret: s.ClientSecret, Endpoint: c.Endpoint, RedirectURL: redirect, Scopes: scopes}
}

func (c *Client) ctx(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, c.HTTP)
}

// AuthURL is where Connect sends the Member: Google's consent screen,
// asking for offline access every time so a refresh token always comes
// back, with their Lobby address as the account to suggest.
func (c *Client) AuthURL(s Settings, redirect, state, hint string) string {
	return c.config(s, redirect).AuthCodeURL(state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"), oauth2.SetAuthURLParam("login_hint", hint))
}

// Grant is what Google gave back for a code.
type Grant struct {
	Refresh string
	Access  string
	Scopes  []string
}

// Exchange turns the callback's code into tokens.
func (c *Client) Exchange(ctx context.Context, s Settings, redirect, code string) (Grant, error) {
	tok, err := c.config(s, redirect).Exchange(c.ctx(ctx), code)
	if err != nil {
		return Grant{}, fmt.Errorf("google: exchange the code: %w", err)
	}
	scope, _ := tok.Extra("scope").(string)
	return Grant{Refresh: tok.RefreshToken, Access: tok.AccessToken, Scopes: strings.Fields(scope)}, nil
}

// Has reports whether the grant includes scope.
func (g Grant) Has(scope string) bool { return slices.Contains(g.Scopes, scope) }

// Access trades a refresh token for an access token.
func (c *Client) Access(ctx context.Context, s Settings, refresh string) (string, error) {
	tok, err := c.config(s, "").TokenSource(c.ctx(ctx), &oauth2.Token{RefreshToken: refresh}).Token()
	var re *oauth2.RetrieveError
	if errors.As(err, &re) && (re.ErrorCode == "invalid_grant" || re.Response != nil && re.Response.StatusCode == http.StatusUnauthorized) {
		return "", ErrExpired
	}
	if err != nil {
		return "", fmt.Errorf("google: refresh: %w", err)
	}
	return tok.AccessToken, nil
}

// Account is the Google account's address, refused unless Google has
// verified it.
func (c *Client) Account(ctx context.Context, access string) (string, error) {
	var info struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err := c.call(ctx, access, http.MethodGet, c.UserInfoURL, nil, &info); err != nil {
		return "", err
	}
	if info.Email == "" || !info.Verified {
		return "", errors.New("google: the account's address isn't verified")
	}
	return info.Email, nil
}

// Calendar is one calendar the Member can write to.
type Calendar struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary,omitempty"`
}

// Calendars lists the calendars the account can write to, main one first.
func (c *Client) Calendars(ctx context.Context, access string) ([]Calendar, error) {
	var list struct {
		Items []struct {
			ID      string `json:"id"`
			Summary string `json:"summary"`
			Own     string `json:"summaryOverride"`
			Primary bool   `json:"primary"`
		} `json:"items"`
	}
	err := c.call(ctx, access, http.MethodGet, c.CalendarURL+"/users/me/calendarList?minAccessRole=writer&maxResults=250", nil, &list)
	if errors.Is(err, ErrCalendar) {
		return nil, ErrScope
	}
	if err != nil {
		return nil, err
	}
	var out []Calendar
	for _, it := range list.Items {
		name := it.Own
		if name == "" {
			name = it.Summary
		}
		out = append(out, Calendar{ID: it.ID, Name: name, Primary: it.Primary})
	}
	slices.SortStableFunc(out, func(a, b Calendar) int {
		switch {
		case a.Primary == b.Primary:
			return 0
		case a.Primary:
			return -1
		}
		return 1
	})
	return out, nil
}

// Write puts one event into the calendar: inserted, or, when Google already
// has an event with its ID (added before, or deleted by the Member and kept
// by Google as cancelled), replaced whole. It reports whether it was new.
func (c *Client) Write(ctx context.Context, access, calendarID, eventID string, event json.RawMessage) (bool, error) {
	base := c.CalendarURL + "/calendars/" + url.PathEscape(calendarID) + "/events"
	err := c.call(ctx, access, http.MethodPost, base, event, nil)
	var conflict *statusError
	if !errors.As(err, &conflict) || conflict.code != http.StatusConflict {
		return err == nil, err
	}
	return false, c.call(ctx, access, http.MethodPut, base+"/"+url.PathEscape(eventID), event, nil)
}

// Revoke ends the site's access at Google.
func (c *Client) Revoke(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RevokeURL, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("google: revoke: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	// 400 means the token was already invalid: the access is gone either way.
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("google: revoke: status %d", res.StatusCode)
	}
	return nil
}

type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("google: status %d", e.code) }

// call sends one Calendar or UserInfo request. A 401 is ErrExpired, a 403 or
// 404 is ErrCalendar (wrapping the status), anything else not 2xx an error
// that names only the status: Google's answers can echo what was sent.
func (c *Client) call(ctx context.Context, access, method, u string, body json.RawMessage, out any) error {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("google: %s: %w", method, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("google: read the answer: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return ErrExpired
	case res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w (%w)", ErrCalendar, &statusError{res.StatusCode})
	case res.StatusCode < 200 || res.StatusCode > 299:
		return &statusError{res.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("google: read the answer: %w", err)
	}
	return nil
}

// Seal encrypts a refresh token for the table with AES-256-GCM, bound to the
// Member it belongs to, so a sealed token copied to another Member's item
// won't open.
func Seal(key []byte, member, token string) (string, error) {
	gcm, err := aead(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(token), []byte(member))), nil
}

// Open reverses Seal.
func Open(key []byte, member, sealed string) (string, error) {
	gcm, err := aead(key)
	if err != nil {
		return "", err
	}
	data, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil || len(data) < gcm.NonceSize() {
		return "", ErrSealed
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(member))
	if err != nil {
		return "", ErrSealed
	}
	return string(plain), nil
}

func aead(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("google: the token key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
