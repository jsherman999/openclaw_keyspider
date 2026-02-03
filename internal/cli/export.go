package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/exporter"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
	"github.com/spf13/cobra"
)

func exportCmd(cfgPath *string) *cobra.Command {
	var format string
	var outPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export graph data (Phase 4: exports only)",
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

			if err := db.ApplyMigrations(ctx, dbConn); err != nil {
				return err
			}
			st := store.New(dbConn)

			var b []byte
			switch format {
			case "json":
				b, _, err = exporter.ExportGraphJSON(ctx, st, limit)
			case "csv":
				b, _, err = exporter.ExportGraphCSV(ctx, st, limit)
			case "graphml":
				b, _, err = exporter.ExportGraphGraphML(ctx, st, limit)
			default:
				return fmt.Errorf("unknown format %q (use json|csv|graphml)", format)
			}
			if err != nil {
				return err
			}

			if outPath == "" || outPath == "-" {
				_, _ = os.Stdout.Write(b)
				return nil
			}
			return os.WriteFile(outPath, b, 0644)
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "export format: json|csv|graphml")
	cmd.Flags().StringVar(&outPath, "out", "-", "output path (or - for stdout)")
	cmd.Flags().IntVar(&limit, "limit", 10000, "max rows for hosts/edges")
	return cmd
}
