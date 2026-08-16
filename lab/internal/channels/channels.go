// Package channels enumerates every route by which Sense reaches an agent, and
// proves an arm has none of them.
//
// The whole measurement rests on one claim: the baseline arm cannot reach
// Sense, and the sense arm can. If that claim is wrong in either direction,
// every number in the corpus is wrong and nothing about it looks wrong.
//
// The repository routes are DERIVED, by running `sense setup` in a throwaway
// project and reading back what it wrote. A written-down list is the failure
// this package exists to prevent: a channel added to the product would leave
// the bench's copy five entries long, every check would still pass, and the
// baseline arm would have acquired a route nobody enumerated.
package channels

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Kind says where a channel lives, which decides how it is checked.
type Kind string

const (
	// Repository is a file `sense setup` writes into the project: the MCP
	// registration, the routing guidance, the hook settings, the skills and the
	// agents. A clean worktree closes these.
	Repository Kind = "repository"
	// Path is the Sense binary, reachable as a CLI fallback. The arm's PATH
	// closes it.
	Path Kind = "path"
	// Home is persisted memory, outside the repository and keyed off the
	// repository path. This is the one a reasonable implementation misses,
	// because nothing in the repository or the prompt shows it. Only a
	// disposable HOME closes it.
	Home Kind = "home"
)

// Channel is one route, and the name is what a failing check reports. A check
// that says "contamination detected" tells nobody which route leaked.
type Channel struct {
	Name string
	Kind Kind
	// Rel is the repository-relative path, for a Repository channel only.
	Rel string
}

// setupTool is the agent tool whose channels are derived. Cycle 03 is Claude
// Code only; naming it here rather than letting detection choose keeps the
// derived list from depending on what happens to be installed on the machine
// running the bench.
const setupTool = "claude-code"

// Derive runs `sense setup` in a throwaway project and reports the repository
// channels it wrote, plus the two that no file on disk can reveal.
//
// workDir must not exist: Derive creates it, along with the throwaway project
// and a throwaway HOME beside it, so the probe cannot touch the operator's
// configuration on its way to telling us what the product writes.
func Derive(ctx context.Context, senseBin, workDir string) ([]Channel, error) {
	project := filepath.Join(workDir, "project")
	home := filepath.Join(workDir, "home")
	for _, dir := range []string{project, home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("derive channels: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, senseBin, "setup", "--tools", setupTool)
	cmd.Dir = project
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("derive channels: sense setup: %w: %s", err, out)
	}

	tree, err := Tree(project)
	if err != nil {
		return nil, fmt.Errorf("derive channels: %w", err)
	}
	written := Sorted(tree)
	if len(written) == 0 {
		// A silent no-op would produce an empty channel list, and an empty list
		// makes every absence check pass. That is the shape of a leak that
		// reads as a clean bill of health.
		return nil, errors.New("derive channels: sense setup wrote nothing; the derived list would be empty and every check would pass")
	}

	cs := make([]Channel, 0, len(written)+2)
	for _, rel := range written {
		cs = append(cs, Channel{Name: rel, Kind: Repository, Rel: rel})
	}
	return append(cs,
		Channel{Name: "the sense binary on PATH", Kind: Path},
		Channel{Name: "the persisted memory directory", Kind: Home},
	), nil
}

// Arm is where one arm's channels would be if it had them.
type Arm struct {
	// Repo is the worktree the session runs against.
	Repo string
	// Home is the disposable HOME.
	Home string
	// PathValue is the PATH the session was given.
	PathValue string
	// SenseBinary is the binary name to look for on that PATH.
	SenseBinary string
}

// Absent reports every channel the arm can still reach, by name. An empty
// result is the only acceptable outcome for the baseline arm.
//
// Each channel is checked individually and reported by name, because "the
// baseline arm is contaminated" does not say which route leaked, and the route
// is the whole diagnosis.
func Absent(cs []Channel, a Arm) []string {
	var reached []string
	for _, c := range cs {
		if why := c.reachedBy(a); why != "" {
			reached = append(reached, why)
		}
	}
	return reached
}

// reachedBy reports how the arm reaches this channel, or "" if it cannot.
func (c Channel) reachedBy(a Arm) string {
	switch c.Kind {
	case Repository:
		if exists(filepath.Join(a.Repo, c.Rel)) {
			return fmt.Sprintf("%s: %s is in the worktree", c.Name, c.Rel)
		}
	case Path:
		if at := onPath(a.PathValue, a.SenseBinary); at != "" {
			return fmt.Sprintf("%s: %s is executable at %s", c.Name, a.SenseBinary, at)
		}
	case Home:
		if dir := MemoryDir(a.Home, a.Repo); exists(dir) {
			return fmt.Sprintf("%s: %s exists", c.Name, dir)
		}
		// The exact directory name depends on how the agent tool flattens a
		// repository path, which is observed rather than documented. The whole
		// .claude tree is checked as well, so an exact match is not what the
		// proof rests on.
		if claude := filepath.Join(a.Home, ".claude"); exists(claude) {
			return fmt.Sprintf("%s: %s exists in the disposable HOME", c.Name, claude)
		}
	}
	return ""
}

// MemoryDir is where an agent tool persists memory for a repository: keyed off
// the repository path, outside the repository, and read by every session that
// runs against that path.
//
// The flattening rule — every character outside [A-Za-z0-9-] becomes a dash —
// is taken from the directories the tool has actually produced, not from
// documentation. Absent therefore also checks the whole .claude tree, so the
// proof does not rest on this staying exact.
func MemoryDir(home, repo string) string {
	return filepath.Join(home, ".claude", "projects", flatten(repo), "memory")
}

func flatten(path string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '-'
		}
	}, path)
}

// onPath reports where name is executable on the given PATH, or "".
func onPath(pathValue, name string) string {
	if name == "" {
		return ""
	}
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
