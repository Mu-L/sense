package isolate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Reading the operator's credential is the effect half of this package, and it
// is in its own file for that reason: the provisioner beside it is a pure
// function of a Credential, so it can be tested with no keychain and no login,
// which is the only reason CI can hold the whole of that decision. A provisioner
// that read the store itself would make every environment test need a seat.
//
// It runs in the attended parent, once per cell, where the keychain is
// reachable. A session never reads a store: it is handed a file.

// HostConfigDir is the operator's own config directory: the tool's config
// variable if the host sets it, and the HOME-relative default otherwise. It is
// the same precedence the tool applies, so a host that has moved its config is
// read where it actually is.
func (r Route) HostConfigDir(lookup func(string) (string, bool)) (string, error) {
	if r.ConfigDirVar != "" {
		if v, ok := lookup(r.ConfigDirVar); ok && v != "" {
			return v, nil
		}
	}
	if r.ConfigDir == "" {
		return "", errors.New("the agent tool declares no config directory, so its credential cannot be located")
	}
	home, ok := lookup("HOME")
	if !ok || home == "" {
		return "", errors.New("cannot locate the host config directory: HOME is not set")
	}
	return filepath.Join(home, r.ConfigDir), nil
}

// HostCredential reads the operator's credential from wherever it lives.
//
// The file first and the platform store second, which is the order the agent
// tool's own sandbox provisioner uses. Both are tried on every platform rather
// than switched on runtime.GOOS: the file is the primary store on Linux and an
// undocumented but real one on macOS, and a machine with no `security` command
// simply fails that read and falls through. A GOOS branch here would be a second
// implementation nobody exercises.
func HostCredential(ctx context.Context, lookup func(string) (string, bool), r Route) (Credential, error) {
	dir, err := r.HostConfigDir(lookup)
	if err != nil {
		return Credential{}, err
	}
	if c, err := r.Read(dir); err == nil && c.Valid() {
		return c, nil
	}
	c, err := r.keychain(ctx, lookup)
	if err != nil {
		return Credential{}, fmt.Errorf("no usable credential in %s or the platform store: %w", dir, err)
	}
	if !c.Valid() {
		return Credential{}, errors.New("the platform store holds a credential with no token, expiry or scopes")
	}
	return c, nil
}

// keychainCmd builds the read. It is a variable so a test can state what the
// store answered without needing one.
var keychainCmd = func(ctx context.Context, account, service string) *exec.Cmd {
	return exec.CommandContext(ctx, "security",
		"find-generic-password", "-a", account, "-s", service, "-w")
}

// keychain reads the platform store.
//
// A failure here is ordinary rather than exceptional: the command does not exist
// off macOS, the item is absent on a machine that has never logged in, and the
// ACL denies a session with no GUI context. All three mean "no credential from
// this source", and the caller says so.
func (r Route) keychain(ctx context.Context, lookup func(string) (string, bool)) (Credential, error) {
	if r.Keychain == "" {
		return Credential{}, errors.New("the agent tool declares no platform credential store")
	}
	account, ok := lookup("USER")
	if !ok || account == "" {
		return Credential{}, errors.New("USER is not set, so the store cannot be addressed")
	}
	out, err := keychainCmd(ctx, account, r.Keychain).Output()
	if err != nil {
		return Credential{}, fmt.Errorf("read the platform credential store: %w", err)
	}
	c, err := r.decode(out)
	if err != nil {
		return Credential{}, fmt.Errorf("read the platform credential store: %w", err)
	}
	return c, nil
}
