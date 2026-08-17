package isolate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Credential is the whole of what a run is given to reach a model.
//
// Three fields, and the shape is measured rather than assumed. Eight shapes were
// written into an isolated run's config directory on 2026-08-17 and asked for one
// word: `accessToken` alone does not authenticate, `accessToken` with an expiry
// does not, and `accessToken` with an expiry and `subscriptionType` does not.
// Scopes is the gate. The smallest thing that authenticates is these three.
//
// What is deliberately NOT here is the point of the type. The host store also
// holds a refresh token and the operator's connector tokens, and neither reaches
// a model. A run that cannot refresh cannot rotate the operator's login, so no
// number of unattended cells can invalidate the host seat by succeeding; and a
// run with no connector tokens holds no credentials for services that have
// nothing to do with this measurement. Both are safety properties, not tidiness.
type Credential struct {
	AccessToken string   `json:"accessToken"`
	ExpiresAt   int64    `json:"expiresAt"`
	Scopes      []string `json:"scopes"`
}

// Route is how a credential reaches one agent tool, as the catalog describes it.
//
// Every field is a per-tool identifier, and not one of them is compiled in. A
// config directory name, a variable, a keychain item or a JSON key written into
// this package would be right for one tool and silently wrong for every other —
// the same rule the contamination checks already follow, and the reason the
// binary-identifier probe exists.
type Route struct {
	// ConfigDirVar is the variable that points the tool at a config directory.
	// It is the door: a per-run value is what makes authentication a channel
	// with a stated direction rather than an accident of what HOME reaches.
	ConfigDirVar string
	// ConfigDir is the tool's config directory name relative to HOME, used to
	// find the OPERATOR's own credential when ConfigDirVar is unset on the host.
	ConfigDir string
	// Keychain is the platform store item the tool keeps its login under, read
	// once by the attended parent. Empty means the tool has no platform store.
	Keychain string
	// Key is the object the credential lives under inside the store document.
	Key string
}

// credentialFile is where the agent tool looks inside its config directory.
//
// Undocumented on macOS and load-bearing here: the published docs name this file
// on Linux and Windows only, and the shipped binary contradicts them. Its
// credential store is a two-backend combiner that reads the keychain first and
// falls back to this path, with the directory honouring the tool's config
// variable. Read from the shipped binary at 2.1.233; re-read when the binary
// moves, because no doc page will announce a change here.
//
// The NAME is the one credential fact that is not per tool: it is the dotfile
// convention rather than a tool identifier, and it carries no tool's name.
const credentialFile = ".credentials.json"

// Empty reports a credential that was never provided.
//
// That is what a key-based host hands a run: the key is an allowlisted
// environment variable, the tool reads it from there, and no file is written at
// all. It is a different thing from an INVALID credential, which is a mistake
// and is refused rather than skipped.
func (c Credential) Empty() bool {
	return c.AccessToken == "" && c.ExpiresAt == 0 && len(c.Scopes) == 0
}

// Valid reports whether a credential could be used at all. An empty token or an
// empty scope list is a credential that will read as logged out, which costs a
// full wall per arm to discover.
func (c Credential) Valid() bool {
	return c.AccessToken != "" && c.ExpiresAt > 0 && len(c.Scopes) > 0
}

// ExpiresBefore reports whether the credential runs out before a moment a caller
// has to reach.
//
// A cell asks this about its own end rather than about now: a token that outlives
// the sense arm and dies during the baseline produces a burned arm and a cell
// that can never be paired, which is the half-pair hazard arriving through the
// credential instead of through an interrupt.
func (c Credential) ExpiresBefore(t time.Time) bool {
	return time.UnixMilli(c.ExpiresAt).Before(t)
}

// Expiry is when the credential runs out, for a message that can name it.
func (c Credential) Expiry() time.Time { return time.UnixMilli(c.ExpiresAt) }

// CredentialPath is where a run's credential lives, given its config directory.
func CredentialPath(configDir string) string { return filepath.Join(configDir, credentialFile) }

// Write provisions one run's credential into its config directory.
//
// It is a pure function of its arguments and a path: no keychain, no login, no
// host. That is what lets the whole of this decision be tested in CI, which has
// no credential and can never be given one.
//
// It builds the document field by field rather than marshalling something read
// from the host, so a field the host store gains later cannot arrive here by
// default. The only way another key reaches a run is for someone to add it to
// Credential and say why.
func (r Route) Write(configDir string, c Credential) error {
	if r.Key == "" {
		return fmt.Errorf("provision credential into %s: the agent tool names no credential key, "+
			"so the document would be written where nothing reads it", configDir)
	}
	if !c.Valid() {
		return fmt.Errorf("provision credential into %s: the credential carries no token, no expiry or no scopes, "+
			"and a run given one reads as logged out", configDir)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("prepare the run's config directory: %w", err)
	}
	// A struct of a string, an int and a string slice cannot fail to encode.
	//
	// Marshalling a bearer token is the whole job here, not an accident: the
	// agent tool reads its credential from this file and there is no other route
	// into an isolated run. What keeps it safe is the mode below, the fact that
	// the document holds three fields and no refresh token, and that cleanup
	// proves the file gone.
	b, _ := json.MarshalIndent(map[string]Credential{r.Key: c}, "", "  ") // #nosec G117 -- provisioning a credential is this function

	// 0600 because the file is a plaintext bearer token for as long as the run
	// lasts. WriteFile does not chmod a file that already exists, and Prepare
	// refuses a root that does, so there is no pre-existing file to inherit a
	// mode from.
	if err := os.WriteFile(CredentialPath(configDir), append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("provision credential into %s: %w", configDir, err)
	}
	return nil
}

// Read reads a credential out of a config directory, host or run.
//
// It is the reverse of Write and takes the same narrow view: whatever else the
// document holds is dropped rather than carried, so reading the operator's own
// config directory yields the three fields and not the connector tokens beside
// them.
func (r Route) Read(configDir string) (Credential, error) {
	path := CredentialPath(configDir)
	b, err := os.ReadFile(path) // #nosec G304 -- a config directory the caller named
	if err != nil {
		return Credential{}, fmt.Errorf("read credential from %s: %w", path, err)
	}
	c, err := r.decode(b)
	if err != nil {
		return Credential{}, fmt.Errorf("read credential from %s: %w", path, err)
	}
	return c, nil
}

// decode pulls the three fields worth carrying out of a store document.
//
// It is shared by the file and the platform store because both hold the same
// document: the combiner writes one shape and reads it from either backend.
func (r Route) decode(b []byte) (Credential, error) {
	if r.Key == "" {
		return Credential{}, fmt.Errorf("the agent tool names no credential key")
	}
	var doc map[string]Credential
	if err := json.Unmarshal(b, &doc); err != nil {
		return Credential{}, err
	}
	c, ok := doc[r.Key]
	if !ok {
		return Credential{}, fmt.Errorf("no %s object", r.Key)
	}
	return c, nil
}
