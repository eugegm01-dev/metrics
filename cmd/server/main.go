package main

import (
    "fmt"
    "log"
    "net/http"
    "strconv"
    "strings"
    "sync"
)

// MemStorage - хранилище метрик в памяти
type MemStorage struct {
    mu       sync.RWMutex
    gauges   map[string]float64
    counters map[string]int64
}

// NewMemStorage создает новое хранилище
func NewMemStorage() *MemStorage {
    return &MemStorage{
        gauges:   make(map[string]float64),
        counters: make(map[string]int64),
    }
}

// UpdateGauge обновляет gauge метрику
func (s *MemStorage) UpdateGauge(name string, value float64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.gauges[name] = value
}

// UpdateCounter обновляет counter метрику
func (s *MemStorage) UpdateCounter(name string, value int64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.counters[name] += value
}

// GetAllMetrics возвращает все метрики (для отладки)
func (s *MemStorage) GetAllMetrics() string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var result string
    result += "Gauges:\n"
    for k, v := range s.gauges {
        result += fmt.Sprintf("  %s: %f\n", k, v)
    }
    result += "Counters:\n"
    for k, v := range s.counters {
        result += fmt.Sprintf("  %s: %d\n", k, v)
    }
    return result
}

// isValidType проверяет корректность типа метрики
func isValidType(metricType string) bool {
    return metricType == "gauge" || metricType == "counter"
}

// updateHandler обрабатывает запрос на обновление метрики
func updateHandler(storage *MemStorage) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Проверяем метод
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        // Разбираем URL: /update/<ТИП>/<ИМЯ>/<ЗНАЧЕНИЕ>
        parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
        
        // Проверяем формат: должно быть 4 части ["update", "type", "name", "value"]
        if len(parts) != 4 {
            http.Error(w, "Invalid request format", http.StatusNotFound)
            return
        }

        // Проверяем первый элемент пути
        if parts[0] != "update" {
            http.Error(w, "Invalid endpoint", http.StatusNotFound)
            return
        }

        metricType := parts[1]
        metricName := parts[2]
        metricValue := parts[3]

        // Проверяем наличие имени метрики
        if metricName == "" {
            http.Error(w, "Metric name is required", http.StatusNotFound)
            return
        }

        // Проверяем тип метрики
        if !isValidType(metricType) {
            http.Error(w, "Invalid metric type", http.StatusBadRequest)
            return
        }

        // Обрабатываем в зависимости от типа
        switch metricType {
        case "gauge":
            value, err := strconv.ParseFloat(metricValue, 64)
            if err != nil {
                http.Error(w, "Invalid gauge value", http.StatusBadRequest)
                return
            }
            storage.UpdateGauge(metricName, value)

        case "counter":
            value, err := strconv.ParseInt(metricValue, 10, 64)
            if err != nil {
                http.Error(w, "Invalid counter value", http.StatusBadRequest)
                return
            }
            storage.UpdateCounter(metricName, value)
        }

        // Успешный ответ
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Metric updated successfully"))
    }
}

// rootHandler для корневого пути (опционально)
func rootHandler(storage *MemStorage) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" {
            http.NotFound(w, r)
            return
        }
        
        info := storage.GetAllMetrics()
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "Metrics Server\n\n%s", info)
    }
}

func main() {
    // Инициализируем хранилище
    storage := NewMemStorage()
    
    // Настраиваем маршруты
    http.HandleFunc("/update/", updateHandler(storage))
    http.HandleFunc("/", rootHandler(storage))

    // Запускаем сервер
    serverAddr := ":8080"
    fmt.Printf("Starting server on %s\n", serverAddr)
    fmt.Println("Available endpoints:")
    fmt.Println("  POST /update/<type>/<name>/<value>")
    fmt.Println("  GET  / (для просмотра метрик)")
    
    if err := http.ListenAndServe(serverAddr, nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
