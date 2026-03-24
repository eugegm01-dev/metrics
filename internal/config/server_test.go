package config

import (
	"flag"
	"os"
	"testing"
	"time"
)

func TestParseServerConfig(t *testing.T) {
    oldArgs := os.Args
    defer func() { os.Args = oldArgs }()
    // Сбрасываем флаги, чтобы избежать конфликта
    flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

    os.Args = []string{"cmd"}
    cfg, err := ParseServerConfig()
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Addr != "localhost:8080" {
        t.Errorf("Addr = %s, want localhost:8080", cfg.Addr)
    }
    if cfg.StoreInterval != 300*time.Second {
        t.Errorf("StoreInterval = %v, want 5m0s", cfg.StoreInterval)
    }
    if cfg.FileStoragePath != "/tmp/metrics-db.json" {
        t.Errorf("FileStoragePath = %s, want /tmp/metrics-db.json", cfg.FileStoragePath)
    }
    if !cfg.Restore {
        t.Error("Restore should be true by default")
    }
}