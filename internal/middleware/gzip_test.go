package middleware

import (
    "bytes"
    "compress/gzip"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestGzipMiddleware(t *testing.T) {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("test response"))
    })
    mw := GzipMiddleware(handler)

    req := httptest.NewRequest("GET", "/", nil)
    req.Header.Set("Accept-Encoding", "gzip")
    rr := httptest.NewRecorder()
    mw.ServeHTTP(rr, req)

    if rr.Header().Get("Content-Encoding") != "gzip" {
        t.Error("Content-Encoding not set to gzip")
    }
    // Проверим, что тело сжато
    gr, err := gzip.NewReader(rr.Body)
    if err != nil {
        t.Fatal(err)
    }
    defer gr.Close()
    buf := new(bytes.Buffer)
    buf.ReadFrom(gr)
    if buf.String() != "test response" {
        t.Errorf("Uncompressed body = %q, want 'test response'", buf.String())
    }
}