// Package app assembles the site server from its environment.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/origin"
	"github.com/CraigDevJohnson/the-lobby/internal/session"
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
	return r, nil
}

// requestLogger writes one line per request. The query string is left out so
// that no secret in an address, such as a Member link, reaches the logs.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"ms", time.Since(start).Milliseconds(),
				"ua", r.UserAgent(),
			)
		})
	}
}
