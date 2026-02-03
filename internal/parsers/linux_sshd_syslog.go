package parsers

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parse syslog-style timestamps like: "Feb  3 22:01:02".
func parseSyslogTS(now time.Time, prefix string) (time.Time, bool) {
	// month day hh:mm:ss (day may be space-padded)
	parts := strings.Fields(prefix)
	if len(parts) < 3 {
		return time.Time{}, false
	}
	monthStr, dayStr, hms := parts[0], parts[1], parts[2]
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return time.Time{}, false
	}
	hmsParts := strings.Split(hms, ":")
	if len(hmsParts) != 3 {
		return time.Time{}, false
	}
	hh, _ := strconv.Atoi(hmsParts[0])
	mm, _ := strconv.Atoi(hmsParts[1])
	ss, _ := strconv.Atoi(hmsParts[2])

	month := map[string]time.Month{"Jan": time.January, "Feb": time.February, "Mar": time.March, "Apr": time.April, "May": time.May, "Jun": time.June, "Jul": time.July, "Aug": time.August, "Sep": time.September, "Oct": time.October, "Nov": time.November, "Dec": time.December}[monthStr]
	if month == 0 {
		return time.Time{}, false
	}

	loc := now.Location()
	ts := time.Date(now.Year(), month, day, hh, mm, ss, 0, loc)
	// If this timestamp appears in the future by >24h, assume it was from last year (year roll-over).
	if ts.After(now.Add(24 * time.Hour)) {
		ts = ts.AddDate(-1, 0, 0)
	}
	return ts.UTC(), true
}

var (
	reAcceptedPubkeyISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.*?Accepted\s+publickey\s+for\s+(?P<user>\S+)\s+from\s+(?P<ip>\S+)\s+port\s+(?P<port>\d+).*?(SHA256:[A-Za-z0-9+/=_-]+)`) // journal short-iso includes ISO timestamp
)

// ParseLineEnhanced parses both syslog-style and journalctl --output=short-iso lines.
func (p *LinuxSSHDParser) ParseLineEnhanced(line string) (ParsedEvent, bool) {
	// journalctl short-iso: "2026-02-03T22:01:02-0500 host sshd[...]: Accepted ... SHA256:..."
	if m := reAcceptedPubkeyISO.FindStringSubmatch(line); m != nil {
		// ISO timestamp is up to first space
		first := strings.Fields(line)
		if len(first) > 0 {
			if ts, err := time.Parse(time.RFC3339, first[0]); err == nil {
				user := m[reAcceptedPubkeyISO.SubexpIndex("user")]
				ip := m[reAcceptedPubkeyISO.SubexpIndex("ip")]
				port, _ := strconv.Atoi(m[reAcceptedPubkeyISO.SubexpIndex("port")])
				fp := ""
				for _, part := range strings.Fields(line) {
					if strings.HasPrefix(part, "SHA256:") {
						fp = part
						break
					}
				}
				return ParsedEvent{TS: ts.UTC(), DestUser: user, SourceIP: ip, SourcePort: port, FingerprintSHA256: fp, AuthMethod: "publickey", Result: "accepted"}, true
			}
		}
	}

	// syslog: "Feb  3 22:01:02 ... Accepted publickey ..."
	// Use first 15 chars for timestamp prefix.
	if len(line) >= 15 {
		ts, ok := parseSyslogTS(p.now().In(time.Local), line[:15])
		if ok {
			// Use existing regex for the rest.
			ev, ok2 := p.ParseLine(line)
			if ok2 {
				ev.TS = ts
				return ev, true
			}
		}
	}

	return ParsedEvent{}, false
}
