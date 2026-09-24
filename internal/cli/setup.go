package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/luuuc/sense/internal/freshen"
	"github.com/luuuc/sense/internal/setup"
)

const setupHelp = `usage: sense setup [flags]

Configure AI tool integrations for this project. Auto-detects installed
tools (Claude Code, Cursor, Codex CLI, Opencode, Windsurf, Cline) and writes
integration files.

Flags:
  --tools   comma-separated list of tools to configure (overrides detection)
  --undo    remove what setup wrote, plus the .sense index

--undo removes Sense from the project, not from the machine: the sense binary
stays where it is. Configs Sense merged into (.mcp.json, .claude/settings.json)
keep everything you put there and lose only Sense's entries. Narrowing it with
--tools unwires those tools and leaves the index alone.

Examples:
  sense setup                           # auto-detect and configure all
  sense setup --tools cursor            # configure Cursor only
  sense setup --tools claude-code,codex-cli,opencode
  sense setup --undo                    # remove Sense from this project
  sense setup --undo --tools cursor     # unwire Cursor, keep the index
`

// RunSetup configures AI tool integrations for the project, or removes them
// with --undo.
func RunSetup(args []string, cio IO) int {
	fs := flag.NewFlagSet("sense setup", flag.ContinueOnError)
	fs.SetOutput(cio.Stderr)
	fs.Usage = func() { _, _ = fmt.Fprint(cio.Stderr, setupHelp) }
	toolsFlag := fs.String("tools", "", "comma-separated list of tools to configure (claude-code,cursor,codex-cli,opencode,windsurf,cline)")
	undoFlag := fs.Bool("undo", false, "remove what setup wrote, plus the .sense index")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitGeneralError
	}

	var opts setup.Options
	if *toolsFlag != "" {
		tools, err := setup.ParseTools(*toolsFlag)
		if err != nil {
			_, _ = fmt.Fprintln(cio.Stderr, "sense setup:", err)
			return ExitGeneralError
		}
		opts.Tools = tools
	}

	if *undoFlag {
		return runSetupUndo(cio, &opts)
	}

	if *toolsFlag == "" {
		setup.PrintDetection(cio.Stdout)
	}
	if _, err := setup.Run(cio.Dir, cio.Stdout, &opts); err != nil {
		_, _ = fmt.Fprintln(cio.Stderr, "sense setup:", err)
		return ExitGeneralError
	}
	return ExitSuccess
}

// runSetupUndo removes the integration files, refusing while an indexer holds
// the write lock. Deleting .sense/ out from under a live `sense mcp` server
// leaves it writing to an index nobody can read, so the fix is to stop the
// server, not to delete more carefully. A run that does not reach the index
// (setup.IndexInScope says so) has nothing to wait for.
func runSetupUndo(cio IO, opts *setup.Options) int {
	if setup.IndexInScope(opts) && freshen.IsWriterLocked(cio.Dir) {
		_, _ = fmt.Fprintln(cio.Stderr, "sense setup --undo: an indexer is running on this project (.sense/index.lock is held).")
		_, _ = fmt.Fprintln(cio.Stderr, "Stop it first. Quit the AI tool running `sense mcp`, or the `sense scan --watch` in your terminal, then run this again.")
		return ExitGeneralError
	}

	// Undo prints what it removed before returning, partial teardowns included,
	// so this error lands after the list rather than in place of it.
	if _, err := setup.Undo(cio.Dir, cio.Stdout, opts); err != nil {
		_, _ = fmt.Fprintln(cio.Stderr, "sense setup --undo:", err)
		return ExitGeneralError
	}
	return ExitSuccess
}
