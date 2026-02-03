package keys

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type PublicKey struct {
	Type       string
	Authorized string // full authorized_keys line sans options ("ssh-ed25519 AAAA... comment")
	Comment    string
	FP256      string // "SHA256:..."
}

// ParseAuthorizedKeysLine parses a single authorized_keys line and returns the key payload.
// It ignores options (command=,from=,etc) if present.
func ParseAuthorizedKeysLine(line string) (*PublicKey, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, false
	}

	// ssh.ParseAuthorizedKey handles options + key + comment.
	pk, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return nil, false
	}

	auth := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk)))
	// MarshalAuthorizedKey doesn't include the comment, so re-append if provided.
	if comment != "" {
		auth = auth + " " + comment
	}

	return &PublicKey{
		Type:       pk.Type(),
		Authorized: auth,
		Comment:    comment,
		FP256:      ssh.FingerprintSHA256(pk),
	}, true
}

// ParseAuthorizedKeysFile parses the full file content.
func ParseAuthorizedKeysFile(content string) []*PublicKey {
	var out []*PublicKey
	s := bufio.NewScanner(strings.NewReader(content))
	for s.Scan() {
		if k, ok := ParseAuthorizedKeysLine(s.Text()); ok {
			out = append(out, k)
		}
	}
	return out
}

func FingerprintFromAuthorizedKey(authorized string) (string, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorized))
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(pk), nil
}

func NormalizeAuthorizedKey(authorized string) (string, error) {
	pk, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(authorized))
	if err != nil {
		return "", err
	}
	b := bytes.TrimSpace(ssh.MarshalAuthorizedKey(pk))
	out := string(b)
	if comment != "" {
		out += " " + comment
	}
	return out, nil
}

func DebugKey(k *PublicKey) string {
	return fmt.Sprintf("%s %s %q", k.Type, k.FP256, k.Comment)
}
