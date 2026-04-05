package main

import (
	"fmt"
	"os"

	_ "net/http/pprof"

	"github.com/eugegm01-dev/metrics/internal/app"
	"go.uber.org/zap"
)

var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	printBuildInfo()

	logger, err := zap.NewDevelopment()
	if err != nil {
		fatal(fmt.Errorf("failed to create logger: %w", err))
	}
	defer func() { _ = logger.Sync() }()
	if err := app.RunServer(); err != nil {
		fatal(fmt.Errorf("server error: %w", err))
	}
}

func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
