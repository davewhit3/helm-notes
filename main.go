package main

import (
	"os"

	"github.com/davewhit3/helm-notes/cmd"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		os.Exit(1)
	}
}
