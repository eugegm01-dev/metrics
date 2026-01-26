package agent

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
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
		// Проверяем отмену перед попыткой
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			// безопасное ожидание с учётом ctx
			var delay time.Duration
			if attempt-1 < len(delays) {
				delay = delays[attempt-1]
			} else if len(delays) > 0 {
				delay = delays[len(delays)-1] // используем последний delay как fallback
			} else {
				delay = time.Second
			}

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-timer.C:
				// продолжить к выполнению операции
			}
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		classifier := NewRetryableErrorClassifier()
		if !classifier.IsRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt < maxRetries {
			zap.L().Sugar().Debugf("Retryable error occurred (attempt %d/%d): %v. Retrying in %v",
				attempt+1, maxRetries, err, func() time.Duration {
					if attempt < len(delays) {
						return delays[attempt]
					}
					if len(delays) > 0 {
						return delays[len(delays)-1]
					}
					return time.Second
				}(),
			)
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}
