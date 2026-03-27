// internal/middleware/gzip.go
package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// GzipMiddleware сжимает HTTP-ответы, если клиент поддерживает gzip.
// Также распаковывает gzip-сжатые тела запросов при наличии заголовка Content-Encoding: gzip.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверка поддержки gzip в Accept-Encoding
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Распаковка тела, если есть Content-Encoding: gzip
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip body", http.StatusBadRequest)
				return
			}
			r.Body = gzReader
			defer gzReader.Close()
		}

		// Обертка для ответа с сжатием
		gzWriter := gzip.NewWriter(w)
		defer gzWriter.Close()

		// Установка заголовка Content-Encoding
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		gzResponse := &gzipResponseWriter{ResponseWriter: w, Writer: gzWriter}
		next.ServeHTTP(gzResponse, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
