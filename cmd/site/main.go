// The site server. With no arguments it serves HTTP: on Lambda through the
// API Gateway adapter, otherwise on a local port. With arguments it runs an
// Owner command, for example `site member add someone@example.com soccer`
// or `site profile kids`.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/CraigDevJohnson/the-lobby/internal/app"
	"github.com/CraigDevJohnson/the-lobby/internal/profile"
	"github.com/CraigDevJohnson/the-lobby/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx := context.Background()
	cfg := app.ConfigFromEnv()

	if len(os.Args) > 1 {
		if err := runCommand(ctx, cfg, os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	handler, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
		return
	}
	addr := os.Getenv("SITE_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Info("listening", "addr", "http://"+addr, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

const usage = `usage:
  site member add <email> [tool ...]   invite a Member, or set their Tools
  site member remove <email>           remove a Member (their Sign-in sessions stop working at once)
  site member list
  site profile kids [--out FILE]       write the kids iPad profile from KIDS_DOH_URL
  site profile adult [--out FILE] [--ca-file FILE]
                                       write the adult iPhone profile from ADULTS_DOH_URL and
                                       GATEWAY_ROOT_CA_PEM, or the certificate file named`

func runCommand(ctx context.Context, cfg app.Config, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", usage)
	}
	switch args[0] {
	case "member":
		return memberCommand(ctx, cfg, args[1:])
	case "profile":
		return profileCommand(cfg, args[1:])
	}
	return fmt.Errorf("%s", usage)
}

// memberCommand is the Owner's Member list (ADR 0004), run against the
// environment's own table.
func memberCommand(ctx context.Context, cfg app.Config, args []string) error {
	st, err := app.NewStore(ctx, cfg)
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("%s", usage)
		}
		m := store.Member{Email: store.NormalizeEmail(args[1]), Tools: args[2:], AddedAt: time.Now().UTC()}
		if !strings.Contains(m.Email, "@") {
			return fmt.Errorf("%q is not an email address", args[1])
		}
		if m.Tools == nil {
			m.Tools = []string{}
		}
		if err := st.PutMember(ctx, m); err != nil {
			return err
		}
		fmt.Printf("member %s: tools %v\n", m.Email, m.Tools)
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("%s", usage)
		}
		if err := st.DeleteMember(ctx, args[1]); err != nil {
			return err
		}
		fmt.Printf("removed %s\n", store.NormalizeEmail(args[1]))
	case "list":
		members, err := st.ListMembers(ctx)
		if err != nil {
			return err
		}
		for _, m := range members {
			fmt.Printf("%s\t%s\t%s\n", m.Email, strings.Join(m.Tools, ","), m.AddedAt.Format(time.RFC3339))
		}
	default:
		return fmt.Errorf("%s", usage)
	}
	return nil
}

// profileCommand writes one of the family device profiles (internal/profile)
// to disk, reading the same settings the server reads: KIDS_DOH_URL,
// ADULTS_DOH_URL and GATEWAY_ROOT_CA_PEM, the last of which --ca-file can
// supply from a file, since a PEM is awkward on a command line. The file is
// the profile's own name unless --out says otherwise; --out - writes to
// standard output. Nothing with a placeholder in it is ever written: an
// unset setting is an error that names it.
func profileCommand(cfg app.Config, args []string) error {
	which := args[0]
	flags := flag.NewFlagSet("site profile "+which, flag.ContinueOnError)
	out := flags.String("out", "", "where to write the profile (default: its own name in the current directory; - for standard output)")
	caFile := flags.String("ca-file", "", "read the Gateway root certificate (PEM) from this file instead of GATEWAY_ROOT_CA_PEM")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%s", usage)
	}
	var (
		name string
		data []byte
		err  error
	)
	switch which {
	case "kids":
		name = profile.KidsFileName
		data, err = profile.Kids{DoHURL: cfg.KidsDoHURL}.Render()
		if errors.Is(err, profile.ErrNotConfigured) {
			return fmt.Errorf("%w: set KIDS_DOH_URL to the Kids location's DNS over HTTPS address", err)
		}
	case "adult":
		name = profile.AdultFileName
		ca := cfg.GatewayRootCA
		if *caFile != "" {
			pemText, readErr := os.ReadFile(*caFile)
			if readErr != nil {
				return readErr
			}
			ca = string(pemText)
		}
		data, err = profile.Adult{DoHURL: cfg.AdultsDoHURL, GatewayRootCA: ca}.Render()
		if errors.Is(err, profile.ErrNotConfigured) {
			return fmt.Errorf("%w: set ADULTS_DOH_URL to the Adults location's DNS over HTTPS address and GATEWAY_ROOT_CA_PEM (or --ca-file) to the Gateway root certificate", err)
		}
	default:
		return fmt.Errorf("%s", usage)
	}
	if err != nil {
		return err
	}
	if *out == "" {
		*out = name
	}
	if *out == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, len(data))
	return nil
}
