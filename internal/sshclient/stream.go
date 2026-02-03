package sshclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Stream runs an SSH command and yields stdout lines to handler.
// If handler returns false, the stream stops.
func (c *Client) Stream(ctx context.Context, host string, remoteCmd string, handler func(line string) bool) error {
	userHost := host
	if c.cfg.SSH.User != "" && !strings.Contains(host, "@") {
		userHost = c.cfg.SSH.User + "@" + host
	}

	args := []string{
		"-o", "BatchMode=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%d", c.cfg.SSH.ConnectTimeoutSeconds),
		userHost,
		"--",
		remoteCmd,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ssh start %s: %w: %s", userHost, err, strings.TrimSpace(stderr.String()))
	}

	s := bufio.NewScanner(stdout)
	for s.Scan() {
		if handler != nil {
			if ok := handler(s.Text()); !ok {
				_ = cmd.Process.Kill()
				break
			}
		}
	}

	_ = cmd.Wait()
	if err := s.Err(); err != nil {
		return err
	}
	return nil
}
