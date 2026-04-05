package agent

import (
	"context"
	"fmt"
	"net"
	"time"
)

// RetryableErrorClassifier определяет, является ли ошибка повторяемой (временной).
type RetryableErrorClassifier struct{}

// NewRetryableErrorClassifier создаёт новый классификатор.
func NewRetryableErrorClassifier() *RetryableErrorClassifier {
	return &RetryableErrorClassifier{}
}

// IsRetryableError возвращает true, если ошибка считается повторяемой (таймауты, отказ соединения, EOF).
func (c *RetryableErrorClassifier) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем сетевые ошибки
	if netErr, ok := err.(net.Error); ok {
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

// Retry выполняет операцию с повторами до maxRetries раз, используя заданные задержки между попытками.
// Учитывает контекст для отмены. Если операция вернула неповторяемую ошибку, повтор останавливается.
func Retry(ctx context.Context, operation func() error, maxRetries int, delays ...time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Проверяем контекст перед каждой попыткой
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}

		// Выполняем операцию
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		classifier := NewRetryableErrorClassifier()

		// Проверяем, нужно ли повторять
		if !classifier.IsRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Если это последняя попытка, выходим
		if attempt == maxRetries {
			break
		}

		// Используем Timer с поддержкой контекста
		delay := delays[attempt]
		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return fmt.Errorf("operation cancelled while waiting for retry: %w", ctx.Err())
		case <-timer.C:
			// Продолжаем со следующей попыткой
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}
