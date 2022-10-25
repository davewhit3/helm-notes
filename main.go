package main

import (
	"fmt"
	"os"

	"github.com/davewhit3/helm-notes/cmd"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
