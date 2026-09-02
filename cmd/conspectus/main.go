package main

import (
	"fmt"
	"os"

	"conspectus/internal/app"
)

var (
	version = "2.0.0-dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	info := app.BuildInfo{Version: version, Commit: commit, Date: date}
	if err := app.Run(os.Args[1:], info); err != nil {
		fmt.Fprintln(os.Stderr, "conspectus:", err)
		os.Exit(1)
	}
}
