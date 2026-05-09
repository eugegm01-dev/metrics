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

// Sender отвечает за HTTP-отправку метрик на сервер.
type Sender struct {
	serverAddr string
	key        string
	publicKey  *rsa.PublicKey
	localIP    string
	client     *retryablehttp.Client
}

var gzipBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

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

// NewSender создает новый экземпляр Sender.
func NewSender(serverAddr, key string, publicKey *rsa.PublicKey) *Sender {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 5 * time.Second
	client.HTTPClient = &http.Client{Timeout: 10 * time.Second}

	return &Sender{
		serverAddr: serverAddr,
		key:        key,
		publicKey:  publicKey,
		localIP:    getLocalIP(),
		client:     client,
	}
}

func computeHash(data []byte, key string) string {
	if key == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// SendBatch отправляет пакет метрик одним POST-запросом на /updates.
func (s *Sender) SendBatch(ctx context.Context, metrics []models.Metrics) error {
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

	fullURL, err := url.JoinPath("http://"+s.serverAddr, "/updates")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	var finalBody []byte
	var isEncrypted bool
	if s.publicKey != nil {
		encData, err := crypto.Encrypt(gzData, s.publicKey)
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

	if s.localIP != "" {
		req.Header.Set("X-Real-IP", s.localIP)
	}

	if isEncrypted {
		req.Header.Set("X-Crypto-Encrypted", "true")
		req.Header.Set("Content-Type", "application/octet-stream")
	} else {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Set("Accept-Encoding", "gzip")

	hash := computeHash(jsonData, s.key)
	if hash != "" {
		req.Header.Set("HashSHA256", hash)
	}

	req = req.WithContext(ctx)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request after retries: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	return nil
}

// getLocalIP возвращает первый не‑loopback IPv4 адрес.
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}
