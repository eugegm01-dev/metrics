package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Структура для хранения данных ответа
type responseData struct {
	status int
	size   int
}

// Наш кастомный ResponseWriter для перехвата данных
type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

// Перехватываем запись тела ответа, чтобы измерить его размер
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

// Перехватываем установку статус-кода
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

// Middleware для логирования запросов и ответов
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}
			lw := &loggingResponseWriter{
				ResponseWriter: w,
				responseData:   responseData,
			}

			next.ServeHTTP(lw, r)

			duration := time.Since(start)

			logger.Info("request",
				zap.String("uri", r.RequestURI),
				zap.String("method", r.Method),
				zap.Duration("duration", duration),
				zap.Int("status", responseData.status),
				zap.Int("size", responseData.size),
			)
		})
	}
}
