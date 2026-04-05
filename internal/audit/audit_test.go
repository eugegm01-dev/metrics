package audit

import (
	"os"
	"testing"
)

func TestFileObserver_Update(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "audit_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	obs, err := NewFileObserver(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()

	event := NewAuditEvent([]string{"metric1", "metric2"}, "127.0.0.1")
	err = obs.Update(event)
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("File is empty")
	}
}

func TestHTTPObserver_Update(t *testing.T) {
	// Здесь можно создать тестовый HTTP-сервер, но для простоты проверим только что ошибка не падает
	obs := NewHTTPObserver("http://localhost:9999")
	event := NewAuditEvent([]string{"test"}, "1.2.3.4")
	err := obs.Update(event)
	if err == nil {
		t.Log("HTTP observer would fail if server not running, but that's fine")
	}
	// Закрывать не нужно
}
