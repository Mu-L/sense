//go:build unix

package run

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup puts the child in its own process group so the whole tree can
// be signalled at once. An agent CLI spawns children of its own, and killing
// only the parent leaves them alive holding the output pipe: the wall passes,
// the run never ends, and the cell's other arm is burned with it. That has
// already happened once, to a 312-second run reaped by a launcher that looked
// detached and was not.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killPoll is how often the group is checked during its grace period. Short
// enough that a session which stops promptly is not held for the whole grace.
const killPoll = 20 * time.Millisecond

// stopGroup asks the child's whole process group to stop, gives it the grace
// period, and then kills it.
//
// The escalation is not politeness. An agent CLI traps its first signal and
// spends seconds flushing: a supervisor that sends one signal and moves on
// truncates a flush that was about to complete, and a supervisor that sends
// only SIGKILL never lets it start. Ten seconds of graceful cleanup is normal
// behaviour for the thing being measured.
//
// It runs as cmd.Cancel, which os/exec invokes only after the process has
// started, so cmd.Process is set and the negative pid is certainly this
// session's group rather than one the OS has recycled.
func stopGroup(cmd *exec.Cmd, grace time.Duration) error {
	pgid := -cmd.Process.Pid
	// A group that has already gone is the normal race, not a failure.
	_ = syscall.Kill(pgid, syscall.SIGTERM)

	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		// Signal 0 asks whether the group is still there without touching it.
		if err := syscall.Kill(pgid, 0); err != nil {
			return nil
		}
		time.Sleep(killPoll)
	}
	_ = syscall.Kill(pgid, syscall.SIGKILL)
	return nil
}
