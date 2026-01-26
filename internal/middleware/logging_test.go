package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestLoggingMiddleware(t *testing.T) {
	// Создаем тестовый логгер
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Создаем тестовый обработчик
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Оборачиваем в middleware
	wrappedHandler := LoggingMiddleware(logger)(handler)

	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Выполняем запрос
	wrappedHandler.ServeHTTP(rr, req)

	// Проверяем статус
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Проверяем тело ответа
	expected := "test response"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}
