package agent

import (
	"testing"
)

func BenchmarkCollectRuntimeMetrics(b *testing.B) {
	for b.Loop() {
		_ = collectRuntimeMetrics()
	}
}

func BenchmarkCollectSystemMetrics(b *testing.B) {
	for b.Loop() {
		_ = collectSystemMetrics()
	}
}
