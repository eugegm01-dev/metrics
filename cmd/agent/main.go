package main

import (
	"fmt"
	"os"

	"github.com/eugegm01-dev/metrics/internal/app"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		fatal(fmt.Errorf("failed to create logger: %w", err))
	}
	defer func() { _ = logger.Sync() }()
	if err := app.RunAgent(); err != nil {
		fatal(fmt.Errorf("agent error: %w", err))
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
