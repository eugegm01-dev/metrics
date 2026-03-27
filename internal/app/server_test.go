package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"go.uber.org/zap"
)

func TestInitRouter(t *testing.T) {
	storage := repository.NewMemStorage()
	logger := zap.NewNop()
	r := initRouter(storage, logger, "", nil)
	if r == nil {
		t.Fatal("router is nil")
	}
	// проверяем, что маршрут /ping зарегистрирован и работает
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestInitStorage(t *testing.T) {
	cfg := &config.ServerConfig{
		FileStoragePath: "",
		StoreInterval:   0,
		Restore:         false,
	}
	logger := zap.NewNop()
	storage, err := initStorage(cfg, nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	// проверяем, что хранилище не nil
	if storage == nil {
		t.Error("storage is nil")
	}
}
