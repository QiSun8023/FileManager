package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// A stalled system-service command must not indefinitely stop directory monitoring.
func runHelperWake(timeout time.Duration, program string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("提醒服务唤起超时：%w", ctx.Err())
	}
	text := strings.TrimSpace(string(out))
	lower := strings.ToLower(text)
	if err != nil || strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "failed") {
		return fmt.Errorf("提醒服务唤起失败：%v %s", err, text)
	}
	return nil
}
