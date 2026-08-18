package isolate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credential is the whole of what a run is given to reach a model: the
// allowlisted fields of the operator's own credential document, and when it
// runs out.
//
// What is deliberately NOT here is the point of the type. A host document also
// holds the operator's connector tokens and, for some tools, a refresh token,
// and a run given the whole document would hold both. Which fields a tool
// actually needs is MEASURED rather than reasoned about, per tool, and the
// measurement is recorded in that tool's README: Claude Code needs an access
// token, an expiry and its scopes and does not need the refresh token beside
// them, so a Claude run cannot rotate the operator's login. Codex was measured
// on 2026-08-18 and does not have that property, and its README says so.
//
// The fields are carried as raw JSON rather than decoded: this package copies a
// credential, it does not interpret one, and a decoded map invites a type
// assertion on somebody else's document.
type Credential struct {
	// Fields is the allowlisted subset, keyed by the dotted path it came from
	// and goes back to.
	Fields map[string]json.RawMessage
	// ExpiresAt is when the credential runs out, in unix milliseconds.
	ExpiresAt int64
}

// Route is how a credential reaches one agent tool, as the catalog describes it.
//
// Every field is a per-tool identifier, and not one of them is compiled in. A
// config directory name, a variable, a keychain item, a file name or a JSON
// path written into this package would be right for one tool and silently wrong
// for every other — the same rule the contamination checks already follow, and
// the reason the binary-identifier probe exists.
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
	// File is the credential document's name inside the config directory.
	File string
	// Fields are the dotted paths a run is given, and nothing outside this list
	// ever reaches one.
	Fields []string
	// Expiry is where the end is read from, as `ms:<path>` for unix
	// milliseconds or `jwt:<path>` for the `exp` claim of a token at that path.
	Expiry string
}

// The two ways a credential document states its own end.
const (
	expiryMillis = "ms:"
	expiryJWT    = "jwt:"
)

// Empty reports a credential that was never provided.
//
// That is what a key-based host hands a run: the key is an allowlisted
// environment variable, the tool reads it from there, and no file is written at
// all. It is a different thing from an INVALID credential, which is a mistake
// and is refused rather than skipped.
func (c Credential) Empty() bool { return len(c.Fields) == 0 && c.ExpiresAt == 0 }

// Valid reports whether a credential could be used at all. A document with no
// fields, or with no readable expiry, will read as logged out or be planned
// against blind, and either costs a full wall per arm to discover.
func (c Credential) Valid() bool { return len(c.Fields) > 0 && c.ExpiresAt > 0 }

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
func (r Route) CredentialPath(configDir string) string { return filepath.Join(configDir, r.File) }

// Write provisions one run's credential into its config directory.
//
// It is a pure function of its arguments and a path: no keychain, no login, no
// host. That is what lets the whole of this decision be tested in CI, which has
// no credential and can never be given one.
//
// It rebuilds the document from the allowlisted paths rather than copying
// something read from the host, so a field the host store gains later cannot
// arrive here by default. The only way another key reaches a run is for someone
// to add it to the tool's declared list and say why.
func (r Route) Write(configDir string, c Credential) error {
	if r.File == "" {
		return fmt.Errorf("provision credential into %s: the agent tool names no credential file, "+
			"so the document would be written where nothing reads it", configDir)
	}
	if !c.Valid() {
		return fmt.Errorf("provision credential into %s: the credential carries no fields or no expiry, "+
			"and a run given one reads as logged out", configDir)
	}
	// The route's declared paths, not whatever the credential happens to carry.
	// A credential read through one tool's route and written through another's
	// would otherwise be written under the first tool's names, into a file the
	// second reads and finds nothing in — an arm that spends a full wall
	// reading as logged out.
	doc := map[string]any{}
	for _, path := range r.Fields {
		value, ok := c.Fields[path]
		if !ok {
			return fmt.Errorf("provision credential into %s: the credential carries no %s, "+
				"and a run given one reads as logged out", configDir, path)
		}
		put(doc, strings.Split(path, "."), value)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("prepare the run's config directory: %w", err)
	}
	// Marshalling a bearer token is the whole job here, not an accident: the
	// agent tool reads its credential from this file and there is no other route
	// into an isolated run. What keeps it safe is the mode below, the fact that
	// the document holds only what the tool was measured to need, and that
	// cleanup proves the file gone.
	b, err := json.MarshalIndent(doc, "", "  ") // #nosec G117 -- provisioning a credential is this function
	if err != nil {
		return fmt.Errorf("provision credential into %s: %w", configDir, err)
	}
	// 0600 because the file is a plaintext bearer token for as long as the run
	// lasts. WriteFile does not chmod a file that already exists, and Prepare
	// refuses a root that does, so there is no pre-existing file to inherit a
	// mode from.
	if err := os.WriteFile(r.CredentialPath(configDir), append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("provision credential into %s: %w", configDir, err)
	}
	return nil
}

// Read reads a credential out of a config directory, host or run.
//
// It is the reverse of Write and takes the same narrow view: whatever else the
// document holds is dropped rather than carried, so reading the operator's own
// config directory yields the declared fields and not the connector tokens
// beside them.
func (r Route) Read(configDir string) (Credential, error) {
	path := r.CredentialPath(configDir)
	if r.File == "" {
		return Credential{}, fmt.Errorf("read credential from %s: the agent tool names no credential file", configDir)
	}
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

// decode pulls the declared fields, and the expiry, out of a store document.
//
// It is shared by the file and the platform store because both hold the same
// document: a tool with two backends writes one shape and reads it from either.
func (r Route) decode(b []byte) (Credential, error) {
	if len(r.Fields) == 0 {
		return Credential{}, fmt.Errorf("the agent tool names no credential fields")
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return Credential{}, err
	}
	c := Credential{Fields: map[string]json.RawMessage{}}
	for _, path := range r.Fields {
		v, ok := at(doc, strings.Split(path, "."))
		if !ok {
			return Credential{}, fmt.Errorf("no %s in the document", path)
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return Credential{}, fmt.Errorf("re-encode %s: %w", path, err)
		}
		c.Fields[path] = raw
	}
	ms, err := r.expiresAt(doc)
	if err != nil {
		return Credential{}, err
	}
	c.ExpiresAt = ms
	return c, nil
}

// expiresAt reads the credential's end out of the document, the way this tool
// states it.
func (r Route) expiresAt(doc map[string]any) (int64, error) {
	switch {
	case strings.HasPrefix(r.Expiry, expiryMillis):
		v, ok := at(doc, strings.Split(strings.TrimPrefix(r.Expiry, expiryMillis), "."))
		ms, isNumber := v.(float64)
		if !ok || !isNumber {
			return 0, fmt.Errorf("no unix-millisecond expiry at %s", r.Expiry)
		}
		return int64(ms), nil
	case strings.HasPrefix(r.Expiry, expiryJWT):
		v, ok := at(doc, strings.Split(strings.TrimPrefix(r.Expiry, expiryJWT), "."))
		token, isString := v.(string)
		if !ok || !isString {
			return 0, fmt.Errorf("no token at %s to read an expiry from", r.Expiry)
		}
		return jwtExpiry(token)
	default:
		return 0, fmt.Errorf("the agent tool states its expiry as %q, which is neither %s… nor %s…",
			r.Expiry, expiryMillis, expiryJWT)
	}
}

// jwtExpiry reads the `exp` claim, in seconds, and returns it in milliseconds.
//
// The signature is not checked, and that is deliberate rather than lax: this
// reads the operator's own token to decide whether a cell can finish before it
// dies. A forged token here would be a token the operator forged for themselves,
// and the model at the other end is what actually verifies it.
func jwtExpiry(token string) (int64, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("the token is not a JWT, so its expiry cannot be read")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("read the token's claims: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("read the token's claims: %w", err)
	}
	if claims.Exp == 0 {
		return 0, fmt.Errorf("the token carries no exp claim, so nothing knows when it dies")
	}
	return claims.Exp * 1000, nil
}

// at walks a dotted path into a decoded document.
func at(doc map[string]any, path []string) (any, bool) {
	var cur any = doc
	for _, step := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[step]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// put writes a value at a dotted path, creating the objects on the way.
func put(doc map[string]any, path []string, value json.RawMessage) {
	cur := doc
	for _, step := range path[:len(path)-1] {
		next, ok := cur[step].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[step] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
}
