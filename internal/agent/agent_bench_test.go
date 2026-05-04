package agent

import (
	"testing"
)

func BenchmarkCollectRuntimeMetrics(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = collectRuntimeMetrics()
	}
}

func BenchmarkCollectSystemMetrics(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = collectSystemMetrics()
	}
}
