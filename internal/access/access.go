// Package access checks the token Cloudflare Access attaches to a request
// after it has proved who the person is (ADR 0004). The site trusts the token
// only for the /signin path, then issues its own Sign-in session.
package access

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Header carries the Access token on requests that passed through Access.
const Header = "Cf-Access-Jwt-Assertion"

type Identity struct {
	Email string
}

type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

// Cloudflare verifies tokens against the team's published signing keys.
type Cloudflare struct {
	verifier *oidc.IDTokenVerifier
}

// NewCloudflare takes the team domain (https://<team>.cloudflareaccess.com)
// and the application's audience tag, both known to the infrastructure.
func NewCloudflare(ctx context.Context, teamDomain, aud string) (*Cloudflare, error) {
	teamDomain = strings.TrimRight(teamDomain, "/")
	if teamDomain == "" || aud == "" {
		return nil, errors.New("access: team domain and audience are required")
	}
	keys := oidc.NewRemoteKeySet(ctx, teamDomain+"/cdn-cgi/access/certs")
	v := oidc.NewVerifier(teamDomain, keys, &oidc.Config{ClientID: aud})
	return &Cloudflare{verifier: v}, nil
}

func (c *Cloudflare) Verify(ctx context.Context, token string) (Identity, error) {
	idt, err := c.verifier.Verify(ctx, token)
	if err != nil {
		return Identity{}, fmt.Errorf("access: %w", err)
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := idt.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("access: claims: %w", err)
	}
	if claims.Email == "" {
		return Identity{}, errors.New("access: token has no email")
	}
	return Identity{Email: claims.Email}, nil
}

// TokenFromRequest finds the Access token, in the header Cloudflare sets or
// the cookie it leaves in the browser.
func TokenFromRequest(r *http.Request) string {
	if t := r.Header.Get(Header); t != "" {
		return t
	}
	if c, err := r.Cookie("CF_Authorization"); err == nil {
		return c.Value
	}
	return ""
}
