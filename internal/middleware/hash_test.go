package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHashMiddleware(t *testing.T) {
    key := "testkey"
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })
    mw := HashMiddleware(key)(handler)

    req := httptest.NewRequest("POST", "/", nil)
    rr := httptest.NewRecorder()
    mw.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rr.Code)
    }
}