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

func (s *Spider) scanAuthorizedKeysAndPersist(ctx context.Context, hostID int64, host string) (int, error) {
	// Pull authorized_keys and persist as:
	// - ssh_keys (fingerprint)
	// - key_instances (authorized_key)
	cmd := `sh -lc '
set -e
for f in /root/.ssh/authorized_keys /home/*/.ssh/authorized_keys; do
  [ -r "$f" ] || continue
  echo "---FILE $f"
  cat "$f"
  echo
done
'`
	out, err := s.ssh.Run(ctx, host, cmd)
	if err != nil {
		return 0, err
	}

	var currentPath string
	count := 0
	scan := bufio.NewScanner(strings.NewReader(out))
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "---FILE ") {
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "---FILE "))
			continue
		}
		k, ok := keys.ParseAuthorizedKeysLine(line)
		if !ok {
			continue
		}
		count++

		pub := k.Authorized
		comment := k.Comment
		kid, err := s.store.UpsertSSHKey(ctx, k.Type, &pub, k.FP256, ptr(comment))
		if err != nil {
			return count, err
		}
		ki := &store.KeyInstance{
			HostID:       hostID,
			Path:         currentPath,
			KeyID:        &kid,
			InstanceType: "authorized_key",
			FirstSeen:    time.Now().UTC(),
		}
		if _, err := s.store.UpsertKeyInstance(ctx, ki); err != nil {
			return count, fmt.Errorf("upsert key_instance: %w", err)
		}
	}
	if err := scan.Err(); err != nil {
		return count, err
	}
	return count, nil
}
