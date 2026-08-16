// Package cell runs the two arms of one job under a single supervising process
// and records whether they can be paired.
//
// It exists to prevent one failure rather than to detect it. Killing a session
// between the two arms of a cell does not merely cost time: the finished arm is
// burned, because it can never be paired, and the money spent on it is gone.
// The old loop had a function that caught a half-pair after the fact. Catching
// it is not preventing it.
package cell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/luuuc/sense/lab/internal/run"
)

// Arm is one side of a cell: a name, and what running it does.
//
// The work is a function rather than a session spec because supervision is all
// this package knows about. What an arm has to do to produce a run — prepare an
// environment, take a worktree, configure a subject — belongs to whoever is
// running the cell.
type Arm struct {
	Name string
	Run  func(ctx context.Context, dir string) (run.Meta, error)
}

// Record is the cell's own metadata, written whether or not the cell finished.
type Record struct {
	// Arms maps an arm's name to its run directory, for the arms that ran.
	Arms map[string]string `json:"arms"`
	// Complete says whether every arm reached a terminal state. Only a complete
	// cell can be paired.
	Complete bool `json:"complete"`
	// Burned names the arms that finished and can never be paired, because the
	// cell was interrupted before its other side ran. Named rather than
	// counted: the point is that a later pass can recognise the specific run
	// and refuse it, not that it knows one exists.
	Burned []string `json:"burned,omitempty"`
	// Unusable names the arms with no result to pair: interrupted part way, or
	// never started at all.
	Unusable []string `json:"unusable,omitempty"`
}

// recordFile is the cell's self-describing record, beside the arms it holds.
const recordFile = "cell-meta.json"

// Run launches every arm in order, under this process, and writes the cell
// record. Both arms are launched by the same supervising process, so there is
// no detached shell to be reaped and no launch to confirm afterwards.
//
// An interruption after the first arm does not raise an error: it produces a
// record that names the burned run, which is what a later pairing attempt needs
// in order to refuse it.
func Run(ctx context.Context, dir string, arms []Arm) (Record, error) {
	if len(arms) == 0 {
		return Record{}, errors.New("a cell with no arms")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Record{}, fmt.Errorf("prepare cell directory: %w", err)
	}

	rec := Record{Arms: map[string]string{}, Complete: true}
	for i, arm := range arms {
		// Checked before the spawn as well as after: an interrupt that arrives
		// while the previous arm was being recorded must not start another
		// session that is certain to be killed. That is a second arm's worth of
		// spend on a cell that can never be paired.
		if ctx.Err() != nil {
			return incomplete(dir, rec, arms[i:])
		}
		at := filepath.Join(dir, arm.Name)

		m, err := arm.Run(ctx, at)
		if ctx.Err() != nil {
			// The arm was cut short. It failed BECAUSE the cell was stopped,
			// so it is an interruption to be recorded rather than an error to
			// be reported: without the record nothing on disk names the arms
			// that finished, and a later pass would pair one of them.
			return incomplete(dir, rec, arms[i:])
		}
		if err != nil {
			return Record{}, fmt.Errorf("cell arm %s: %w", arm.Name, err)
		}
		if m.Outcome == run.Interrupted {
			// The arm that was cut has no result to pair. The arms before it
			// finished and are burned.
			return incomplete(dir, rec, arms[i:])
		}
		rec.Arms[arm.Name] = at
	}
	return rec, write(dir, rec)
}

// incomplete records a cell that was stopped part way. Every arm already in the
// record finished and is burned; every arm from the interruption onwards has no
// result to pair, whether it was cut or never started.
func incomplete(dir string, rec Record, remaining []Arm) (Record, error) {
	rec.Complete = false
	for name := range rec.Arms {
		rec.Burned = append(rec.Burned, name)
	}
	slices.Sort(rec.Burned)
	for _, arm := range remaining {
		rec.Unusable = append(rec.Unusable, arm.Name)
	}
	return rec, write(dir, rec)
}

// Pair reports the arms of a complete cell, ready to be compared.
//
// It refuses an incomplete one by name. The baseline's budget derives from its
// paired sense wall, so a burned arm has no partner it could ever be measured
// against, and pairing it with a later run would compare two things that were
// never a cell.
func Pair(dir string) (map[string]run.Meta, error) {
	rec, err := ReadRecord(dir)
	if err != nil {
		return nil, err
	}
	if !rec.Complete {
		return nil, fmt.Errorf("cell %s is incomplete: %v has no result, and %v can never be paired",
			dir, rec.Unusable, rec.Burned)
	}
	metas := make(map[string]run.Meta, len(rec.Arms))
	for name, at := range rec.Arms {
		m, err := run.Read(at)
		if err != nil {
			return nil, fmt.Errorf("cell %s: arm %s: %w", dir, name, err)
		}
		metas[name] = m
	}
	return metas, nil
}

// ReadRecord reports the cell recorded in dir. A directory with no record is a
// cell that was stopped by something that could not write one, which is the
// same rule a run directory follows: invalid on discovery, never resumed.
func ReadRecord(dir string) (Record, error) {
	b, err := os.ReadFile(filepath.Join(dir, recordFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Record{}, fmt.Errorf("cell %s: %w", dir, run.ErrNoTerminalState)
	case err != nil:
		return Record{}, fmt.Errorf("read cell %s: %w", dir, err)
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return Record{}, fmt.Errorf("cell %s: unreadable record: %w", dir, err)
	}
	return rec, nil
}

func write(dir string, rec Record) error {
	// Record is a fixed struct of strings and bools, so encoding cannot fail.
	b, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, recordFile), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write cell record: %w", err)
	}
	return nil
}
