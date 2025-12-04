package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eugegm01-dev/metrics/internal/repository"
)

func TestUpdateHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	handler := UpdateHandler(storage)

	// Тест 1: Успешный counter
	req := httptest.NewRequest("POST", "/update/counter/test/527", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Проверяем, что counter сохранился
	val, ok := storage.GetCounter("test")
	if !ok || val != 527 {
		t.Errorf("Counter not saved correctly")
	}
}
