package spider

import (
	"context"
	"fmt"
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
	HostsVisited   int
	EdgesUpserted  int
	ConcernsRaised int
}

func New(cfg *config.Config, dbc *db.DB) *Spider {
	return &Spider{cfg: cfg, db: dbc, store: store.New(dbc), ssh: sshclient.New(cfg)}
}

func (s *Spider) ScanHost(ctx context.Context, destHost string, since time.Duration, spiderDepth int) (*ScanResult, error) {
	// Phase 2: BFS spider expansion from the jump server only.
	// The only "identity" resolution is DNS (reverse + forward best-effort).
	type item struct {
		host  string
		depth int
	}
	queue := []item{{host: destHost, depth: 0}}
	visited := map[string]bool{}

	res := &ScanResult{}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		if visited[it.host] {
			continue
		}
		visited[it.host] = true
		res.HostsVisited++

		// Determine reachability from jump server.
		reachable := s.ssh.CanConnect(ctx, it.host)

		destID, err := s.store.UpsertHost(ctx, it.host, nil, "linux", reachable)
		if err != nil {
			return nil, err
		}
		if !reachable {
			res.ConcernsRaised++
			_, _ = s.store.InsertConcern(ctx, "high", "UNREACHABLE_HOST", &destID, nil, nil, "jump server cannot ssh to host")
			continue
		}

		logText, err := s.fetchSSHDLogs(ctx, it.host, since)
		if err != nil {
			return nil, err
		}

		p := parsers.NewLinuxSSHDParser(time.Now)
		inserted, edgesUp, concerns, sources := s.ingestLogs(ctx, destID, logText, p)
		res.EventsInserted += inserted
		res.EdgesUpserted += edgesUp
		res.ConcernsRaised += concerns

		keysSeen, err := s.scanAuthorizedKeysAndPersist(ctx, destID, it.host)
		if err != nil {
			return nil, err
		}
		res.KeysSeen += keysSeen

		// Key hunt for sources (private key locations only; no key contents stored).
		// Note: This is best-effort and bounded by allow_roots.
		if s.cfg.KeyHunt.Enabled {
			for _, src := range sources {
				_ = s.bestEffortKeyHunt(ctx, src)
			}
		}

		if it.depth < spiderDepth {
			for _, src := range sources {
				if src == "" {
					continue
				}
				queue = append(queue, item{host: src, depth: it.depth + 1})
			}
		}
	}

	return res, nil
}

func (s *Spider) fetchSSHDLogs(ctx context.Context, host string, since time.Duration) (string, error) {
	// Prefer journalctl if available; otherwise fall back to common files.
	sinceArg := fmt.Sprintf("--since '%dm'", int(since.Minutes()))
	cmd := "sh -lc \"(command -v journalctl >/dev/null 2>&1 && journalctl -u ssh -u sshd " + sinceArg + " --no-pager) || (test -r /var/log/secure && tail -n 20000 /var/log/secure) || (test -r /var/log/auth.log && tail -n 20000 /var/log/auth.log)\""
	return s.ssh.Run(ctx, host, cmd)
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
