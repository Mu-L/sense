// Command sense-lab is the bench instrument for Sense: it measures whether
// Sense makes an AI coding agent reach answers it cannot reach without it.
//
// It is not shipped to Sense users, is not on the install path, and is built
// from source by whoever develops Sense.
package main

import (
	"os"

	"github.com/luuuc/sense/lab/internal/cli"
)

// osExit indirects os.Exit so the one-line main wrapper is testable without
// killing the test process. Same seam as cmd/sense.
var osExit = os.Exit

func main() {
	osExit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
