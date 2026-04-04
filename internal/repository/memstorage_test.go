package repository

import (
	"strings"
	"testing"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

func TestMemStorage_UpdateGauge(t *testing.T) {
	s := NewMemStorage()
	_ = s.UpdateGauge("test", 42.5)
	val, ok, err := s.GetGauge("test")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 42.5 {
		t.Errorf("GetGauge() = %v, %v; want 42.5, true", val, ok)
	}
}

func TestMemStorage_UpdateCounter(t *testing.T) {
	s := NewMemStorage()
	_ = s.UpdateCounter("test", 10)
	_ = s.UpdateCounter("test", 5)
	val, ok, err := s.GetCounter("test")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 15 {
		t.Errorf("GetCounter() = %v, %v; want 15, true", val, ok)
	}
}

func TestMemStorage_GetGauge_NotFound(t *testing.T) {
	s := NewMemStorage()
	val, ok, err := s.GetGauge("missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok || val != 0 {
		t.Errorf("GetGauge() = %v, %v; want 0, false", val, ok)
	}
}

func TestMemStorage_GetCounter_NotFound(t *testing.T) {
	s := NewMemStorage()
	val, ok, err := s.GetCounter("missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok || val != 0 {
		t.Errorf("GetCounter() = %v, %v; want 0, false", val, ok)
	}
}

func TestMemStorage_GetAllMetrics(t *testing.T) {
	s := NewMemStorage()
	_ = s.UpdateGauge("g1", 1.1)
	_ = s.UpdateGauge("g2", 2.2)
	_ = s.UpdateCounter("c1", 10)
	output, err := s.GetAllMetrics()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "g1: 1.100000") ||
		!strings.Contains(output, "g2: 2.200000") ||
		!strings.Contains(output, "c1: 10") {
		t.Errorf("GetAllMetrics() output missing expected entries: %s", output)
	}
}

func TestMemStorage_UpdateBatch(t *testing.T) {
	s := NewMemStorage()
	gVal := 3.14
	cVal := int64(100)
	metrics := []models.Metrics{
		{ID: "g1", MType: models.Gauge, Value: &gVal},
		{ID: "c1", MType: models.Counter, Delta: &cVal},
	}
	err := s.UpdateBatch(metrics)
	if err != nil {
		t.Fatalf("UpdateBatch failed: %v", err)
	}
	val, ok, err := s.GetGauge("g1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 3.14 {
		t.Errorf("GetGauge('g1') = %v, %v; want 3.14, true", val, ok)
	}
	valC, okC, err := s.GetCounter("c1")
	if err != nil {
		t.Fatal(err)
	}
	if !okC || valC != 100 {
		t.Errorf("GetCounter('c1') = %v, %v; want 100, true", valC, okC)
	}
}

func TestMemStorage_UpdateBatch_Invalid(t *testing.T) {
	s := NewMemStorage()
	metrics := []models.Metrics{
		{ID: "bad", MType: models.Gauge, Value: nil},
	}
	err := s.UpdateBatch(metrics)
	if err == nil {
		t.Error("UpdateBatch with nil Value should return error")
	}
}
