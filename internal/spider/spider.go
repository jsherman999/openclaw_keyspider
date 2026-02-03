package spider

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
	"github.com/jsherman999/openclaw_keyspider/internal/db"
	"github.com/jsherman999/openclaw_keyspider/internal/parsers"
	"github.com/jsherman999/openclaw_keyspider/internal/sshclient"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
)

type Spider struct {
	cfg   *config.Config
	db    *db.DB
	store *store.Store
	ssh   *sshclient.Client
}

type ScanResult struct {
	EventsInserted int
	KeysSeen       int
}

func New(cfg *config.Config, dbc *db.DB) *Spider {
	return &Spider{cfg: cfg, db: dbc, store: store.New(dbc), ssh: sshclient.New(cfg)}
}

func (s *Spider) ScanHost(ctx context.Context, destHost string, since time.Duration) (*ScanResult, error) {
	// Phase 1: log pull + parse + store access events.
	destID, err := s.store.UpsertHost(ctx, destHost, nil, "linux", true)
	if err != nil {
		return nil, err
	}

	logText, err := s.fetchSSHDLogs(ctx, destHost, since)
	if err != nil {
		return nil, err
	}

	p := parsers.NewLinuxSSHDParser(time.Now)
	var inserted int

	scanner := bufio.NewScanner(strings.NewReader(logText))
	for scanner.Scan() {
		line := scanner.Text()
		ev, ok := p.ParseLine(line)
		if !ok {
			continue
		}

		storeEv := &store.AccessEvent{
			TS:         ev.TS,
			DestHostID: destID,
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

		// DNS enrichment (reverse lookup) into source_host label.
		if s.cfg.Discovery.DNS.Enabled && ev.SourceIP != "" {
			if names, _ := net.LookupAddr(ev.SourceIP); len(names) > 0 {
				name := strings.TrimSuffix(names[0], ".")
				storeEv.SourceHost = &name
			}
		}

		if _, err := s.store.InsertAccessEvent(ctx, storeEv); err != nil {
			return nil, err
		}
		inserted++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan logs: %w", err)
	}

	// Phase 1: authorized_keys scan (placeholder: count only; mapping will come next)
	keysSeen, _ := s.scanAuthorizedKeys(ctx, destHost)

	return &ScanResult{EventsInserted: inserted, KeysSeen: keysSeen}, nil
}

func (s *Spider) fetchSSHDLogs(ctx context.Context, host string, since time.Duration) (string, error) {
	// Prefer journalctl if available; otherwise fall back to common files.
	sinceArg := fmt.Sprintf("--since '%dm'", int(since.Minutes()))
	cmd := "sh -lc \"(command -v journalctl >/dev/null 2>&1 && journalctl -u ssh -u sshd " + sinceArg + " --no-pager) || (test -r /var/log/secure && tail -n 20000 /var/log/secure) || (test -r /var/log/auth.log && tail -n 20000 /var/log/auth.log)\""
	return s.ssh.Run(ctx, host, cmd)
}

func (s *Spider) scanAuthorizedKeys(ctx context.Context, host string) (int, error) {
	cmd := `sh -lc 'for f in /root/.ssh/authorized_keys /home/*/.ssh/authorized_keys; do [ -r "$f" ] || continue; echo "--- $f"; cat "$f"; done'`
	out, err := s.ssh.Run(ctx, host, cmd)
	if err != nil {
		return 0, err
	}
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "ssh-") || strings.HasPrefix(line, "ecdsa-") {
			count++
		}
	}
	return count, scanner.Err()
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

// ensure net is used (go vet-friendly)
var _ = net.IP{}
