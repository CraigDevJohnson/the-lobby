// The site server. With no arguments it serves HTTP: on Lambda through the
// API Gateway adapter, otherwise on a local port. With arguments it runs an
// Owner command, for example `site member add someone@example.com soccer`.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/CraigDevJohnson/website/internal/app"
	"github.com/CraigDevJohnson/website/internal/store"
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
  site member list`

func runCommand(ctx context.Context, cfg app.Config, args []string) error {
	st, err := app.NewStore(ctx, cfg)
	if err != nil {
		return err
	}
	if len(args) < 2 || args[0] != "member" {
		return fmt.Errorf("%s", usage)
	}
	switch args[1] {
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("%s", usage)
		}
		m := store.Member{Email: store.NormalizeEmail(args[2]), Tools: args[3:], AddedAt: time.Now().UTC()}
		if !strings.Contains(m.Email, "@") {
			return fmt.Errorf("%q is not an email address", args[2])
		}
		if m.Tools == nil {
			m.Tools = []string{}
		}
		if err := st.PutMember(ctx, m); err != nil {
			return err
		}
		fmt.Printf("member %s: tools %v\n", m.Email, m.Tools)
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("%s", usage)
		}
		if err := st.DeleteMember(ctx, args[2]); err != nil {
			return err
		}
		fmt.Printf("removed %s\n", store.NormalizeEmail(args[2]))
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
