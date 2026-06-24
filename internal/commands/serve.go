package commands

import (
	"os/signal"
	"strings"
	"syscall"

	"github.com/Sanjays2402/tsk/internal/serve"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var addr string
	var token string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the web UI + JSON API over .tsk.md",
		Long: `serve starts a small local HTTP server that exposes the same .tsk.md
through a web UI (bundled into the binary) and a JSON API. The TUI and CLI
continue to work; the web UI is just another surface over the same file.

Defaults to binding 127.0.0.1:7878 — local only, no auth. Pass --addr to
bind elsewhere; when you do, pass --token to require a bearer token on every
/api/* request (clients send "Authorization: Bearer <token>"; a browser
authenticates once by opening the printed ?token= link, which sets a session
cookie). Without --token an off-loopback bind is unauthenticated — only do
that on a trusted network.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			file, _ := cmd.Flags().GetString("file")
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			srv := serve.New(serve.Options{
				Addr:     addr,
				File:     file,
				TZ:       ResolveTZ(),
				Token:    token,
				StaticFS: serve.EmbeddedSPA(),
			})
			out := cmd.OutOrStdout()
			if token != "" {
				pf(out, "tsk serve listening on http://%s (token auth ON)\n", srv.Addr())
				pf(out, "open: http://%s/?token=%s\n", srv.Addr(), token)
			} else {
				pf(out, "tsk serve listening on http://%s\n", srv.Addr())
				if !isLoopbackAddr(addr) {
					pln(out, "warning: bound off-loopback with no --token; the API is unauthenticated")
				}
			}
			pln(out, "ctrl-c to stop")
			return srv.ListenAndServe(ctx)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7878", "address to bind (host:port)")
	cmd.Flags().StringVar(&token, "token", "", "require this bearer token on /api/* (recommended when binding off 127.0.0.1)")
	return cmd
}

// isLoopbackAddr reports whether a host:port binds only the local loopback,
// where running without a token is safe. Anything else (0.0.0.0, a LAN IP, a
// bare port) is treated as off-loopback so we can warn about unauthenticated
// exposure.
func isLoopbackAddr(addr string) bool {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	host = strings.TrimSpace(host)
	switch host {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	}
	return false
}
