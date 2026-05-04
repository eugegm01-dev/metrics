package middleware

import (
	"net"
	"net/http"
)

func TrustedSubnetMiddleware(trustedSubnet string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if trustedSubnet == "" {
			return next
		}

		_, ipNet, err := net.ParseCIDR(trustedSubnet)
		if err != nil {
			// На старте должна быть проверка, здесь fallback
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ipStr := r.Header.Get("X-Real-IP")
			if ipStr == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			ip := net.ParseIP(ipStr)
			if ip == nil || !ipNet.Contains(ip) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
