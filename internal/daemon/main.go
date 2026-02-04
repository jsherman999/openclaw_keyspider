package daemon

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/api"
	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/watcher"
	"github.com/jsherman999/openclaw_keyspider/internal/watchhub"
	"github.com/jsherman999/openclaw_keyspider/internal/worker"
	"github.com/spf13/cobra"
)

func Main() {
	var cfgPath string

	root := &cobra.Command{Use: "keyspiderd", Short: "Keyspider daemon (API + workers)"}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "config file (yaml)")

	root.AddCommand(migrateCmd(&cfgPath))
	root.AddCommand(serveCmd(&cfgPath))

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func migrateCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			dbConn, err := db.Open(ctx, cfg.DB.DSN)
			if err != nil {
				return err
			}
			defer dbConn.Close()
			return db.ApplyMigrations(ctx, dbConn)
		},
	}
}

func serveCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}

			ctx := context.Background()
			dbConn, err := db.Open(ctx, cfg.DB.DSN)
			if err != nil {
				return err
			}
			defer dbConn.Close()
			if err := db.ApplyMigrations(ctx, dbConn); err != nil {
				return err
			}

			hub := watchhub.New()
			h := api.New(cfg, dbConn, hub)
			srv := &http.Server{Addr: cfg.API.Listen, Handler: h.Router()}

			bgCtx, bgCancel := context.WithCancel(context.Background())
			defer bgCancel()

			// Phase 3 watcher (streaming)
			go func() {
				w := watcher.New(cfg, dbConn, hub)
				w.Run(bgCtx)
			}()

			// Background scan worker (web/UI-triggered scan jobs)
			go func() {
				sw := worker.NewScanWorker(cfg, dbConn)
				sw.Run(bgCtx)
			}()

			go func() {
				log.Printf("keyspiderd listening on %s", cfg.API.Listen)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Printf("listen error: %v", err)
				}
			}()

			stop := make(chan os.Signal, 2)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
			<-stop
			log.Printf("shutting down")

			shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := srv.Shutdown(shCtx); err != nil {
				return fmt.Errorf("shutdown: %w", err)
			}
			return nil
		},
	}
}
