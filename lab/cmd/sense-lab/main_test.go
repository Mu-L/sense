package main

import (
	"os"
	"testing"
)

// main's whole job is handing cli.Run's exit code to os.Exit. Two different
// codes through the same seam, because one case cannot tell "passes the code
// through" from "always exits 2" — and that distinction is the entire claim.
func TestMainHandsRunExitCodeToOSExit(t *testing.T) {
	for _, tc := range []struct {
		name string
		arg  string
		want int
	}{
		{"a command that succeeds", "version", 0},
		{"a command that does not exist", "no-such-command", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldArgs, oldExit := os.Args, osExit
			t.Cleanup(func() { os.Args, osExit = oldArgs, oldExit })

			// -1 is a value Run never returns, so "never called" is
			// distinguishable from "called with 0" without a bool flag.
			got := -1
			osExit = func(code int) { got = code }
			os.Args = []string{"sense-lab", tc.arg}
			defer silenceOutput(t)()

			main()

			if got != tc.want {
				t.Fatalf("main passed exit code %d to osExit, want %d", got, tc.want)
			}
		})
	}
}

// silenceOutput points os.Stdout and os.Stderr at /dev/null and returns the
// restore func, for a test that exercises code writing to the real streams.
func silenceOutput(t *testing.T) func() {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	return func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		_ = devnull.Close()
	}
}
