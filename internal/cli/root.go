package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/spider"
	"github.com/spf13/cobra"
)

func Main() {
	var cfgPath string

	root := &cobra.Command{
		Use:   "keyspider",
		Short: "Keyspider CLI",
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "config file (yaml)")

	root.AddCommand(scanCmd(&cfgPath))

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func scanCmd(cfgPath *string) *cobra.Command {
	var host string
	var since time.Duration

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a host for SSH access events and authorized_keys (phase 1)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			dbConn, err := db.Open(ctx, cfg.DB.DSN)
			if err != nil {
				return err
			}
			defer dbConn.Close()

			if err := db.ApplyMigrations(ctx, dbConn); err != nil {
				return err
			}

			sp := spider.New(cfg, dbConn)
			res, err := sp.ScanHost(ctx, host, since)
			if err != nil {
				return err
			}

			fmt.Printf("host=%s events_inserted=%d keys_seen=%d\n", host, res.EventsInserted, res.KeysSeen)
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "destination host to scan")
	cmd.Flags().DurationVar(&since, "since", 168*time.Hour, "how far back to scan logs")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}
