//go:build unix

package run

import (
	"os/exec"
	"syscall"
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

// killGroup signals the child's whole process group. It runs as cmd.Cancel,
// which os/exec invokes only after the process has started, so cmd.Process is
// set and the negative pid is certainly this session's group rather than one
// the OS has recycled.
func killGroup(cmd *exec.Cmd) error {
	// A group that has already gone is the normal race, not a failure.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return nil
}
