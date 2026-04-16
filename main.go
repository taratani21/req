package main

import (
	"os"

	"github.com/taratani21/req/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
