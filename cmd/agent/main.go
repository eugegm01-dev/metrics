package main

import (
	"log"

	"github.com/eugegm01-dev/metrics/internal/app"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger", err)
	}
	defer logger.Sync()

	if err := app.RunAgent(); err != nil {
		logger.Fatal("Agent error", zap.Error(err))
	}
}
