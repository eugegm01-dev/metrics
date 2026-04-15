// internal/middleware/crypto.go
package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/eugegm01-dev/metrics/internal/crypto"
)

// CryptoMiddleware расшифровывает тело запроса, если присутствует заголовок X-Crypto-Encrypted.
func CryptoMiddleware(privKey *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if privKey == nil || r.Header.Get("X-Crypto-Encrypted") != "true" {
				next.ServeHTTP(w, r)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			defer func() { _ = r.Body.Close() }()

			decrypted, err := crypto.Decrypt(bodyBytes, privKey)
			if err != nil {
				http.Error(w, "decryption failed", http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(decrypted))
			r.ContentLength = int64(len(decrypted))
			r.Header.Del("X-Crypto-Encrypted")
			r.Header.Set("Content-Encoding", "gzip")

			next.ServeHTTP(w, r)
		})
	}
}
