// internal/agent/sender.go
package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	crypto "github.com/eugegm01-dev/metrics/internal/crypto"
	models "github.com/eugegm01-dev/metrics/internal/model"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// retryClient - клиент с автоматическими повторными попытками
var retryClient *retryablehttp.Client

var gzipBufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}
var publicKey *rsa.PublicKey

func gzipData(data []byte) ([]byte, error) {
	buf := gzipBufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		gzipBufPool.Put(buf)
	}()
	gw := gzip.NewWriter(buf)
	if _, err := gw.Write(data); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	localIP     string
	localIPOnce sync.Once
)

func init() {
	retryClient = retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.HTTPClient = &http.Client{
		Timeout: 10 * time.Second,
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

// SendMetricsBatch отправляет несколько метрик одним POST-запросом на эндпоинт /updates.
func SendMetricsBatch(serverAddr string, metrics []models.Metrics, key string) error {
	if len(metrics) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

	fullURL, err := url.JoinPath("http://"+serverAddr, "/updates")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	var finalBody []byte
	var isEncrypted bool

	if publicKey != nil {
		encData, err := crypto.Encrypt(gzData, publicKey)
		if err != nil {
			return fmt.Errorf("encrypt payload: %w", err)
		}
		finalBody = encData
		isEncrypted = true
	} else {
		finalBody = gzData
	}

	req, err := retryablehttp.NewRequest("POST", fullURL, bytes.NewBuffer(finalBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if isEncrypted {
		req.Header.Set("X-Crypto-Encrypted", "true")
		req.Header.Set("Content-Type", "application/octet-stream")
	} else {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Set("Accept-Encoding", "gzip")

	hash := computeHash(jsonData, key)
	if hash != "" {
		req.Header.Set("HashSHA256", hash)
	}
	if ip := getLocalIP(); ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request after retries: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	return nil

}

// SendMetrics отправляет каждую метрику по отдельности через эндпоинт /update.
func SendMetrics(serverAddr string, metrics []models.Metrics, key string) error {
	for _, metric := range metrics {

		jsonData, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("marshal metric: %w", err)
		}

		gzData, err := gzipData(jsonData)
		if err != nil {
			return fmt.Errorf("gzip data: %w", err)
		}

		var finalBody []byte
		isEncrypted := false
		if publicKey != nil {
			encData, err := crypto.Encrypt(gzData, publicKey)
			if err != nil {
				return fmt.Errorf("encrypt payload: %w", err)
			}
			finalBody = encData
			isEncrypted = true
		} else {
			finalBody = gzData
		}

		fullURL, err := url.JoinPath("http://"+serverAddr, "/update")
		if err != nil {
			return fmt.Errorf("build URL: %w", err)
		}

		req, err := retryablehttp.NewRequest("POST", fullURL, bytes.NewBuffer(finalBody))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		if ip := getLocalIP(); ip != "" {
			req.Header.Set("X-Real-IP", ip)
		}

		if isEncrypted {
			req.Header.Set("X-Crypto-Encrypted", "true")
			req.Header.Set("Content-Type", "application/octet-stream")
		} else {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")
		}
		req.Header.Set("Accept-Encoding", "gzip")

		hash := computeHash(jsonData, key)
		if hash != "" {
			req.Header.Set("HashSHA256", hash)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		resp, err := retryClient.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status %d for metric %s", resp.StatusCode, metric.ID)
		}
	}
	return nil
}

// InitAgentCrypto загружает публичный ключ один раз.
func InitAgentCrypto(path string) error {
	if path == "" {
		return nil
	}
	var err error
	publicKey, err = crypto.LoadPublicKey(path)
	return err
}
func SendMetricsBatchWithContext(ctx context.Context, serverAddr string, metrics []models.Metrics, key string) error {
	if len(metrics) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

	fullURL, err := url.JoinPath("http://"+serverAddr, "/updates")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	var finalBody []byte
	var isEncrypted bool
	if publicKey != nil {
		encData, err := crypto.Encrypt(gzData, publicKey)
		if err != nil {
			return fmt.Errorf("encrypt payload: %w", err)
		}
		finalBody = encData
		isEncrypted = true
	} else {
		finalBody = gzData
	}

	req, err := retryablehttp.NewRequest("POST", fullURL, bytes.NewBuffer(finalBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add this block
	if ip := getLocalIP(); ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}

	if isEncrypted {
		req.Header.Set("X-Crypto-Encrypted", "true")
		req.Header.Set("Content-Type", "application/octet-stream")
	} else {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Set("Accept-Encoding", "gzip")

	hash := computeHash(jsonData, key)
	if hash != "" {
		req.Header.Set("HashSHA256", hash)
	}

	// Use provided context
	req = req.WithContext(ctx)

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request after retries: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	if ip := getLocalIP(); ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}

	return nil
}
func getLocalIP() string {
	localIPOnce.Do(func() {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				localIP = ipnet.IP.String()
				break
			}
		}
	})
	return localIP
}
