package main

import (
	"github.com/eugegm01-dev/metrics/internal/app"
)

func main() {
	if err := app.RunServer(); err != nil {
		panic(err)
	}
}
