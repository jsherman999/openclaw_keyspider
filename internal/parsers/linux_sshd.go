package parsers

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LinuxSSHDParser parses common OpenSSH sshd log formats.
// Phase 1 focuses on Accepted publickey lines.

type LinuxSSHDParser struct {
	now func() time.Time
}

type ParsedEvent struct {
	TS               time.Time
	DestUser         string
	SourceIP         string
	SourcePort       int
	FingerprintSHA256 string
	AuthMethod       string
	Result           string
}

func NewLinuxSSHDParser(now func() time.Time) *LinuxSSHDParser {
	return &LinuxSSHDParser{now: now}
}

var (
	// Example:
	// Feb  3 22:01:02 host sshd[123]: Accepted publickey for root from 1.2.3.4 port 2222 ssh2: ED25519 SHA256:Abc...
	reAcceptedPubkey = regexp.MustCompile(`(?i)Accepted\s+publickey\s+for\s+(?P<user>\S+)\s+from\s+(?P<ip>\S+)\s+port\s+(?P<port>\d+)\s+ssh2:.*?(SHA256:[A-Za-z0-9+/=_-]+)`) // permissive
)

func (p *LinuxSSHDParser) ParseLine(line string) (ParsedEvent, bool) {
	m := reAcceptedPubkey.FindStringSubmatch(line)
	if m == nil {
		return ParsedEvent{}, false
	}
	user := m[reAcceptedPubkey.SubexpIndex("user")]
	ip := m[reAcceptedPubkey.SubexpIndex("ip")]
	portStr := m[reAcceptedPubkey.SubexpIndex("port")]
	port, _ := strconv.Atoi(portStr)

	fp := ""
	for _, part := range strings.Fields(line) {
		if strings.HasPrefix(part, "SHA256:") {
			fp = part
			break
		}
	}

	return ParsedEvent{
		TS:                p.now().UTC(),
		DestUser:          user,
		SourceIP:          ip,
		SourcePort:        port,
		FingerprintSHA256: fp,
		AuthMethod:        "publickey",
		Result:            "accepted",
	}, true
}
