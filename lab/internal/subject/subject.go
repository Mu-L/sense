// Package subject prepares an arm's repository: what the code-intelligence
// treatment installs, and nothing else.
//
// The two arms are deliberately asymmetric. The sense arm gets the product's
// own setup surface, whatever that currently writes, because the arm must see
// what a real user sees. The baseline arm gets nothing, and lab/internal/channels
// proves it.
package subject

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/channels"
)

// Sense prepares the sense arm's worktree: index it, then configure it, in that
// order and both deliberately.
//
// The order matters more than it looks. `sense scan` runs setup itself whenever
// the index directory is absent, and the client detector falls back to Claude
// Code when nothing in the environment matches — so a headless scan of an
// unindexed repository writes the routing channels as a side effect, and
// nothing about it is visible afterwards. Creating the index directory first
// makes that first-run branch false, so configuring the repository stays an
// explicit act with a recorded result.
//
// It returns what setup wrote, as path to content hash. A change to
// `sense setup` changes what the sense arm sees, which changes the measurement,
// and nothing about that is visible in a result months later. Recorded per run,
// two runs far apart are comparable on what was installed rather than on an
// assumption that it was the same.
func Sense(ctx context.Context, senseBin, repo string) (map[string]string, error) {
	// The index lives at <repo>/.sense, where the MCP server the agent launches
	// looks for it. Putting it anywhere else is a sense arm that is configured,
	// reports no error, and reaches an empty index.
	senseDir := filepath.Join(repo, ".sense")
	if err := os.MkdirAll(senseDir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare index directory: %w", err)
	}

	// -dir, not a positional path: sense scan silently ignores positionals.
	scan := exec.CommandContext(ctx, senseBin, "scan", "-dir", repo)
	if out, err := scan.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("index the subject: %w: %s", err, out)
	}

	before, err := treeOf(repo)
	if err != nil {
		return nil, err
	}

	setup := exec.CommandContext(ctx, senseBin, "setup", "--tools", "claude-code")
	setup.Dir = repo
	if out, err := setup.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("configure the subject: %w: %s", err, out)
	}

	after, err := treeOf(repo)
	if err != nil {
		return nil, err
	}
	return added(before, after), nil
}

// added is what the second tree holds that the first did not, or holds
// differently. Diffing rather than listing, so an index directory or a
// .gitignore line the scan left behind is not reported as something setup
// wrote.
func added(before, after map[string]string) map[string]string {
	wrote := map[string]string{}
	for path, hash := range after {
		if before[path] != hash {
			wrote[path] = hash
		}
	}
	return wrote
}

// treeOf hashes every file below root, keyed by its relative path. Anything
// under .git or .sense is skipped: both are machinery rather than a channel,
// and both are large enough to make this the slowest step in a run.
func treeOf(root string) (map[string]string, error) {
	tree, err := channels.Tree(root, ".git", ".sense")
	if err != nil {
		return nil, fmt.Errorf("read the subject tree: %w", err)
	}
	return tree, nil
}
