package audit

// Observer определяет интерфейс наблюдателя
type Observer interface {
	Update(event *AuditEvent) error
	Close() error
}
