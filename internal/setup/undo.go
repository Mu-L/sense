package setup

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// This file is the inverse of setup.go: it removes what Run wrote. The split
// mirrors the write side. Each tool's teardown (unconfigureX) lives in that
// tool's own file next to its configureX, and the registry pairs them. What
// lives here is the project-level teardown that belongs to no single tool (the
// .sense index directory and the .gitignore entry, both written by `sense
// scan`) plus the filesystem helpers the per-tool teardowns share.

// cacheDirNote names the machine-wide embedding runtime cache. It is left in
// place on purpose: every project on the machine shares it, so removing it to
// undo one project's setup would force a re-extract for all the others.
// The path is embed's (see internal/embed/bundle.go, ortCacheDir).
const cacheDirNote = "the shared embedding runtime in ~/.sense/cache (or $SENSE_CACHE_DIR) is left in place, every project on this machine shares it"

// Undo removes every file `sense setup` wrote into root, plus the .sense index
// directory and the .gitignore entry `sense scan` added. It never deletes a
// file it does not own: a config it merged into (.mcp.json, settings.json) has
// only Sense's entries stripped, and the file survives with the user's own
// content intact. Files that held nothing but Sense's section are deleted
// rather than left empty.
//
// When opts names specific tools the teardown is partial. Only those tools'
// integration files go, and the index stays, since the index is not any one
// tool's to remove.
func Undo(root string, out io.Writer, opts *Options) (*Result, error) {
	res := &Result{}
	res.Notes = append(res.Notes, cacheDirNote)

	err := undoTools(root, res, opts)
	if err == nil && IndexInScope(opts) {
		err = undoProject(root, res)
	}

	// The summary prints on the way out either way, and the result comes back
	// populated even when err is non-nil. A teardown that stops halfway has
	// already changed the project; a user who is only told "it failed" cannot
	// tell which half they are in.
	printUndoSummary(out, res, err == nil)
	return res, err
}

// undoTools tears each tool down in turn, stopping at the first failure. A
// tool that fails partway still returns what it managed to remove, so the
// result accumulates the real state of the project rather than losing it.
func undoTools(root string, res *Result, opts *Options) error {
	for _, t := range resolveUndoTools(opts) {
		tr, err := unconfigureTool(root, t)
		if tr != nil {
			res.Tools = append(res.Tools, *tr)
		}
		if err != nil {
			return fmt.Errorf("remove %s integration: %w", t.DisplayName(), err)
		}
	}
	return nil
}

// resolveUndoTools returns the tools to tear down: the ones named in opts, or
// every tool Sense knows how to configure. Teardown defaults to all of them
// rather than the detected ones on purpose. Detection reports what is
// installed now, not what was configured then, and a tool uninstalled since
// setup ran still has its Sense files sitting in the repo.
func resolveUndoTools(opts *Options) []Tool {
	if opts != nil && len(opts.Tools) > 0 {
		return opts.Tools
	}
	return AllTools()
}

// IndexInScope reports whether the .sense index is this teardown's to remove.
// It is, only when the caller did not narrow the run with --tools: a teardown
// aimed at one tool unwires that tool, and the index is not any one tool's to
// take. Naming every tool explicitly still counts as narrowed, because the
// caller asked for tools, not for the project.
//
// Exported because `sense setup --undo` has to know the same thing to decide
// whether a held index lock blocks the run, and one predicate in one place is
// what keeps the two from drifting apart.
func IndexInScope(opts *Options) bool {
	return opts == nil || len(opts.Tools) == 0
}

// undoProject removes the artifacts `sense scan` wrote: the .sense directory
// and the .gitignore entry pointing at it.
func undoProject(root string, res *Result) error {
	senseDir := filepath.Join(root, ".sense")

	if hadConfig(senseDir) {
		res.Notes = append(res.Notes, "removed .sense/config.yml, which was yours and not generated; restore it from version control if you kept a copy")
	}

	size, existed := dirSize(senseDir)
	if existed {
		if err := os.RemoveAll(senseDir); err != nil {
			return fmt.Errorf("remove .sense: %w", err)
		}
		res.Project = append(res.Project, fmt.Sprintf(".sense/ (index, %s)", humanSize(size)))
	}

	removed, err := removeSenseFromGitignore(root)
	if err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}
	if removed {
		res.Project = append(res.Project, ".gitignore entry")
	}
	return nil
}

// hadConfig reports whether the index directory holds a hand-written
// config.yml. Everything else under .sense/ is generated and regenerates on
// the next scan; config.yml is the one file a user authored, so its removal
// is called out rather than folded into the directory line.
func hadConfig(senseDir string) bool {
	_, err := os.Stat(filepath.Join(senseDir, "config.yml"))
	return err == nil
}

// gitignoreComment is the header `sense scan` writes above its entry (see
// addSenseToGitignore in internal/scan/util.go). Matching it lets teardown
// take the comment with the line it annotates, instead of leaving a comment
// explaining an entry that is no longer there.
const gitignoreComment = "# Sense index"

// removeSenseFromGitignore drops the .sense entry, and the Sense comment above
// it, from root/.gitignore. Other entries and the file itself are left alone:
// a .gitignore is the user's file, never Sense's, even when Sense added a line
// to it.
func removeSenseFromGitignore(root string) (bool, error) {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	var kept []string
	removed := false
	for _, line := range strings.Split(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case ".sense", ".sense/":
			removed = true
			// Drop the comment this entry belongs to, if it sits just above.
			if n := len(kept); n > 0 && strings.HasPrefix(strings.TrimSpace(kept[n-1]), gitignoreComment) {
				kept = kept[:n-1]
			}
		default:
			kept = append(kept, line)
		}
	}
	if !removed {
		return false, nil
	}

	content := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if content == "" {
		return true, os.WriteFile(path, nil, 0o644)
	}
	return true, os.WriteFile(path, []byte(content+"\n"), 0o644)
}

// dirSize sums the bytes under dir, reporting existed=false for a missing one
// so a caller can tell "nothing to remove" from "removed something empty". It
// cannot fail: the number goes in a summary line, so a directory that will not
// walk undercounts rather than aborting a teardown that is otherwise fine.
func dirSize(dir string) (size int64, existed bool) {
	if _, err := os.Stat(dir); err != nil {
		return 0, false
	}

	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			size += info.Size()
		}
		return nil
	})
	return size, true
}

// humanSize renders a byte count the way the rest of the CLI talks about
// index size: whole units, no false precision.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 2 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMG"[exp])
}

// removeFile deletes path. A file that is already gone is success: teardown is
// idempotent, so running it twice is not an error the second time.
func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// removeOwnedFile deletes a file Sense wrote outright. Only for files Sense
// owns end to end (a skill, an agent, the OpenCode plugin), never for a config
// it merged into, so the outcome is only ever deleted or unchanged.
func removeOwnedFile(path string) (outcome, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return outcomeUnchanged, nil
		}
		return outcomeUnchanged, err
	}
	if err := removeFile(path); err != nil {
		return outcomeUnchanged, err
	}
	return outcomeDeleted, nil
}

// removeDirIfEmpty removes dir when nothing is left in it. A non-empty
// directory is left alone: the user's own files live in .claude/ too, and an
// os.Remove that fails on them is exactly the behaviour we want.
func removeDirIfEmpty(dir string) {
	_ = os.Remove(dir)
}

// outcome is what a teardown step did to one file. The distinction reaches the
// summary: reporting a stripped config as "removed .mcp.json" would claim a
// deletion that did not happen, since that file is still on disk with the
// user's own servers in it.
type outcome int

const (
	outcomeUnchanged outcome = iota // held nothing of Sense's
	outcomeStripped                 // Sense's entries removed, the file survives
	outcomeDeleted                  // held nothing but Sense's, so it is gone
)

// record notes a torn-down path on the result, in the words of what actually
// happened to it. An unchanged file is not listed: the summary says what was
// there, not what was looked for.
func (tr *ToolResult) record(o outcome, rel string) {
	switch o {
	case outcomeStripped:
		tr.Files = append(tr.Files, "Sense entries from "+rel)
	case outcomeDeleted:
		tr.Files = append(tr.Files, rel)
	case outcomeUnchanged:
	}
}

// printUndoSummary lists what came out of the project and closes with a line
// saying whether that was all of it. complete is false when the teardown
// stopped on an error, in which case the caller reports the error itself and
// this only has to make the partial state legible.
func printUndoSummary(out io.Writer, res *Result, complete bool) {
	touched := false

	for _, tr := range res.Tools {
		if len(tr.Files) == 0 {
			continue
		}
		touched = true
		_, _ = fmt.Fprintf(out, "Removing %s integration...\n", tr.Tool.DisplayName())
		for _, f := range tr.Files {
			_, _ = fmt.Fprintf(out, "  removed %s\n", f)
		}
		_, _ = fmt.Fprintln(out, "")
	}

	if len(res.Project) > 0 {
		touched = true
		_, _ = fmt.Fprintln(out, "Removing index...")
		for _, p := range res.Project {
			_, _ = fmt.Fprintf(out, "  removed %s\n", p)
		}
		_, _ = fmt.Fprintln(out, "")
	}

	switch {
	case !complete && !touched:
		// Nothing was removed and something went wrong: the error is the whole
		// story, and a summary of nothing would only bury it.
		return
	case !complete:
		_, _ = fmt.Fprintln(out, "Stopped early. Everything listed above is removed, the rest is still in place.")
	case !touched:
		// Notes are guidance about what a teardown spares. With nothing torn
		// down there is nothing to spare, so they would be noise.
		_, _ = fmt.Fprintln(out, "Nothing to remove. No Sense files found in this project.")
		return
	default:
		_, _ = fmt.Fprintln(out, "Done. Sense is removed from this project. The sense binary is untouched.")
	}

	for _, n := range res.Notes {
		_, _ = fmt.Fprintf(out, "  note: %s\n", n)
	}
}
