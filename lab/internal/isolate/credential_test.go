package isolate

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// aRoute is a credential route of the shape a catalog declares, stated once so a
// test that is about something else does not have to. The names are a test's
// own: this package compiles none of them in.
func aRoute() Route {
	return Route{
		ConfigDirVar: "TOOL_CONFIG_DIR",
		ConfigDir:    ".tool",
		Keychain:     "Tool-credentials",
		Key:          "toolOauth",
	}
}

// aCredential is a usable credential, stated once so a test that is about
// something else does not have to.
func aCredential() Credential {
	return Credential{
		AccessToken: "sk-ant-oat-example",
		ExpiresAt:   time.Now().Add(24 * time.Hour).UnixMilli(),
		Scopes:      []string{"user:inference", "user:profile"},
	}
}

func TestTheProvisionedFileCarriesOnlyTheThreeFieldsThatAuthenticate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")

	if err := aRoute().Write(dir, aCredential()); err != nil {
		t.Fatalf("provision: %v", err)
	}

	b, err := os.ReadFile(CredentialPath(dir))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var doc map[string]map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("the provisioned file is not JSON: %v", err)
	}
	oauth, ok := doc[aRoute().Key]
	if !ok {
		t.Fatalf("no %s object; the combiner reads that key and nothing else", aRoute().Key)
	}
	want := map[string]bool{"accessToken": true, "expiresAt": true, "scopes": true}
	for key := range oauth {
		if !want[key] {
			t.Errorf("the provisioned file carries %q, which a run does not need to reach a model", key)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("the provisioned file is missing %q; measured 2026-08-17, a run without it reads as logged out", key)
	}
}

func TestNoRefreshTokenOrConnectorTokenCanReachARun(t *testing.T) {
	// The safety property, asserted on the bytes rather than on the type: a run
	// that cannot refresh cannot rotate the operator's login, and a run with no
	// mcpOAuth holds no credentials for the operator's connectors. Both are
	// invisible in any bench result, so nothing else would catch a regression.
	dir := filepath.Join(t.TempDir(), "config")
	if err := aRoute().Write(dir, aCredential()); err != nil {
		t.Fatalf("provision: %v", err)
	}

	b, err := os.ReadFile(CredentialPath(dir))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	for _, forbidden := range []string{"refreshToken", "mcpOAuth"} {
		if strings.Contains(string(b), forbidden) {
			t.Errorf("%s reached a run's credential file:\n%s", forbidden, b)
		}
	}
}

func TestTheProvisionedCredentialIsNotReadableByAnyoneElse(t *testing.T) {
	// Asserted on what PREPARE builds, not on a bare Write into a fresh path.
	// The two differ, and the difference is the whole point: Prepare has already
	// created the config directory, and MkdirAll does not chmod one that exists,
	// so a Write into a directory made at 0755 leaves it at 0755. Testing Write
	// alone passes on a directory no run ever has.
	root := filepath.Join(t.TempDir(), "run")
	env, err := Prepare(Spec{Root: root, Arm: Baseline, Credential: aCredential(), Route: aRoute()})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	info, err := os.Stat(CredentialPath(env.Config))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credential file mode is %o, want 600", perm)
	}
	parent, err := os.Stat(env.Config)
	if err != nil {
		t.Fatalf("stat the config directory: %v", err)
	}
	// A 0600 file inside a world-listable directory still tells everyone on the
	// machine that the credential is there and what it is called.
	if perm := parent.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("the config directory is %o, which lets someone else list the credential beside it", perm)
	}
}

func TestAnUnusableCredentialIsRefusedRatherThanWritten(t *testing.T) {
	full := aCredential()
	for _, tc := range []struct {
		what string
		cred Credential
	}{
		{"no token at all", Credential{ExpiresAt: full.ExpiresAt, Scopes: full.Scopes}},
		{"no expiry", Credential{AccessToken: full.AccessToken, Scopes: full.Scopes}},
		// Scopes is the gate: measured 2026-08-17, a token with an expiry and no
		// scopes returns "Not logged in" for a full wall's worth of nothing.
		{"no scopes", Credential{AccessToken: full.AccessToken, ExpiresAt: full.ExpiresAt}},
	} {
		t.Run(tc.what, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "config")

			if err := aRoute().Write(dir, tc.cred); err == nil {
				t.Fatal("a credential that cannot authenticate was provisioned; the run would burn a wall to find out")
			}
			if _, err := os.Stat(CredentialPath(dir)); err == nil {
				t.Error("a refused credential was written anyway")
			}
		})
	}
}

func TestProvisioningReportsAConfigDirectoryItCannotCreate(t *testing.T) {
	// A file where the directory should be. The run must not proceed believing it
	// has a credential.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}

	if err := aRoute().Write(filepath.Join(blocked, "config"), aCredential()); err == nil {
		t.Fatal("provisioning reported success with nowhere to write")
	}
}

func TestReadingACredentialTakesTheThreeFieldsAndLeavesTheRest(t *testing.T) {
	// The host's own store, which holds the operator's connector tokens and the
	// refresh token beside the login. Reading it must not carry them along.
	dir := t.TempDir()
	host := `{
	  "toolOauth": {
	    "accessToken": "sk-ant-oat-host",
	    "refreshToken": "sk-ant-ort-host",
	    "expiresAt": 1799999999000,
	    "scopes": ["user:inference"],
	    "subscriptionType": "max"
	  },
	  "mcpOAuth": {"some-connector": {"accessToken": "connector-token"}}
	}`
	if err := os.WriteFile(CredentialPath(dir), []byte(host), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}

	c, err := aRoute().Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if c.AccessToken != "sk-ant-oat-host" {
		t.Errorf("accessToken = %q, want the host's", c.AccessToken)
	}
	if c.ExpiresAt != 1799999999000 {
		t.Errorf("expiresAt = %d, want the host's", c.ExpiresAt)
	}
	if len(c.Scopes) != 1 || c.Scopes[0] != "user:inference" {
		t.Errorf("scopes = %v, want the host's", c.Scopes)
	}

	// The proof that the narrowing happened is what a run would then be given.
	into := filepath.Join(t.TempDir(), "config")
	if err := aRoute().Write(into, c); err != nil {
		t.Fatalf("provision what was read: %v", err)
	}
	b, err := os.ReadFile(CredentialPath(into))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	for _, forbidden := range []string{"refreshToken", "mcpOAuth", "connector-token", "subscriptionType"} {
		if strings.Contains(string(b), forbidden) {
			t.Errorf("%s survived the round trip from the host store into a run", forbidden)
		}
	}
}

func TestReadingReportsAStoreItCannotUse(t *testing.T) {
	for _, tc := range []struct {
		what     string
		contents string
		write    bool
	}{
		{what: "no file at all"},
		{what: "not JSON", contents: "logged out", write: true},
		{what: "some other document", contents: `{"somethingElse": {}}`, write: true},
	} {
		t.Run(tc.what, func(t *testing.T) {
			dir := t.TempDir()
			if tc.write {
				if err := os.WriteFile(CredentialPath(dir), []byte(tc.contents), 0o600); err != nil {
					t.Fatalf("set up: %v", err)
				}
			}

			if _, err := aRoute().Read(dir); err == nil {
				t.Fatal("a store that carries no credential was read as one")
			}
		})
	}
}

func TestACredentialThatExpiresBeforeTheCellEndsIsKnownToExpire(t *testing.T) {
	// The question a preflight has to ask. Asking whether it is valid NOW passes a
	// credential that dies between the sense arm and the baseline, which burns the
	// finished arm: it can never be paired.
	now := time.Now()
	c := Credential{
		AccessToken: "t",
		Scopes:      []string{"user:inference"},
		ExpiresAt:   now.Add(30 * time.Minute).UnixMilli(),
	}

	if c.ExpiresBefore(now) {
		t.Error("a credential good for another half hour was reported as already expired")
	}
	if !c.ExpiresBefore(now.Add(time.Hour)) {
		t.Error("a credential that dies in half an hour was reported as good for an hour")
	}
	if got := c.Expiry(); got.Sub(now).Round(time.Minute) != 30*time.Minute {
		t.Errorf("Expiry() = %v, which cannot name the expiry in a refusal", got)
	}
}

func TestTheCredentialIsWrittenOnlyIntoTheRunsOwnConfigDirectory(t *testing.T) {
	// A run directory is read back for months, so a token anywhere in it that is
	// not the one file cleanup knows about is a token that outlives its purpose.
	root := filepath.Join(t.TempDir(), "run")
	c := aCredential()

	env, err := Prepare(Spec{Root: root, Arm: Baseline, HostPath: "", Credential: c, Route: aRoute()})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	var carrying []string
	err = filepath.WalkDir(env.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil //nolint:nilerr // an unreadable entry is not a finding
		}
		b, readErr := os.ReadFile(path) // #nosec G304 -- the test's own run directory
		if readErr == nil && strings.Contains(string(b), c.AccessToken) {
			carrying = append(carrying, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the run directory: %v", err)
	}
	if want := []string{CredentialPath(env.Config)}; !slices.Equal(carrying, want) {
		t.Errorf("files carrying the token = %v, want only %v", carrying, want)
	}
}

func TestAKeyBasedRunIsGivenNoCredentialFileAtAll(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")

	env, err := Prepare(Spec{Root: root, Arm: Baseline, Route: aRoute()})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if _, err := os.Stat(CredentialPath(env.Config)); err == nil {
		t.Error("a run that was given no credential was provisioned with a file anyway")
	}
}

func TestARunGivenAHalfFilledCredentialIsRefusedRatherThanRun(t *testing.T) {
	// Distinct from the empty case above, and the distinction is the point: an
	// absent credential is a key-based host, and a half-filled one is a mistake
	// that would cost a wall per arm to discover.
	root := filepath.Join(t.TempDir(), "run")

	if _, err := Prepare(Spec{
		Root:       root,
		Arm:        Baseline,
		Credential: Credential{AccessToken: "t"},
		Route:      aRoute(),
	}); err == nil {
		t.Fatal("an environment was prepared around a credential that cannot authenticate")
	}
}

func TestTheSessionIsPointedAtTheConfigDirectoryTheCredentialIsIn(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")
	r := aRoute()

	env, err := Prepare(Spec{Root: root, Arm: Baseline, Credential: aCredential(), Route: r})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	got := envMap(t, env.Environ)
	if got[r.ConfigDirVar] != env.Config {
		t.Errorf("%s = %q, want the run's own config directory %q", r.ConfigDirVar, got[r.ConfigDirVar], env.Config)
	}
	if _, err := os.Stat(CredentialPath(got[r.ConfigDirVar])); err != nil {
		t.Errorf("the directory the session is pointed at holds no credential: %v", err)
	}
	// Outside the disposable HOME, or the contamination proof reads a
	// provisioned credential as persisted memory and every arm looks dirty.
	if strings.HasPrefix(env.Config, env.Home+string(filepath.Separator)) {
		t.Errorf("the config directory %q is inside the disposable HOME %q", env.Config, env.Home)
	}
}

func TestAToolWithNoConfigVariableIsGivenNone(t *testing.T) {
	// codex and opencode take no config-directory variable. Inventing one would
	// set a variable in their session that means nothing, and setting some other
	// tool's would be worse.
	root := filepath.Join(t.TempDir(), "run")
	r := aRoute()
	r.ConfigDirVar = ""

	env, err := Prepare(Spec{Root: root, Arm: Baseline, Credential: aCredential(), Route: r})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	for _, kv := range env.Environ {
		if strings.Contains(kv, "CONFIG_DIR=") {
			t.Errorf("a config-directory variable was invented: %q", kv)
		}
	}
}

// A tool that names no credential key has nowhere its credential could be read
// from, so a file written for it is a file nothing reads — and the run would
// spend a full wall per arm discovering that it is logged out.
func TestARouteWithNoCredentialKeyCannotProvisionOrRead(t *testing.T) {
	keyless := aRoute()
	keyless.Key = ""
	dir := filepath.Join(t.TempDir(), "config")

	if err := keyless.Write(dir, aCredential()); err == nil {
		t.Error("a credential was written under no key at all")
	}
	if _, err := os.Stat(CredentialPath(dir)); err == nil {
		t.Error("a refused credential was written anyway")
	}

	// And the same on the way in. The document deliberately HOLDS a usable
	// credential under the empty key, so this cannot pass by the decode failing
	// for some other reason: without the guard the read succeeds and hands back a
	// credential from a key nothing writes under.
	host := t.TempDir()
	stored := `{"": {"accessToken":"t","expiresAt":1799999999000,"scopes":["user:inference"]}}`
	if err := os.WriteFile(CredentialPath(host), []byte(stored), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}
	if _, err := keyless.Read(host); err == nil {
		t.Error("a credential was read from a store with no key to read it under")
	}
}

// The write can fail for reasons that have nothing to do with the credential —
// a full disk, a read-only mount — and a run must not proceed believing it has
// one. The failure has to arrive here, in the attended parent, rather than as an
// arm that exits in a second reading as a model with nothing to say.
func TestAConfigDirectoryThatCannotBeWrittenIntoIsReported(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only directory does not stop writing here")
	}
	dir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := aRoute().Write(dir, aCredential()); err == nil {
		t.Fatal("provisioning reported success although the credential could not be written")
	}
}
