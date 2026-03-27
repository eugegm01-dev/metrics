package model

const (
	// Counter тип для метрик-счётчиков.

	Counter = "counter"
	// Gauge тип для метрик-значений.

	Gauge = "gauge"
)

// Metrics представляет метрику в системе.
// ID – имя метрики, MType – тип ("gauge" или "counter").
// Delta используется для счётчиков, Value – для gauges.
// Hash опционален для проверки целостности.

type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}
