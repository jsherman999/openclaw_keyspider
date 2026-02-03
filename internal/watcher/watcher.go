package watcher

import (
	"context"
	"log"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/spider"
)

// Watcher is a minimal phase-3 implementation:
// - For each configured host, periodically re-scan recent sshd logs.
// - This is not a true tail -f yet; it's a safe approximation that works everywhere.
// - Events remain scrollable/searchable via DB + API.
//
// Later we can upgrade to ssh + journalctl -f/tail -F with per-host cursors.

type Watcher struct {
	cfg *config.Config
	db  *db.DB
	sp  *spider.Spider
}

func New(cfg *config.Config, dbc *db.DB) *Watcher {
	return &Watcher{cfg: cfg, db: dbc, sp: spider.New(cfg, dbc)}
}

func (w *Watcher) Run(ctx context.Context) {
	if !w.cfg.Watcher.Enabled {
		return
	}
	if len(w.cfg.Watcher.Hosts) == 0 {
		log.Printf("watcher enabled but watcher.hosts empty")
		return
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, h := range w.cfg.Watcher.Hosts {
				// Scan only last 2 minutes to approximate tail.
				_, _ = w.sp.ScanHost(ctx, h, 2*time.Minute, 0)
			}
		}
	}
}
