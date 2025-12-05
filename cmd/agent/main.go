package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"time"
)

func main() {
	pollCount := int64(0)
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))

	pollTicker := time.NewTicker(2 * time.Second)
	reportTicker := time.NewTicker(10 * time.Second)

	fmt.Println("Starting metrics agent (Iteration 2)...")

	for {
		select {
		case <-pollTicker.C:
			pollCount++
			fmt.Printf("[%s] Poll #%d - collecting metrics\n",
				time.Now().Format("15:04:05"), pollCount)

		case <-reportTicker.C:
			fmt.Printf("[%s] Sending metrics to server...\n",
				time.Now().Format("15:04:05"))

			// Собираем метрики
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)

			// Отправляем несколько ключевых метрик
			successCount := 0

			// Gauge метрики (float)
			gaugeMetrics := []struct {
				name  string
				value float64
			}{
				{"Alloc", float64(mem.Alloc)},
				{"HeapAlloc", float64(mem.HeapAlloc)},
				{"RandomValue", randSource.Float64() * 100},
			}

			for _, m := range gaugeMetrics {
				url := fmt.Sprintf("http://localhost:8080/update/gauge/%s/%f",
					m.name, m.value)

				req, err := http.NewRequest("POST", url, nil)
				if err != nil {
					fmt.Printf("  ERROR creating request for %s: %v\n", m.name, err)
					continue
				}
				req.Header.Set("Content-Type", "text/plain")

				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					fmt.Printf("  ERROR sending %s: %v\n", m.name, err)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					successCount++
					fmt.Printf("  ✓ Sent gauge %s: %f\n", m.name, m.value)
				} else {
					fmt.Printf("  ✗ Server error for %s: %d\n", m.name, resp.StatusCode)
				}
			}

			// Counter метрика (int) - ОСОБЕННО ВАЖНО!
			url := fmt.Sprintf("http://localhost:8080/update/counter/PollCount/%d", pollCount)
			req, err := http.NewRequest("POST", url, nil)
			if err != nil {
				fmt.Printf("  ERROR creating request for PollCount: %v\n", err)
			} else {
				req.Header.Set("Content-Type", "text/plain")
				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					fmt.Printf("  ERROR sending PollCount: %v\n", err)
				} else {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						successCount++
						fmt.Printf("  ✓ Sent counter PollCount: %d\n", pollCount)
					} else {
						fmt.Printf("  ✗ Server error for PollCount: %d\n", resp.StatusCode)
					}
				}
			}

			fmt.Printf("Successfully sent %d/4 metrics\n\n", successCount)
		}
	}
}
