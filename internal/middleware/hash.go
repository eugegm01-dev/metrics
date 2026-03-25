package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// HashMiddleware добавляет заголовок хеша к ответам и проверяет хеши входящих запросов.
// Использует заданный ключ для вычисления HMAC-SHA256 тела запроса/ответа.
// Если ключ пустой, middleware ничего не делает.
func HashMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Если ключ не задан, пропускаем проверку
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Для запросов с телом проверяем хеш
			if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
				// Сохраняем оригинальное тело
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}

				// Восстанавливаем тело для дальнейшего использования
				r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

				// Проверяем хеш, если он есть в заголовке
				receivedHash := r.Header.Get("HashSHA256")
				if receivedHash != "" {
					// Вычисляем ожидаемый хеш
					expectedHash := computeHash(bodyBytes, key)

					if receivedHash != expectedHash {
						http.Error(w, "Invalid hash", http.StatusBadRequest)
						return
					}
				}
			}

			// Обертка для ResponseWriter, чтобы перехватывать ответ
			hw := &hashResponseWriter{
				ResponseWriter: w,
				key:            key,
			}

			next.ServeHTTP(hw, r)
		})
	}
}

// computeHash вычисляет HMAC-SHA256 хеш от данных с ключом
func computeHash(data []byte, key string) string {
	if key == "" {
		return ""
	}

	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// hashResponseWriter перехватывает запись ответа для добавления хеша
type hashResponseWriter struct {
	http.ResponseWriter
	key    string
	body   []byte
	status int
}

func (hw *hashResponseWriter) WriteHeader(status int) {
	hw.status = status
	hw.ResponseWriter.WriteHeader(status)
}

func (hw *hashResponseWriter) Write(b []byte) (int, error) {
	hw.body = append(hw.body, b...)
	return hw.ResponseWriter.Write(b)
}

func (hw *hashResponseWriter) Flush() {
	if f, ok := hw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (hw *hashResponseWriter) Close() error {
	// Добавляем хеш в заголовок, если есть ключ и тело не пустое
	if hw.key != "" && len(hw.body) > 0 {
		hash := computeHash(hw.body, hw.key)
		hw.ResponseWriter.Header().Set("HashSHA256", hash)
	}

	// Записываем заголовки и тело
	hw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(hw.body)))
	hw.ResponseWriter.Write(hw.body)

	return nil
}
