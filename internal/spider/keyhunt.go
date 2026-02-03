package spider

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jsherman999/openclaw_keyspider/internal/keys"
	"github.com/jsherman999/openclaw_keyspider/internal/store"
)

// bestEffortKeyHunt tries to locate private key files on a source host (reachable from jump only).
// It does NOT pull private key contents; it only records paths and (if ssh-keygen works) the derived public fingerprint.
func (s *Spider) bestEffortKeyHunt(ctx context.Context, sourceHost string) error {
	if sourceHost == "" {
		return nil
	}
	if !s.ssh.CanConnect(ctx, sourceHost) {
		// unreachable sources are handled via concerns in ingestLogs.
		return nil
	}

	hid, _ := s.store.UpsertHost(ctx, sourceHost, &sourceHost, "linux", true)

	roots := s.cfg.KeyHunt.AllowRoots
	if len(roots) == 0 {
		return nil
	}

	// Find likely key files with a bounded search.
	// We intentionally avoid scanning arbitrary paths outside allow_roots.
	// Output format: one path per line.
	findExpr := ""
	for i, r := range roots {
		if i > 0 {
			findExpr += " "
		}
		findExpr += fmt.Sprintf("%q", r)
	}

	cmd := fmt.Sprintf(`sh -lc '
set -e
roots=%s
# Candidate filenames + basic sanity: regular file, not huge.
find $roots -xdev -maxdepth %d -type f \
  \( -name "id_rsa" -o -name "id_ed25519" -o -name "id_ecdsa" -o -name "identity" -o -name "*.pem" -o -name "id_*" \) \
  -size -2M 2>/dev/null | head -n %d
'`, findExpr, s.cfg.KeyHunt.MaxDepth, s.cfg.KeyHunt.MaxFiles)

	out, err := s.ssh.Run(ctx, sourceHost, cmd)
	if err != nil {
		return err
	}

	scan := bufio.NewScanner(strings.NewReader(out))
	for scan.Scan() {
		path := strings.TrimSpace(scan.Text())
		if path == "" {
			continue
		}

		// Verify it looks like a private key without exfiltrating it.
		// If it is, derive public key via ssh-keygen -y, then compute fingerprint locally via sshd-style tools later.
		// Here we store the public key string returned.
		deriveCmd := fmt.Sprintf(`sh -lc 'set -e; 
if head -n 1 %q | grep -q "BEGIN OPENSSH PRIVATE KEY"; then echo "PRIV"; 
  ssh-keygen -y -f %q 2>/dev/null | sed -e "s/[[:space:]]\+$//"; 
else exit 0; fi'`, path, path)
		derived, derr := s.ssh.Run(ctx, sourceHost, deriveCmd)
		if derr != nil {
			// Still record the path as a potential key file.
			ki := &store.KeyInstance{HostID: hid, Path: path, InstanceType: "private", FirstSeen: time.Now().UTC()}
			_, _ = s.store.UpsertKeyInstance(ctx, ki)
			continue
		}

		lines := strings.Split(strings.TrimSpace(derived), "\n")
		if len(lines) >= 2 && strings.TrimSpace(lines[0]) == "PRIV" {
			pub := strings.TrimSpace(lines[1])
			// We treat the derived output as an authorized key line without comment.
			// Fingerprint mapping is done in keys package.
			fp, fpErr := fingerprintFromPublicKeyLine(pub)
			var keyID *int64
			if fpErr == nil && fp != "" {
				kid, _ := s.store.UpsertSSHKey(ctx, strings.Fields(pub)[0], &pub, fp, nil)
				keyID = &kid
			}

			ki := &store.KeyInstance{HostID: hid, Path: path, KeyID: keyID, InstanceType: "private", FirstSeen: time.Now().UTC()}
			_, _ = s.store.UpsertKeyInstance(ctx, ki)
		}
	}
	return scan.Err()
}

func fingerprintFromPublicKeyLine(publicLine string) (string, error) {
	k, ok := keys.ParseAuthorizedKeysLine(publicLine)
	if !ok {
		return "", fmt.Errorf("parse public key")
	}
	return k.FP256, nil
}
