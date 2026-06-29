package main

import (
	"fmt"
	"os"

	"github.com/epoxsizer/kan/internal/cli"
)

var (
	version = "0.1.5"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.New(version, commit, date).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
