package handler_test

import (
        "bytes"
        "encoding/json"
        "fmt"
        "net/http"
        "net/http/httptest"

        "github.com/go-chi/chi/v5"
        "github.com/eugegm01-dev/metrics/internal/handler"
        "github.com/eugegm01-dev/metrics/internal/model"
        "github.com/eugegm01-dev/metrics/internal/repository"
)

func ExampleUpdateJSONHandler() {
        // Создаём in-memory хранилище
        storage := repository.NewMemStorage()

        // Создаём хэндлер без ключа (без хеша)
        h := handler.UpdateJSONHandler(storage, "")

        // Запускаем тестовый сервер
        ts := httptest.NewServer(h)
        defer ts.Close()

        // Подготавливаем gauge-метрику
        gaugeVal := 42.5
        metric := model.Metrics{
                ID:    "temperature",
                MType: model.Gauge,
                Value: &gaugeVal,
        }
        body, _ := json.Marshal(metric)

        // Отправляем POST-запрос
        resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
        if err != nil {
                fmt.Println("Error:", err)
                return
        }
        defer resp.Body.Close()

        // Декодируем ответ
        var respMetric model.Metrics
        json.NewDecoder(resp.Body).Decode(&respMetric)

        fmt.Printf("ID: %s, Type: %s, Value: %.1f\n", respMetric.ID, respMetric.MType, *respMetric.Value)
        // Output:
        // ID: temperature, Type: gauge, Value: 42.5
}

func ExampleUpdatesHandler() {
        storage := repository.NewMemStorage()
        // Хэндлер без аудита
        h := handler.UpdatesHandler(storage, "", nil)

        ts := httptest.NewServer(h)
        defer ts.Close()

        // Подготавливаем батч метрик
        gaugeVal := 1.23
        counterDelta := int64(5)
        metrics := []model.Metrics{
                {ID: "g1", MType: model.Gauge, Value: &gaugeVal},
                {ID: "c1", MType: model.Counter, Delta: &counterDelta},
        }
        body, _ := json.Marshal(metrics)

        resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
        if err != nil {
                fmt.Println("Error:", err)
                return
        }
        defer resp.Body.Close()

        fmt.Println("Status:", resp.Status)
        // Output:
        // Status: 200 OK
}

func ExampleValueJSONHandler() {
        storage := repository.NewMemStorage()
        // Предзаполняем gauge-метрику
        storage.UpdateGauge("temperature", 42.5)

        h := handler.ValueJSONHandler(storage, "")

        ts := httptest.NewServer(h)
        defer ts.Close()

        // Запрос значения метрики
        metric := model.Metrics{
                ID:    "temperature",
                MType: model.Gauge,
        }
        body, _ := json.Marshal(metric)

        resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
        if err != nil {
                fmt.Println("Error:", err)
                return
        }
        defer resp.Body.Close()

        var respMetric model.Metrics
        json.NewDecoder(resp.Body).Decode(&respMetric)

        fmt.Printf("Value: %.1f\n", *respMetric.Value)
        // Output:
        // Value: 42.5
}

func ExampleGetMetricHandler() {
        storage := repository.NewMemStorage()
        storage.UpdateCounter("visits", 100)

        // GetMetricHandler требует chi-роутера для извлечения параметров
        r := chi.NewRouter()
        r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage, ""))

        ts := httptest.NewServer(r)
        defer ts.Close()

        resp, err := http.Get(ts.URL + "/value/counter/visits")
        if err != nil {
                fmt.Println("Error:", err)
                return
        }
        defer resp.Body.Close()

        fmt.Println("Status:", resp.Status)
        // Output:
        // Status: 200 OK
}

func ExampleIndexHTMLHandler() {
        storage := repository.NewMemStorage()
        storage.UpdateGauge("test", 3.14)

        h := handler.IndexHTMLHandler(storage, "")

        ts := httptest.NewServer(h)
        defer ts.Close()

        resp, err := http.Get(ts.URL)
        if err != nil {
                fmt.Println("Error:", err)
                return
        }
        defer resp.Body.Close()

        fmt.Println("Status:", resp.Status)
        // Output:
        // Status: 200 OK
}