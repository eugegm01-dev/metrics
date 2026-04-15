package audit

import "time"

// AuditEvent представляет событие аудита: список имён метрик и IP-адрес клиента.
type AuditEvent struct {
	TS        int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// NewAuditEvent создаёт AuditEvent с текущей временной меткой.
func NewAuditEvent(metrics []string, ipAddress string) *AuditEvent {
	return &AuditEvent{
		TS:        time.Now().Unix(),
		Metrics:   metrics,
		IPAddress: ipAddress,
	}
}
