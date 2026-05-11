package scene

import (
	"context"
	"time"
)

// retry 执行 fn 最多 maxAttempts 次，间隔使用 exponential backoff（1s, 2s, 4s...）。
// 如果 context 被取消，立即返回。
func retry(ctx context.Context, maxAttempts int, fn func() error) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if i < maxAttempts-1 {
			delay := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return lastErr
}
