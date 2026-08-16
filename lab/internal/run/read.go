package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNoTerminalState marks a run directory that was started and never finished.
var ErrNoTerminalState = errors.New("no terminal state: the run was interrupted by something that could not record it")

// Read reports the run recorded in dir.
//
// A directory with no run-meta.json holds a run that started and never reached
// a terminal state. Signals cover an interrupt; nothing covers a reboot or a
// power loss, so the gap is closed by a rule instead: such a directory is
// invalid on discovery and is never resumed. Resuming one would silently pair a
// run whose conditions nobody can reconstruct, and nothing about the resulting
// number would show it.
func Read(dir string) (Meta, error) {
	b, err := os.ReadFile(filepath.Join(dir, metaFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Meta{}, fmt.Errorf("run %s: %w", dir, ErrNoTerminalState)
	case err != nil:
		return Meta{}, fmt.Errorf("read run %s: %w", dir, err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("run %s: unreadable metadata: %w", dir, err)
	}
	if m.Outcome == "" {
		return Meta{}, fmt.Errorf("run %s: %w", dir, ErrNoTerminalState)
	}
	return m, nil
}
