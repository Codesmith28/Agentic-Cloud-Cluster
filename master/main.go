package main

import (
	"os"
	"strings"

	"master/internal/app"
	"master/internal/config"
)

func main() {
	cfg := config.LoadConfig()

	// Handle non-interactive test runner commands
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "test") {
		os.Exit(app.RunTestCommand(cfg, os.Args[2:]))
	}

	// Run master node application
	app.New(cfg).Run()
}
