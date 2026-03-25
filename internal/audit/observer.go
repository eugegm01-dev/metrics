package audit

// Observer определяет интерфейс наблюдателя за событиями аудита.
type Observer interface {
	Update(event *AuditEvent) error
	Close() error
}
