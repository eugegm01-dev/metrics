package agent

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestRetryableErrorClassifier_IsRetryableError(t *testing.T) {
	c := NewRetryableErrorClassifier()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"timeout", &net.DNSError{IsTimeout: true}, true},
		{"connection refused", errors.New("connection refused"), true},
		{"EOF", errors.New("EOF"), true},
		{"other", errors.New("some error"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetry(t *testing.T) {
	ctx := context.Background()
	// успешная операция
	err := Retry(ctx, func() error { return nil }, 1, time.Millisecond)
	if err != nil {
		t.Errorf("Retry failed: %v", err)
	}
	// операция с ошибкой, которая не повторяется
	err = Retry(ctx, func() error { return errors.New("permanent") }, 1, time.Millisecond)
	if err == nil {
		t.Error("expected error, got nil")
	}
	// операция, которая повторяется
	var attempts int
	err = Retry(ctx, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("connection refused")
		}
		return nil
	}, 2, time.Millisecond, time.Millisecond)
	if err != nil {
		t.Errorf("Retry failed: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}
