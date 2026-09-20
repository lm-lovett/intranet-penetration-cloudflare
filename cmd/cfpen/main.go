package main

import (
	"fmt"
	"os"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "cfpen: %v\n", err)
		os.Exit(1)
	}
}
