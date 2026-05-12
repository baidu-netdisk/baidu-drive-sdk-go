package scene

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetry_Success_FirstAttempt(t *testing.T) {
	calls := 0
	err := retry(context.Background(), 3, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetry_Success_SecondAttempt(t *testing.T) {
	calls := 0
	err := retry(context.Background(), 3, func() error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestRetry_AllFail(t *testing.T) {
	calls := 0
	err := retry(context.Background(), 3, func() error {
		calls++
		return errors.New("persistent error")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
	if err.Error() != "persistent error" {
		t.Errorf("error = %q, want 'persistent error'", err.Error())
	}
}

func TestRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := retry(ctx, 3, func() error {
		return errors.New("should not retry")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	start := time.Now()
	calls := 0
	_ = retry(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return errors.New("fail")
		}
		return nil
	})
	elapsed := time.Since(start)
	// 1s + 2s = 3s minimum; allow some slack
	if elapsed < 2*time.Second {
		t.Errorf("elapsed = %v, expected >= 2s (exponential backoff 1s+2s)", elapsed)
	}
}
