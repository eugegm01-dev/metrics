package audit

import "sync"
	"go.uber.org/zap"

// Subject реализует паттерн «наблюдатель» для событий аудита.
// Хранит список наблюдателей и уведомляет их при возникновении событий.
type Subject struct {
	mu        sync.RWMutex
	observers []Observer
}
// Observer определяет интерфейс наблюдателя за событиями аудита.
type Observer interface {
	Update(event *AuditEvent) error
	Close() error
}
// NewSubject создаёт новый Subject.
func NewSubject() *Subject {
	return &Subject{
		observers: make([]Observer, 0),
	}
}

// Attach добавляет наблюдателя к субъекту.
func (s *Subject) Attach(observer Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, observer)
}

// Notify асинхронно отправляет событие всем наблюдателям.
func (s *Subject) Notify(event *AuditEvent) {
	s.mu.RLock()
	observers := make([]Observer, len(s.observers))
	copy(observers, s.observers)
	s.mu.RUnlock()

	for _, observer := range observers {
		go func(obs Observer) {
			if err := obs.Update(event); err != nil {
				zap.L().Error("audit update failed", zap.Error(err))
			}
		}(observer)
	}
}

// Close закрывает всех наблюдателей и возвращает возможную ошибку.
func (s *Subject) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for _, observer := range s.observers {
		if err := observer.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
