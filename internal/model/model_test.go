package model

import "testing"

func TestConstants(t *testing.T) {
    if Counter != "counter" {
        t.Errorf("Counter constant = %s, want 'counter'", Counter)
    }
    if Gauge != "gauge" {
        t.Errorf("Gauge constant = %s, want 'gauge'", Gauge)
    }
}
