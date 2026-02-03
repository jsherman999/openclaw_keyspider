package watcher

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/parsers"
	"github.com/jsherman999/openclaw_keyspider/internal/sshclient"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
	"github.com/jsherman999/openclaw_keyspider/internal/watchhub"
)

// Phase 3 tightened watcher:
// - True streaming mode using ssh + journalctl -f (preferred) or tail -F.
// - Best-effort cursor tracking for journalctl (stores cursor in DB).
// - Publishes inserted events to an in-process hub for SSE.

type Watcher struct {
	cfg   *config.Config
	db    *db.DB
	st    *store.Store
	ssh   *sshclient.Client
	hub   *watchhub.Hub
	parse *parsers.LinuxSSHDParser
}

func New(cfg *config.Config, dbc *db.DB, hub *watchhub.Hub) *Watcher {
	return &Watcher{cfg: cfg, db: dbc, st: store.New(dbc), ssh: sshclient.New(cfg), hub: hub, parse: parsers.NewLinuxSSHDParser(time.Now)}
}

func (w *Watcher) Run(ctx context.Context) {
	if !w.cfg.Watcher.Enabled {
		return
	}
	if len(w.cfg.Watcher.Hosts) == 0 {
		log.Printf("watcher enabled but watcher.hosts empty")
		return
	}

	for _, host := range w.cfg.Watcher.Hosts {
		h := host
		go w.watchHost(ctx, h)
	}

	<-ctx.Done()
}

func (w *Watcher) watchHost(ctx context.Context, host string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !w.ssh.CanConnect(ctx, host) {
			// Backoff; also mark host unreachable.
			hid, _ := w.st.UpsertHost(ctx, host, &host, "linux", false)
			_, _ = w.st.InsertConcern(ctx, "high", "UNREACHABLE_HOST", &hid, nil, nil, "watcher cannot ssh to host")
			time.Sleep(10 * time.Second)
			continue
		}

		hid, _ := w.st.UpsertHost(ctx, host, &host, "linux", true)
		_ = w.st.EnsureWatcher(ctx, hid, "auto")

		state, _ := w.st.GetWatcherState(ctx, hid)
		mode := "auto"
		if state != nil && state.Mode != "" {
			mode = state.Mode
		}

		// Decide command.
		useJournal := mode == "journal" || mode == "auto"
		if useJournal {
			err := w.streamJournal(ctx, host, hid, state)
			if err == nil {
				continue
			}
			log.Printf("watcher(%s): journal stream failed, falling back to tail: %v", host, err)
		}

		_ = w.streamTail(ctx, host, hid)
		// If stream ends, retry.
		time.Sleep(2 * time.Second)
	}
}

func (w *Watcher) streamJournal(ctx context.Context, host string, hostID int64, state *store.WatcherState) error {
	// Use journalctl short-iso and show cursor, so we can resume.
	cursorArg := ""
	if state != nil && state.Cursor != nil && *state.Cursor != "" {
		cursorArg = "--after-cursor '" + *state.Cursor + "'"
	} else {
		cursorArg = "--since '2 minutes ago'"
	}

	cmd := "sh -lc \"command -v journalctl >/dev/null 2>&1 || exit 2; journalctl -f -u ssh -u sshd --no-pager --output=short-iso --show-cursor " + cursorArg + "\""

	var lastCursor string
	err := w.ssh.Stream(ctx, host, cmd, func(line string) bool {
		// journalctl cursor lines look like: "-- cursor: s=..."
		if strings.HasPrefix(line, "-- cursor:") {
			lastCursor = strings.TrimSpace(strings.TrimPrefix(line, "-- cursor:"))
			_ = w.st.UpdateWatcherCursor(ctx, hostID, lastCursor)
			return true
		}
		w.handleLogLine(ctx, hostID, host, line)
		return true
	})
	return err
}

func (w *Watcher) streamTail(ctx context.Context, host string, hostID int64) error {
	// Tail common log files.
	cmd := `sh -lc '
if [ -r /var/log/secure ]; then tail -n 0 -F /var/log/secure; exit $?; fi
if [ -r /var/log/auth.log ]; then tail -n 0 -F /var/log/auth.log; exit $?; fi
exit 2
'`
	return w.ssh.Stream(ctx, host, cmd, func(line string) bool {
		w.handleLogLine(ctx, hostID, host, line)
		return true
	})
}

func (w *Watcher) handleLogLine(ctx context.Context, hostID int64, host string, line string) {
	ev, ok := w.parse.ParseLineEnhanced(line)
	if !ok {
		return
	}

	storeEv := &store.AccessEvent{
		TS:         ev.TS,
		DestHostID: hostID,
		DestUser:   ptr(ev.DestUser),
		SourceIP:   ptr(ev.SourceIP),
		SourcePort: ptrInt(ev.SourcePort),
		Fingerprint: func() *string {
			if ev.FingerprintSHA256 == "" {
				return nil
			}
			return &ev.FingerprintSHA256
		}(),
		AuthMethod: ptr(ev.AuthMethod),
		Result:     ptr(ev.Result),
		RawLine:    line,
	}

	id, err := w.st.InsertAccessEvent(ctx, storeEv)
	if err != nil {
		return
	}

	// minimal edge update; label is IP until DNS enrichment via spider scan.
	srcLabel := ev.SourceIP
	var srcHostID *int64
	if strings.Contains(srcLabel, ".") {
		reach := w.ssh.CanConnect(ctx, srcLabel)
		hid, _ := w.st.UpsertHost(ctx, srcLabel, &srcLabel, "linux", reach)
		srcHostID = &hid
		if !reach {
			_, _ = w.st.InsertConcern(ctx, "high", "UNREACHABLE_SOURCE", &hid, nil, &id, "source seen by watcher but not reachable from jump")
		}
	}
	_, _ = w.st.UpsertEdge(ctx, srcHostID, srcLabel, hostID, "log", 80)

	// Publish SSE payload
	payload := map[string]any{
		"access_event_id": id,
		"dest_host":       host,
		"ts":              ev.TS,
		"dest_user":       ev.DestUser,
		"source_ip":       ev.SourceIP,
		"source_port":     ev.SourcePort,
		"fingerprint":     ev.FingerprintSHA256,
		"raw":             line,
	}
	if b, err := json.Marshal(payload); err == nil {
		w.hub.Publish(b)
	}
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrInt(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
