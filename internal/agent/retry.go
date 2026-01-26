package agent

import (
	"context"
	"fmt"
	"net"
	"time"
)

// RetryableErrorClassifier классифицирует ошибки как retryable/non-retryable
type RetryableErrorClassifier struct{}

func NewRetryableErrorClassifier() *RetryableErrorClassifier {
	return &RetryableErrorClassifier{}
}

// IsRetryableError проверяет, является ли ошибка retryable
func (c *RetryableErrorClassifier) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем сетевые ошибки
	if netErr, ok := err.(net.Error); ok {
		// Метод Temporary() устарел в Go 1.23 и удален в Go 1.24
		// Используем только Timeout() проверку
		if netErr.Timeout() {
			return true
		}
	}

	// Проверяем ошибки HTTP
	if err.Error() == "EOF" ||
		err.Error() == "connection refused" ||
		err.Error() == "connection reset by peer" ||
		err.Error() == "dial tcp: connection refused" ||
		err.Error() == "read: connection reset by peer" {
		return true
	}

	return false
}

// Retry выполняет операцию с повторными попытками
func Retry(ctx context.Context, operation func() error, maxRetries int, delays ...time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Проверяем, нужно ли повторять
		classifier := NewRetryableErrorClassifier()
		if !classifier.IsRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Для HTTP ошибок проверяем статус код
		if attempt < maxRetries {
			fmt.Printf("Retryable error occurred (attempt %d/%d): %v. Retrying in %v...\n",
				attempt+1, maxRetries, err, delays[attempt])
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}
