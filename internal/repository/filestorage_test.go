package repository

import (
	"os"
	"testing"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

func TestFileStorage_LoadSave(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "metrics_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpName := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpName)

	fs, err := NewFileStorage(tmpName, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	_ = fs.UpdateGauge("test", 123.45)
	_ = fs.UpdateCounter("test_counter", 42)
	if err := fs.SaveToFile(); err != nil {
		t.Fatal(err)
	}

	fs2, err := NewFileStorage(tmpName, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fs2.Close()

	val, ok, err := fs2.GetGauge("test")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 123.45 {
		t.Errorf("GetGauge after load = %v, %v; want 123.45, true", val, ok)
	}
	valC, okC, err := fs2.GetCounter("test_counter")
	if err != nil {
		t.Fatal(err)
	}
	if !okC || valC != 42 {
		t.Errorf("GetCounter after load = %v, %v; want 42, true", valC, okC)
	}
}

func TestFileStorage_UpdateBatch(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "metrics_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpName := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpName)

	fs, err := NewFileStorage(tmpName, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()

	gVal := 1.23
	cVal := int64(10)
	metrics := []models.Metrics{
		{ID: "g", MType: models.Gauge, Value: &gVal},
		{ID: "c", MType: models.Counter, Delta: &cVal},
	}
	if err := fs.UpdateBatch(metrics); err != nil {
		t.Fatal(err)
	}
	val, ok, err := fs.GetGauge("g")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 1.23 {
		t.Errorf("GetGauge = %v, %v; want 1.23, true", val, ok)
	}
	valC, okC, err := fs.GetCounter("c")
	if err != nil {
		t.Fatal(err)
	}
	if !okC || valC != 10 {
		t.Errorf("GetCounter = %v, %v; want 10, true", valC, okC)
	}
}

func TestFileStorage_PeriodicSave(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "metrics_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpName := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpName)

	fs, err := NewFileStorage(tmpName, 100*time.Millisecond, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = fs.UpdateGauge("test", 99.99)

	time.Sleep(200 * time.Millisecond)

	if err := fs.SaveToFile(); err != nil {
		t.Fatal(err)
	}
	fs.Close()

	time.Sleep(50 * time.Millisecond)

	fs2, err := NewFileStorage(tmpName, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	defer fs2.Close()

	val, ok, err := fs2.GetGauge("test")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 99.99 {
		t.Errorf("Periodic save failed: got %v, %v", val, ok)
	}
}
