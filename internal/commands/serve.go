package commands

import (
	"os/signal"
	"syscall"

	"github.com/Sanjays2402/tsk/internal/serve"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the web UI + JSON API over .tsk.md",
		Long: `serve starts a small local HTTP server that exposes the same .tsk.md
through a web UI (bundled into the binary) and a JSON API. The TUI and CLI
continue to work; the web UI is just another surface over the same file.

Defaults to binding 127.0.0.1:7878 — local only, no auth. Pass --addr to
override, but bind off-loopback at your own risk; token auth is on the
roadmap, not in this build.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			file, _ := cmd.Flags().GetString("file")
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			srv := serve.New(serve.Options{
				Addr:     addr,
				File:     file,
				TZ:       ResolveTZ(),
				StaticFS: serve.EmbeddedSPA(),
			})
			pf(cmd.OutOrStdout(), "tsk serve listening on http://%s\n", srv.Addr())
			pln(cmd.OutOrStdout(), "ctrl-c to stop")
			return srv.ListenAndServe(ctx)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7878", "address to bind (host:port)")
	return cmd
}
