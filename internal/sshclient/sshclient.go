package sshclient

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jsherman999/openclaw_keyspider/internal/config"
)

type Client struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Client { return &Client{cfg: cfg} }

func (c *Client) CanConnect(ctx context.Context, host string) bool {
	// Lightweight connectivity check.
	ctx2, cancel := context.WithTimeout(ctx, c.cfg.SSH.ConnectTimeout)
	defer cancel()
	_, err := c.Run(ctx2, host, "true")
	return err == nil
}

func (c *Client) Run(ctx context.Context, host string, remoteCmd string) (string, error) {
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("ssh %s: %w: %s", userHost, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
