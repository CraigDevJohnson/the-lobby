// Package app assembles the site server from its environment.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/origin"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
	"github.com/CraigDevJohnson/the-lobby/internal/soccer"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
	"github.com/CraigDevJohnson/the-lobby/internal/web"
)

type Config struct {
	Env              string // SITE_ENV: dev, prod, or local
	BaseURL          string // SITE_BASE_URL
	AccessTeamDomain string // ACCESS_TEAM_DOMAIN
	AccessAUD        string // ACCESS_AUD
	OriginSecret     string // ORIGIN_SECRET
	Table            string // SESSIONS_TABLE; empty means in-memory
	SoccerURL        string // SOCCER_BACKEND_URL: the Schedule Downloader's backend
}

func ConfigFromEnv() Config {
	env := os.Getenv("SITE_ENV")
	if env == "" {
		env = "local"
	}
	return Config{
		Env:              env,
		BaseURL:          os.Getenv("SITE_BASE_URL"),
		AccessTeamDomain: os.Getenv("ACCESS_TEAM_DOMAIN"),
		AccessAUD:        os.Getenv("ACCESS_AUD"),
		OriginSecret:     os.Getenv("ORIGIN_SECRET"),
		Table:            os.Getenv("SESSIONS_TABLE"),
		SoccerURL:        os.Getenv("SOCCER_BACKEND_URL"),
	}
}

// NewStore picks DynamoDB when a table is configured, otherwise memory.
func NewStore(ctx context.Context, cfg Config) (store.Store, error) {
	if cfg.Table == "" {
		return store.NewMemory(), nil
	}
	awscfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return store.NewDynamo(dynamodb.NewFromConfig(awscfg), cfg.Table), nil
}

func New(ctx context.Context, cfg Config, log *slog.Logger) (http.Handler, error) {
	st, err := NewStore(ctx, cfg)
	if err != nil {
		return nil, err
	}
	var verifier access.Verifier
	if cfg.AccessTeamDomain != "" || cfg.AccessAUD != "" {
		v, err := access.NewCloudflare(ctx, cfg.AccessTeamDomain, cfg.AccessAUD)
		if err != nil {
			return nil, err
		}
		verifier = v
	} else {
		log.Warn("ACCESS_TEAM_DOMAIN and ACCESS_AUD are unset: sign-in is disabled")
	}
	if cfg.OriginSecret == "" {
		log.Warn("ORIGIN_SECRET is unset: requests that bypass Cloudflare are not refused")
	}
	h := &web.Handler{
		Env:      cfg.Env,
		Store:    st,
		Sessions: &session.Manager{Store: st, Secure: cfg.Env != "local"},
		Access:   verifier,
		UI:       web.UI(),
		Log:      log,
	}
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(requestLogger(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(8 * time.Second))
	r.Use(origin.Require(cfg.OriginSecret))
	h.Routes(r)
	tool, err := newSoccer(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	tool.Routes(r)
	return r, nil
}

// localSoccerURL is where the soccer repo's `task run` listens.
const localSoccerURL = "http://127.0.0.1:8081"

// newSoccer connects the Schedule Downloader's backend. On a developer's
// computer that is the backend's local server, called plainly; anywhere else
// it is the backend's Lambda address, called with signed requests (ADR 0006).
func newSoccer(ctx context.Context, cfg Config, log *slog.Logger) (*soccer.Tool, error) {
	tool := &soccer.Tool{
		Backend: strings.TrimSuffix(cfg.SoccerURL, "/"),
		BaseURL: cfg.BaseURL,
		// Shorter than the request timeout, so a slow backend is answered
		// as unavailable rather than cut off.
		Client: &http.Client{Timeout: 7 * time.Second},
		Log:    log,
	}
	switch {
	case cfg.Env == "local":
		if tool.Backend == "" {
			tool.Backend = localSoccerURL
		}
	case tool.Backend == "":
		log.Warn("SOCCER_BACKEND_URL is unset: the Schedule Downloader answers as unavailable")
	default:
		awscfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("aws config: %w", err)
		}
		tool.Sign = soccer.SigV4(awscfg.Credentials, awscfg.Region)
	}
	return tool, nil
}

// requestLogger writes one line per request. The query string is left out so
// that no secret in an address, such as a Member link, reaches the logs, and
// a calendar link is logged as its route, without its token.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			path := loggedPath(r.URL.Path)
			log.Info("request",
				"method", r.Method,
				"path", path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"ms", time.Since(start).Milliseconds(),
				"ua", r.UserAgent(),
			)
		})
	}
}

// loggedPath is the path as the request log records it. Anything under the
// calendar link address is logged as the route, whatever the method or the
// outcome, so a link's token never reaches the logs.
func loggedPath(path string) string {
	if strings.HasPrefix(path, soccer.LinkPrefix) {
		return soccer.LinkRoute
	}
	return path
}
