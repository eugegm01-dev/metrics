package audit

import "time"

// AuditEvent представляет событие аудита
type AuditEvent struct {
	TS        int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// NewAuditEvent создаёт новое событие аудита
func NewAuditEvent(metrics []string, ipAddress string) *AuditEvent {
	return &AuditEvent{
		TS:        time.Now().Unix(),
		Metrics:   metrics,
		IPAddress: ipAddress,
	}
}
