package main

import (
	"fmt"
	"os"

	"github.com/cozy-labs/cozy-nextdb/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
