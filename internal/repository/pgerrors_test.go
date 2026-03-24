package repository

import (
    "errors"
    "testing"

    "github.com/jackc/pgerrcode"
    "github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresErrorClassifier_Classify(t *testing.T) {
    c := NewPostgresErrorClassifier()
    tests := []struct {
        name     string
        err      error
        expected PGErrorClassification
    }{
        {"nil error", nil, NonRetriable},
        {"connection exception", &pgconn.PgError{Code: pgerrcode.ConnectionException}, Retriable},
        {"deadlock detected", &pgconn.PgError{Code: pgerrcode.DeadlockDetected}, Retriable},
        {"unique violation", &pgconn.PgError{Code: "23505"}, NonRetriable},
        {"unknown error", errors.New("some error"), NonRetriable},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := c.Classify(tt.err)
            if got != tt.expected {
                t.Errorf("Classify() = %v, want %v", got, tt.expected)
            }
        })
    }
}