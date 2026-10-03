package ratchet

import (
	"context"
	"os/exec"
	"time"
)

// git runs a git command in dir and returns stdout.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	argv := append([]string{"-C", dir}, args...)
	out, err := exec.CommandContext(c, "git", argv...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
