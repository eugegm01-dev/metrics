package main

import (
	"github.com/eugegm01-dev/metrics/internal/app"
)

func main() {
	if err := app.RunAgent(); err != nil {
		panic(err)
	}
}
