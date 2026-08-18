package isolate

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
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
		File:         ".credentials.json",
		Fields: []string{
			"toolOauth.accessToken",
			"toolOauth.expiresAt",
			"toolOauth.scopes",
		},
		Expiry: "ms:toolOauth.expiresAt",
	}
}

// aCredentialPath is where this test's route keeps a credential.
func aCredentialPath(dir string) string { return aRoute().CredentialPath(dir) }

// aCredential is a usable credential, stated once so a test that is about
// something else does not have to.
func aCredential() Credential {
	expires := time.Now().Add(24 * time.Hour).UnixMilli()
	return Credential{
		Fields: map[string]json.RawMessage{
			"toolOauth.accessToken": json.RawMessage(`"sk-ant-oat-example"`),
			"toolOauth.expiresAt":   json.RawMessage(strconv.FormatInt(expires, 10)),
			"toolOauth.scopes":      json.RawMessage(`["user:inference","user:profile"]`),
		},
		ExpiresAt: expires,
	}
}

// theToken is the access token inside a credential, for a test that wants to
// look for it on disk.
func theToken(c Credential) string {
	var token string
	_ = json.Unmarshal(c.Fields["toolOauth.accessToken"], &token)
	return token
}

// without is this credential with one declared field taken out.
func without(c Credential, field string) Credential {
	out := Credential{Fields: map[string]json.RawMessage{}, ExpiresAt: c.ExpiresAt}
	for k, v := range c.Fields {
		if k != field {
			out.Fields[k] = v
		}
	}
	return out
}

func TestTheProvisionedFileCarriesOnlyTheDeclaredFields(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")

	if err := aRoute().Write(dir, aCredential()); err != nil {
		t.Fatalf("provision: %v", err)
	}

	b, err := os.ReadFile(aCredentialPath(dir))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var doc map[string]map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("the provisioned file is not JSON: %v", err)
	}
	oauth, ok := doc["toolOauth"]
	if !ok {
		t.Fatal("no toolOauth object; the declared paths nest under it and the tool reads it there")
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

	b, err := os.ReadFile(aCredentialPath(dir))
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

	info, err := os.Stat(aRoute().CredentialPath(env.Config))
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
	noExpiry := aCredential()
	noExpiry.ExpiresAt = 0
	for _, tc := range []struct {
		what string
		cred Credential
	}{
		{"nothing at all", Credential{}},
		{"no expiry", noExpiry},
		{"no fields", Credential{ExpiresAt: full.ExpiresAt}},
	} {
		t.Run(tc.what, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "config")

			if err := aRoute().Write(dir, tc.cred); err == nil {
				t.Fatal("a credential that cannot authenticate was provisioned; the run would burn a wall to find out")
			}
			if _, err := os.Stat(aCredentialPath(dir)); err == nil {
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

func TestReadingACredentialTakesTheDeclaredFieldsAndLeavesTheRest(t *testing.T) {
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
	if err := os.WriteFile(aCredentialPath(dir), []byte(host), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}

	c, err := aRoute().Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := theToken(c); got != "sk-ant-oat-host" {
		t.Errorf("accessToken = %q, want the host's", got)
	}
	if c.ExpiresAt != 1799999999000 {
		t.Errorf("expiresAt = %d, want the host's", c.ExpiresAt)
	}
	if got := string(c.Fields["toolOauth.scopes"]); got != `["user:inference"]` {
		t.Errorf("scopes = %s, want the host's", got)
	}

	// The proof that the narrowing happened is what a run would then be given.
	into := filepath.Join(t.TempDir(), "config")
	if err := aRoute().Write(into, c); err != nil {
		t.Fatalf("provision what was read: %v", err)
	}
	b, err := os.ReadFile(aCredentialPath(into))
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
				if err := os.WriteFile(aCredentialPath(dir), []byte(tc.contents), 0o600); err != nil {
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
		Fields:    map[string]json.RawMessage{"toolOauth.accessToken": json.RawMessage(`"t"`)},
		ExpiresAt: now.Add(30 * time.Minute).UnixMilli(),
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
		if readErr == nil && strings.Contains(string(b), theToken(c)) {
			carrying = append(carrying, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the run directory: %v", err)
	}
	if want := []string{aRoute().CredentialPath(env.Config)}; !slices.Equal(carrying, want) {
		t.Errorf("files carrying the token = %v, want only %v", carrying, want)
	}
}

func TestAKeyBasedRunIsGivenNoCredentialFileAtAll(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")

	env, err := Prepare(Spec{Root: root, Arm: Baseline, Route: aRoute()})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if _, err := os.Stat(aRoute().CredentialPath(env.Config)); err == nil {
		t.Error("a run that was given no credential was provisioned with a file anyway")
	}
}

func TestARunGivenAHalfFilledCredentialIsRefusedRatherThanRun(t *testing.T) {
	// Distinct from the empty case above, and the distinction is the point: an
	// absent credential is a key-based host, and a half-filled one is a mistake
	// that would cost a wall per arm to discover.
	root := filepath.Join(t.TempDir(), "run")

	half := without(aCredential(), "toolOauth.expiresAt")
	half.ExpiresAt = 0
	if _, err := Prepare(Spec{
		Root:       root,
		Arm:        Baseline,
		Credential: half,
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
	if _, err := os.Stat(r.CredentialPath(got[r.ConfigDirVar])); err != nil {
		t.Errorf("the directory the session is pointed at holds no credential: %v", err)
	}
	// Outside the disposable HOME, or the contamination proof reads a
	// provisioned credential as persisted memory and every arm looks dirty.
	if strings.HasPrefix(env.Config, env.Home+string(filepath.Separator)) {
		t.Errorf("the config directory %q is inside the disposable HOME %q", env.Config, env.Home)
	}
}

func TestAToolWithNoConfigVariableIsGivenNone(t *testing.T) {
	// A tool that takes no config-directory variable is given none. Inventing
	// one would set a variable in its session that means nothing, and setting
	// some other tool's would be worse. Codex was assumed to be such a tool and
	// measured on 2026-08-18 not to be: it takes CODEX_HOME. The rule is what is
	// under test here, not the roster.
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

// A tool that names no credential file has nowhere its credential could be read
// from or written to, so anything written for it is written where nothing reads
// it — and the run would spend a full wall per arm discovering it is logged out.
func TestARouteWithNoCredentialFileCannotProvisionOrRead(t *testing.T) {
	fileless := aRoute()
	fileless.File = ""
	dir := filepath.Join(t.TempDir(), "config")

	if err := fileless.Write(dir, aCredential()); err == nil {
		t.Error("a credential was written for a tool that names no file")
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		t.Errorf("a refused credential left %d files behind", len(entries))
	}
	if _, err := fileless.Read(t.TempDir()); err == nil {
		t.Error("a credential was read for a tool that names no file")
	}
}

// A tool whose declared fields are not all in the document is a tool whose
// login is half there, and a run given it reads as logged out.
func TestAHostDocumentMissingADeclaredFieldIsRefused(t *testing.T) {
	dir := t.TempDir()
	host := `{"toolOauth": {"accessToken": "t", "expiresAt": 1799999999000}}`
	if err := os.WriteFile(aCredentialPath(dir), []byte(host), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}

	if _, err := aRoute().Read(dir); err == nil {
		t.Fatal("a document missing a declared field was read as a usable credential")
	}
}

// The expiry is stated two ways by the two tools measured so far, and a route
// that could not read its own would be planned against a credential that dies
// mid-cell.
func TestTheExpiryIsReadTheWayTheToolStatesIt(t *testing.T) {
	for _, tc := range []struct {
		what   string
		expiry string
		doc    string
		want   int64
	}{
		{
			what:   "unix milliseconds at a path",
			expiry: "ms:toolOauth.expiresAt",
			doc:    `{"toolOauth":{"accessToken":"t","expiresAt":1799999999000,"scopes":["s"]}}`,
			want:   1799999999000,
		},
		{
			what:   "the exp claim of a token at a path",
			expiry: "jwt:toolOauth.accessToken",
			// exp 1799999999, with no signature to check because none is checked.
			doc:  `{"toolOauth":{"accessToken":"header.eyJleHAiOjE3OTk5OTk5OTl9.sig","expiresAt":1,"scopes":["s"]}}`,
			want: 1799999999000,
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			r := aRoute()
			r.Expiry = tc.expiry
			dir := t.TempDir()
			if err := os.WriteFile(r.CredentialPath(dir), []byte(tc.doc), 0o600); err != nil {
				t.Fatalf("set up: %v", err)
			}

			c, err := r.Read(dir)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if c.ExpiresAt != tc.want {
				t.Errorf("ExpiresAt = %d, want %d", c.ExpiresAt, tc.want)
			}
		})
	}
}

// An expiry that cannot be read is refused rather than defaulted to zero: a
// zero expiry reads as "already expired" in one place and "not stated" in
// another, and a planner cannot tell a dead credential from an unreadable one.
func TestAnUnreadableExpiryIsRefused(t *testing.T) {
	for _, tc := range []struct {
		what   string
		expiry string
		doc    string
	}{
		{"a form nobody implements", "seconds:toolOauth.expiresAt", `{"toolOauth":{"accessToken":"t","expiresAt":1,"scopes":["s"]}}`},
		{"milliseconds that are not a number", "ms:toolOauth.accessToken", `{"toolOauth":{"accessToken":"t","expiresAt":1,"scopes":["s"]}}`},
		{"a path that is not there", "ms:toolOauth.notThere", `{"toolOauth":{"accessToken":"t","expiresAt":1,"scopes":["s"]}}`},
		{"a token that is not a JWT", "jwt:toolOauth.accessToken", `{"toolOauth":{"accessToken":"t","expiresAt":1,"scopes":["s"]}}`},
		{"a JWT whose claims are not JSON", "jwt:toolOauth.accessToken", `{"toolOauth":{"accessToken":"a.bm90IGpzb24.c","expiresAt":1,"scopes":["s"]}}`},
		{"a JWT with no exp claim", "jwt:toolOauth.accessToken", `{"toolOauth":{"accessToken":"a.eyJzdWIiOiJ4In0.c","expiresAt":1,"scopes":["s"]}}`},
		{"a JWT whose claims are not base64", "jwt:toolOauth.accessToken", `{"toolOauth":{"accessToken":"a.!!!.c","expiresAt":1,"scopes":["s"]}}`},
	} {
		t.Run(tc.what, func(t *testing.T) {
			r := aRoute()
			r.Expiry = tc.expiry
			dir := t.TempDir()
			if err := os.WriteFile(r.CredentialPath(dir), []byte(tc.doc), 0o600); err != nil {
				t.Fatalf("set up: %v", err)
			}

			if _, err := r.Read(dir); err == nil {
				t.Fatal("a credential whose expiry cannot be read was read as usable")
			}
		})
	}
}

// A route that declares no fields would copy nothing, and a run given an empty
// document reads as logged out.
func TestARouteWithNoDeclaredFieldsCannotRead(t *testing.T) {
	fieldless := aRoute()
	fieldless.Fields = nil
	dir := t.TempDir()
	if err := os.WriteFile(fieldless.CredentialPath(dir),
		[]byte(`{"toolOauth":{"accessToken":"t","expiresAt":1799999999000,"scopes":["s"]}}`), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}

	if _, err := fieldless.Read(dir); err == nil {
		t.Fatal("a credential was read for a tool that declares no fields")
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

// A tool that keeps its login inside HOME and nowhere else cannot be
// provisioned with a file: HOME is the disposable one, and state inside it is
// exactly what the contamination proof reads as a dirty arm. Handed through the
// environment, the run holds the credential and the HOME stays empty.
func TestACredentialCanReachARunThroughTheEnvironmentInstead(t *testing.T) {
	r := aRoute()
	r.EnvVar = "TOOL_AUTH_CONTENT"
	dir := filepath.Join(t.TempDir(), "config")

	env, err := r.Provision(dir, aCredential())
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if len(env) != 1 || !strings.HasPrefix(env[0], "TOOL_AUTH_CONTENT=") {
		t.Fatalf("environment = %v, want the tool's own variable carrying the document", env)
	}
	if !strings.Contains(env[0], "sk-ant-oat-example") {
		t.Errorf("the document does not carry the token:\n%s", env[0])
	}
	// Nothing on disk, which is the whole point.
	if _, err := os.Stat(dir); err == nil {
		t.Error("a run given its credential through the environment was also written a config directory")
	}
}

func TestAToolThatTakesAFileIsGivenAFileAndNoEnvironment(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")

	env, err := aRoute().Provision(dir, aCredential())
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if len(env) != 0 {
		t.Errorf("environment = %v, want nothing: this tool reads a file", env)
	}
	if _, err := os.Stat(aCredentialPath(dir)); err != nil {
		t.Errorf("no credential file was written: %v", err)
	}
}

// A key-based host provisions nothing at all, and that is a different thing
// from a half-filled credential, which is a mistake.
func TestAKeyBasedHostProvisionsNothingByEitherRoute(t *testing.T) {
	for _, r := range []Route{aRoute(), func() Route { r := aRoute(); r.EnvVar = "TOOL_AUTH_CONTENT"; return r }()} {
		dir := filepath.Join(t.TempDir(), "config")

		env, err := r.Provision(dir, Credential{})
		if err != nil {
			t.Fatalf("provision: %v", err)
		}
		if len(env) != 0 {
			t.Errorf("environment = %v, want nothing", env)
		}
		if _, err := os.Stat(dir); err == nil {
			t.Error("a key-based run was provisioned anyway")
		}
	}
}

func TestAHalfFilledCredentialIsRefusedOnTheEnvironmentRouteToo(t *testing.T) {
	r := aRoute()
	r.EnvVar = "TOOL_AUTH_CONTENT"
	half := without(aCredential(), "toolOauth.accessToken")

	if _, err := r.Provision(filepath.Join(t.TempDir(), "config"), half); err == nil {
		t.Fatal("a credential missing a declared field was handed to a run")
	}
}

// An API key has no end to read, and a tool that says so is a different thing
// from a tool whose expiry nobody wired up: the second would have a cell
// planned against a credential it cannot check.
func TestACredentialThatDeclaresNoExpiryIsUsableAndNeverExpires(t *testing.T) {
	r := aRoute()
	r.Expiry = "never"
	r.Fields = []string{"provider.key", "provider.type"}
	dir := t.TempDir()
	if err := os.WriteFile(r.CredentialPath(dir),
		[]byte(`{"provider":{"key":"sk-key","type":"api"},"other":{"key":"not-this-one"}}`), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}

	c, err := r.Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !c.Valid() {
		t.Error("an API key reads as unusable because it has no expiry")
	}
	if c.Empty() {
		t.Error("an API key reads as no credential at all, which is what a key-based host looks like")
	}
	if c.ExpiresBefore(time.Now().Add(100 * 365 * 24 * time.Hour)) {
		t.Error("a credential that never expires was reported as expiring")
	}
	// The other provider's key is not this run's business.
	into := filepath.Join(t.TempDir(), "config")
	if err := r.Write(into, c); err != nil {
		t.Fatalf("provision: %v", err)
	}
	b, err := os.ReadFile(r.CredentialPath(into))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(b), "not-this-one") {
		t.Errorf("a provider this run does not use reached it:\n%s", b)
	}
}
