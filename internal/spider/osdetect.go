package spider

import (
	"context"
	"strings"
)

func (s *Spider) detectOSType(ctx context.Context, host string) string {
	out, err := s.ssh.Run(ctx, host, "uname -s")
	if err != nil {
		return "linux"
	}
	o := strings.TrimSpace(out)
	switch {
	case strings.EqualFold(o, "AIX"):
		return "aix"
	case strings.EqualFold(o, "Linux"):
		return "linux"
	default:
		return strings.ToLower(o)
	}
}
