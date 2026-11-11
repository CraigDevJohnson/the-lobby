// Package app assembles the site server from its environment.
package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CraigDevJohnson/the-lobby/internal/access"
	"github.com/CraigDevJohnson/the-lobby/internal/backend"
	"github.com/CraigDevJohnson/the-lobby/internal/devices"
	"github.com/CraigDevJohnson/the-lobby/internal/foundry"
	"github.com/CraigDevJohnson/the-lobby/internal/google"
	"github.com/CraigDevJohnson/the-lobby/internal/minecraft"
	"github.com/CraigDevJohnson/the-lobby/internal/origin"
	"github.com/CraigDevJohnson/the-lobby/internal/profile"
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
	AccessAdminAUD   string // ACCESS_ADMIN_AUD: the /admin application's audience (ADR 0010)
	OriginSecret     string // ORIGIN_SECRET
	Table            string // SESSIONS_TABLE; empty means in-memory
	SoccerURL        string // SOCCER_BACKEND_URL: the Let's Plan Soccer backend
	SoccerSigning    string // SOCCER_BACKEND_SIGNING: lambda, off, or empty for automatic
	MinecraftURL     string // MINECRAFT_BACKEND_URL: the Minecraft Launcher's backend
	MinecraftSigning string // MINECRAFT_BACKEND_SIGNING: lambda, off, or empty for automatic
	FoundryURL       string // FOUNDRY_BACKEND_URL: Foundry's backend, the foundry repo's controller
	FoundrySigning   string // FOUNDRY_BACKEND_SIGNING: lambda, off, or empty for automatic
	FoundryJoinURL   string // FOUNDRY_JOIN_URL: Foundry's join page, offered when its status can't be checked
	// GOOGLE_PARAMETERS: the Parameter Store path, ending in "/", under
	// which Add to Google Calendar's client-id, client-secret and token-key
	// SecureStrings live. Unset means that method isn't offered.
	GoogleParameters string
	// The family device profiles (internal/profile): the two Gateway DNS
	// locations' DNS over HTTPS endpoints and the account's Gateway root
	// certificate as PEM. Unset means not configured yet.
	KidsDoHURL    string // KIDS_DOH_URL
	AdultsDoHURL  string // ADULTS_DOH_URL
	GatewayRootCA string // GATEWAY_ROOT_CA_PEM
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
		AccessAdminAUD:   os.Getenv("ACCESS_ADMIN_AUD"),
		OriginSecret:     os.Getenv("ORIGIN_SECRET"),
		Table:            os.Getenv("SESSIONS_TABLE"),
		SoccerURL:        os.Getenv("SOCCER_BACKEND_URL"),
		SoccerSigning:    os.Getenv("SOCCER_BACKEND_SIGNING"),
		MinecraftURL:     os.Getenv("MINECRAFT_BACKEND_URL"),
		MinecraftSigning: os.Getenv("MINECRAFT_BACKEND_SIGNING"),
		FoundryURL:       os.Getenv("FOUNDRY_BACKEND_URL"),
		FoundrySigning:   os.Getenv("FOUNDRY_BACKEND_SIGNING"),
		FoundryJoinURL:   os.Getenv("FOUNDRY_JOIN_URL"),
		GoogleParameters: os.Getenv("GOOGLE_PARAMETERS"),
		KidsDoHURL:       os.Getenv("KIDS_DOH_URL"),
		AdultsDoHURL:     os.Getenv("ADULTS_DOH_URL"),
		GatewayRootCA:    os.Getenv("GATEWAY_ROOT_CA_PEM"),
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
	r.Use(timeouts(8*time.Second, map[string]time.Duration{
		// Ten Teams' games are many calls to Google; the Lambda allows 30s.
		soccer.AddGooglePath: 25 * time.Second,
	}))
	r.Use(origin.Require(cfg.OriginSecret))
	h.Routes(r)
	tool, err := newSoccer(ctx, cfg, h, st, log)
	if err != nil {
		return nil, err
	}
	tool.Routes(r)
	launcher, err := newMinecraft(ctx, cfg, h, log)
	if err != nil {
		return nil, err
	}
	launcher.Routes(r)
	tabletop, err := newFoundry(ctx, cfg, h, log)
	if err != nil {
		return nil, err
	}
	tabletop.Routes(r)
	family, err := newDevices(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	family.Routes(r)
	return r, nil
}

// newDevices serves the family device profiles at /admin behind their own
// Access application (ADR 0010), named by ACCESS_ADMIN_AUD. Unset, the
// routes serve without the check on a developer's computer and are refused
// anywhere else. The profiles come from KIDS_DOH_URL, ADULTS_DOH_URL and
// GATEWAY_ROOT_CA_PEM; until those are set the page says they are not
// configured, and a wrong value is logged here rather than stopping the
// site, since the plan that set it should have refused it.
func newDevices(ctx context.Context, cfg Config, log *slog.Logger) (*devices.Handler, error) {
	h := &devices.Handler{
		Env:   cfg.Env,
		Kids:  profile.Kids{DoHURL: cfg.KidsDoHURL},
		Adult: profile.Adult{DoHURL: cfg.AdultsDoHURL, GatewayRootCA: cfg.GatewayRootCA},
		UI:    web.UI(),
		Log:   log,
	}
	for name, render := range map[string]func() ([]byte, error){"kids": h.Kids.Render, "adult": h.Adult.Render} {
		if _, err := render(); err != nil {
			if errors.Is(err, profile.ErrNotConfigured) {
				log.Warn("the family device profiles are not configured yet", "profile", name, "err", err)
			} else {
				log.Error("a family device profile setting is wrong", "profile", name, "err", err)
			}
		}
	}
	if cfg.AccessAdminAUD == "" {
		if cfg.Env == "local" {
			log.Warn("ACCESS_ADMIN_AUD is unset: the family devices page serves without an Access check")
		} else {
			log.Warn("ACCESS_ADMIN_AUD is unset: the family devices page is refused until its Access application exists")
		}
		return h, nil
	}
	v, err := access.NewCloudflare(ctx, cfg.AccessTeamDomain, cfg.AccessAdminAUD)
	if err != nil {
		return nil, err
	}
	h.Access = v
	return h, nil
}

// localSoccerURL is where the soccer repo's `task run` listens.
const localSoccerURL = "http://127.0.0.1:8081"

// newSoccer connects the Let's Plan Soccer backend. On a developer's
// computer that is the backend's local server, called plainly; anywhere else
// it is the backend's Lambda address, called with signed requests (ADR 0006).
// SOCCER_BACKEND_SIGNING overrides that: "lambda" signs even locally, to try
// the deployed backend from a developer's computer, and "off" never signs.
// My calendar is kept in the site's own table, st (ADR 0011).
func newSoccer(ctx context.Context, cfg Config, h *web.Handler, st store.Store, log *slog.Logger) (*soccer.Tool, error) {
	tool := &soccer.Tool{
		Backend: strings.TrimSuffix(cfg.SoccerURL, "/"),
		BaseURL: cfg.BaseURL,
		// Shorter than the request timeout, so a slow backend is answered
		// as unavailable rather than cut off.
		Client: &http.Client{Timeout: 7 * time.Second},
		Log:    log,
		Check:  soccerAccess(h),
		Store:  st,
	}
	if cfg.GoogleParameters != "" {
		tool.Google = google.New(&http.Client{Timeout: 10 * time.Second})
		tool.GoogleSettings = googleSettings(cfg.GoogleParameters)
	}
	local := cfg.Env == "local"
	sign, err := signing("SOCCER_BACKEND_SIGNING", cfg.SoccerSigning, local)
	if err != nil {
		return nil, err
	}
	if tool.Backend == "" && local {
		tool.Backend = localSoccerURL
	}
	if tool.Backend == "" {
		log.Warn("SOCCER_BACKEND_URL is unset: Let's Plan Soccer answers as unavailable")
		return tool, nil
	}
	if !sign {
		if !local {
			log.Warn("SOCCER_BACKEND_SIGNING is off: requests to the Let's Plan Soccer backend are not signed")
		}
		return tool, nil
	}
	tool.Sign, err = signer(ctx, "soccer")
	if err != nil {
		return nil, err
	}
	return tool, nil
}

// googleSettings reads Add to Google Calendar's client ID, client secret and
// token key from Parameter Store the first time they are needed, and keeps
// them once read. They never pass through the Lambda's environment, so a
// role that can read the function's settings and the table still can't open
// a Member's Google token.
func googleSettings(path string) soccer.GoogleSettings {
	var (
		mu  sync.Mutex
		got *google.Settings
	)
	return func(ctx context.Context) (google.Settings, error) {
		mu.Lock()
		defer mu.Unlock()
		if got != nil {
			return *got, nil
		}
		awscfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return google.Settings{}, fmt.Errorf("aws config: %w", err)
		}
		names := []string{path + "client-id", path + "client-secret", path + "token-key"}
		out, err := ssm.NewFromConfig(awscfg).GetParameters(ctx, &ssm.GetParametersInput{Names: names, WithDecryption: aws.Bool(true)})
		if err != nil {
			return google.Settings{}, fmt.Errorf("read the Google settings: %w", err)
		}
		if len(out.InvalidParameters) > 0 {
			return google.Settings{}, fmt.Errorf("the Google settings aren't all set: missing %s", strings.Join(out.InvalidParameters, ", "))
		}
		v := map[string]string{}
		for _, p := range out.Parameters {
			v[aws.ToString(p.Name)] = strings.TrimSpace(aws.ToString(p.Value))
		}
		key, err := base64.StdEncoding.DecodeString(v[names[2]])
		if err != nil || len(key) != 32 {
			return google.Settings{}, errors.New("the Google token key must be 32 bytes in base64")
		}
		got = &google.Settings{ClientID: v[names[0]], ClientSecret: v[names[1]], Key: key}
		return *got, nil
	}
}

// timeouts gives every request d to answer, except the paths in longer.
func timeouts(d time.Duration, longer map[string]time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		short := middleware.Timeout(d)(next)
		long := map[string]http.Handler{}
		for p, ld := range longer {
			long[p] = middleware.Timeout(ld)(next)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h, ok := long[r.URL.Path]; ok {
				h.ServeHTTP(w, r)
				return
			}
			short.ServeHTTP(w, r)
		})
	}
}

// soccerAccess answers My calendar's access check from the Sign-in session
// and the Member record, re-read on every request, as launcherAccess does.
func soccerAccess(h *web.Handler) func(http.ResponseWriter, *http.Request) (soccer.Access, string, error) {
	return func(w http.ResponseWriter, r *http.Request) (soccer.Access, string, error) {
		email, tools, err := h.Member(w, r)
		if err != nil {
			return soccer.SignedOut, "", err
		}
		if email == "" {
			return soccer.SignedOut, "", nil
		}
		if slices.Contains(tools, soccer.Grant) {
			return soccer.Granted, email, nil
		}
		return soccer.NoAccess, email, nil
	}
}

// newMinecraft connects the Minecraft Launcher's backend the same way as the
// Let's Plan Soccer backend, behind the Launcher's own access check. It has no
// local default: until MINECRAFT_BACKEND_URL names a backend, the page says
// no server is configured, on a developer's computer too.
func newMinecraft(ctx context.Context, cfg Config, h *web.Handler, log *slog.Logger) (*minecraft.Tool, error) {
	tool := &minecraft.Tool{
		Backend: strings.TrimSuffix(cfg.MinecraftURL, "/"),
		Check:   launcherAccess(h),
		Client:  &http.Client{Timeout: 7 * time.Second},
		Log:     log,
	}
	local := cfg.Env == "local"
	sign, err := signing("MINECRAFT_BACKEND_SIGNING", cfg.MinecraftSigning, local)
	if err != nil {
		return nil, err
	}
	if tool.Backend == "" {
		log.Warn("MINECRAFT_BACKEND_URL is unset: the Minecraft Launcher says no server is configured")
		return tool, nil
	}
	if !sign {
		if !local {
			log.Warn("MINECRAFT_BACKEND_SIGNING is off: requests to the Minecraft Launcher's backend are not signed")
		}
		return tool, nil
	}
	tool.Sign, err = signer(ctx, "minecraft")
	if err != nil {
		return nil, err
	}
	return tool, nil
}

// launcherAccess answers the Minecraft routes' access check for the grants a
// route accepts (the Launcher's, or Minecraft Admin's) from the Sign-in
// session and the Member record, re-read on every request.
func launcherAccess(h *web.Handler) func(http.ResponseWriter, *http.Request, ...string) (minecraft.Access, string, error) {
	return func(w http.ResponseWriter, r *http.Request, grants ...string) (minecraft.Access, string, error) {
		email, tools, err := h.Member(w, r)
		if err != nil {
			return minecraft.SignedOut, "", err
		}
		if email == "" {
			return minecraft.SignedOut, "", nil
		}
		for _, g := range grants {
			if slices.Contains(tools, g) {
				return minecraft.Granted, email, nil
			}
		}
		return minecraft.NoAccess, email, nil
	}
}

// newFoundry connects Foundry's backend, the foundry repo's controller, the
// same way as the Minecraft Launcher's, behind Foundry's own access check.
// It has no local default either: until FOUNDRY_BACKEND_URL names a backend,
// a Member with the grant is told no server is configured. FOUNDRY_JOIN_URL,
// when set, is held to the controller's rule for its join page.
func newFoundry(ctx context.Context, cfg Config, h *web.Handler, log *slog.Logger) (*foundry.Tool, error) {
	if cfg.FoundryJoinURL != "" && !foundry.ValidJoinURL(cfg.FoundryJoinURL) {
		return nil, fmt.Errorf("FOUNDRY_JOIN_URL %q: want exactly https://host[:port]/join, or unset", cfg.FoundryJoinURL)
	}
	tool := &foundry.Tool{
		Backend: strings.TrimSuffix(cfg.FoundryURL, "/"),
		Check:   foundryAccess(h),
		JoinURL: cfg.FoundryJoinURL,
		Client:  &http.Client{Timeout: 7 * time.Second},
		Log:     log,
	}
	local := cfg.Env == "local"
	sign, err := signing("FOUNDRY_BACKEND_SIGNING", cfg.FoundrySigning, local)
	if err != nil {
		return nil, err
	}
	if tool.Backend == "" {
		log.Warn("FOUNDRY_BACKEND_URL is unset: Foundry says no server is configured")
		return tool, nil
	}
	if !sign {
		if !local {
			log.Warn("FOUNDRY_BACKEND_SIGNING is off: requests to Foundry's backend are not signed")
		}
		return tool, nil
	}
	tool.Sign, err = signer(ctx, "foundry")
	if err != nil {
		return nil, err
	}
	return tool, nil
}

// foundryAccess answers Foundry's access check for the grants a route accepts
// from the Sign-in session and the Member record, re-read on every request,
// as launcherAccess does for the Launcher.
func foundryAccess(h *web.Handler) func(http.ResponseWriter, *http.Request, ...string) (foundry.Access, string, error) {
	return func(w http.ResponseWriter, r *http.Request, grants ...string) (foundry.Access, string, error) {
		email, tools, err := h.Member(w, r)
		if err != nil {
			return foundry.SignedOut, "", err
		}
		if email == "" {
			return foundry.SignedOut, "", nil
		}
		for _, t := range tools {
			if slices.Contains(grants, t) {
				return foundry.Granted, email, nil
			}
		}
		return foundry.NoAccess, email, nil
	}
}

// signing reads a backend's *_BACKEND_SIGNING setting: unset signs everywhere
// but a developer's computer, "lambda" always signs and "off" never does.
// Anything else stops the site from starting.
func signing(name, value string, local bool) (bool, error) {
	switch value {
	case "":
		return !local, nil
	case "lambda":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("%s %q: want lambda, off, or unset", name, value)
}

// signer signs requests to a Tool backend's Lambda address with the site's
// own AWS credentials (ADR 0006).
func signer(ctx context.Context, tool string) (func(*http.Request, []byte) error, error) {
	awscfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	if awscfg.Region == "" {
		return nil, fmt.Errorf("signing requests to the %s backend needs an AWS region (AWS_REGION)", tool)
	}
	return backend.SigV4(awscfg.Credentials, awscfg.Region), nil
}

// requestLogger writes one line per request. The query string is left out so
// that no secret in an address, such as a Member link, reaches this log, and
// a calendar link is logged as its route, without its token. This is the
// only log that records paths: API Gateway's access log in front of the
// Lambda leaves the path out for the same reason (#28), so when the request
// came through API Gateway the line carries its request id, "rid", and the
// two logs can be read together.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			path := loggedPath(r.URL.Path)
			attrs := []any{
				"method", r.Method,
				"path", path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"ms", time.Since(start).Milliseconds(),
				"ua", r.UserAgent(),
			}
			if rid := gatewayRequestID(r.Context()); rid != "" {
				attrs = append(attrs, "rid", rid)
			}
			log.Info("request", attrs...)
		})
	}
}

// gatewayRequestID is API Gateway's id for the request, the requestId its
// access log records, when the request came in through the Lambda adapter.
// On a developer's computer there is none.
func gatewayRequestID(ctx context.Context) string {
	if rc, ok := core.GetAPIGatewayV2ContextFromContext(ctx); ok {
		return rc.RequestID
	}
	return ""
}

// loggedPath is the path as the request log records it. Anything under the
// calendar link address is logged as the route, whatever the method or the
// outcome, so a Session link's token or a Member link's secret never reaches
// the request log. The two kinds of link are told apart, so the log still
// counts which calendar apps fetch each.
func loggedPath(path string) string {
	switch {
	case strings.HasPrefix(path, soccer.LinkPrefix+"member/"):
		return soccer.MemberLinkRoute
	case strings.HasPrefix(path, soccer.LinkPrefix):
		return soccer.LinkRoute
	}
	return path
}
