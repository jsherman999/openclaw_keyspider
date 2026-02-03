package spider

import (
	"bufio"
	"context"
	"net"
	"strings"

	"github.com/jsherman999/openclaw_keyspider/internal/parsers"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
)

func (s *Spider) ingestLogs(ctx context.Context, destID int64, logText string, p *parsers.LinuxSSHDParser) (inserted int, edgesUp int, concerns int, sources []string) {
	scanner := bufio.NewScanner(strings.NewReader(logText))
	sourceSet := map[string]bool{}

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
		srcLabel := ""
		if s.cfg.Discovery.DNS.Enabled && ev.SourceIP != "" {
			if names, _ := net.LookupAddr(ev.SourceIP); len(names) > 0 {
				srcLabel = strings.TrimSuffix(names[0], ".")
				storeEv.SourceHost = &srcLabel
			}
		}
		if srcLabel == "" {
			srcLabel = ev.SourceIP
		}

		id, err := s.store.InsertAccessEvent(ctx, storeEv)
		if err != nil {
			// best effort: keep going
			continue
		}
		inserted++

		// Edge
		var srcHostID *int64
		if srcLabel != "" {
			// If DNS gives us a hostname, record it as a host and probe reachability.
			if strings.Contains(srcLabel, ".") {
				reach := s.ssh.CanConnect(ctx, srcLabel)
				hid, _ := s.store.UpsertHost(ctx, srcLabel, &srcLabel, "linux", reach)
				srcHostID = &hid
				if !reach {
					concerns++
					_, _ = s.store.InsertConcern(ctx, "high", "UNREACHABLE_SOURCE", &hid, nil, &id, "source seen in logs but not reachable from jump")
				}
			}
			if _, err := s.store.UpsertEdge(ctx, srcHostID, srcLabel, destID, "log", 80); err == nil {
				edgesUp++
			}
		}

		if srcLabel != "" {
			sourceSet[srcLabel] = true
		}
	}

	for k := range sourceSet {
		sources = append(sources, k)
	}
	return inserted, edgesUp, concerns, sources
}
