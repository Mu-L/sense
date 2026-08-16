package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/luuuc/sense/lab/internal/catalog"
)

// defaultConfigDir is where the catalog lives, relative to the working
// directory. It is a path, not a compiled-in fact: nothing about which
// subjects, agents, models or repositories exist is in the binary.
const defaultConfigDir = "lab"

func configFlag(fs *flag.FlagSet, into *string) {
	fs.StringVar(into, "config", defaultConfigDir, "directory holding subjects/, agents/, models/, repos/ and executors/")
}

// catalogCmd loads the config and prints what it found. It is the smallest
// thing that proves the config is real: it spends nothing, runs nothing, and
// fails on a malformed file rather than on a run.
func catalogCmd(args []string, stdout, stderr io.Writer) int {
	var dir string
	fs := flag.NewFlagSet("catalog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &dir)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	c, err := catalog.Load(dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab catalog: %v\n", err)
		return exitError
	}

	for _, s := range catalog.IDs(c.Subjects) {
		_, _ = fmt.Fprintf(stdout, "subject  %-14s %s\n", s, c.Subjects[s].Kind)
	}
	for _, a := range catalog.IDs(c.Agents) {
		_, _ = fmt.Fprintf(stdout, "agent    %-14s %s\n", a, c.Agents[a].Binary)
	}
	for _, m := range catalog.IDs(c.Models) {
		_, _ = fmt.Fprintf(stdout, "model    %-14s %s\n", m, c.Models[m].Provider)
	}
	for _, r := range catalog.IDs(c.Repos) {
		_, _ = fmt.Fprintf(stdout, "repo     %-14s %s @ %.8s\n", r, c.Repos[r].Stack, c.Repos[r].Commit)
	}
	for _, e := range catalog.IDs(c.Executors) {
		_, _ = fmt.Fprintf(stdout, "executor %-14s preserves %v\n", e, c.Executors[e].PreservesAuth)
	}
	return exitOK
}
