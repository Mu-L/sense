package isolate

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// storeDocument is what a credential store holds: the login the tool reads, and
// the operator's connector tokens and refresh token beside it.
func storeDocument(t *testing.T, token string, c Credential) string {
	t.Helper()
	doc := map[string]any{
		"toolOauth": map[string]any{
			"accessToken":  token,
			"refreshToken": "sk-refresh-host",
			"expiresAt":    c.ExpiresAt,
			"scopes":       []string{"user:inference"},
		},
		"mcpOAuth": map[string]any{"a-connector": map[string]string{"accessToken": "connector-token"}},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("build the store document: %v", err)
	}
	return string(b)
}

// answeringKeychain makes the store return the given text, and restores the real
// read afterwards.
func answeringKeychain(t *testing.T, out string, fail bool) {
	t.Helper()
	original := keychainCmd
	t.Cleanup(func() { keychainCmd = original })
	keychainCmd = func(ctx context.Context, _, _ string) *exec.Cmd {
		if fail {
			return exec.CommandContext(ctx, "false")
		}
		// printf rather than echo: the payload is JSON and must arrive
		// byte for byte.
		return exec.CommandContext(ctx, "printf", "%s", out)
	}
}

func TestTheHostConfigDirectoryIsWhereTheToolsOwnVariableSaysItIs(t *testing.T) {
	r := aRoute()
	host := hostWith(map[string]string{"HOME": "/home/op", r.ConfigDirVar: "/elsewhere/config"})

	got, err := r.HostConfigDir(host)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if got != "/elsewhere/config" {
		t.Errorf("host config dir = %q, want the value the host's own variable names", got)
	}
}

func TestTheHostConfigDirectoryFallsBackToTheToolsDefaultUnderHome(t *testing.T) {
	r := aRoute()
	got, err := r.HostConfigDir(hostWith(map[string]string{"HOME": "/home/op"}))
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	if want := filepath.Join("/home/op", r.ConfigDir); got != want {
		t.Errorf("host config dir = %q, want %q", got, want)
	}
}

func TestAHostWhoseConfigDirectoryCannotBeLocatedIsReportedRatherThanGuessed(t *testing.T) {
	for _, tc := range []struct {
		what  string
		route Route
		host  map[string]string
	}{
		{"the tool declares no config directory", Route{File: ".credentials.json"}, map[string]string{"HOME": "/home/op"}},
		{"nothing says where HOME is", aRoute(), nil},
	} {
		t.Run(tc.what, func(t *testing.T) {
			if _, err := tc.route.HostConfigDir(hostWith(tc.host)); err == nil {
				t.Fatal("a config directory was invented; a run would then be provisioned from nowhere")
			}
		})
	}
}

func TestTheOperatorsFileIsPreferredOverThePlatformStore(t *testing.T) {
	// The order the agent tool's own sandbox provisioner uses, and it is the half
	// that works on a machine with no keychain at all.
	r := aRoute()
	dir := t.TempDir()
	if err := os.WriteFile(r.CredentialPath(dir),
		[]byte(storeDocument(t, "from-the-file", aCredential())), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}
	answeringKeychain(t, storeDocument(t, "from-the-store", aCredential()), false)

	got, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: dir, "USER": "op",
	}), r)
	if err != nil {
		t.Fatalf("read the host credential: %v", err)
	}
	if token := theToken(got); token != "from-the-file" {
		t.Errorf("accessToken = %q, want the file's", token)
	}
}

func TestThePlatformStoreIsReadWhenThereIsNoFile(t *testing.T) {
	r := aRoute()
	answeringKeychain(t, storeDocument(t, "from-the-store", aCredential()), false)

	got, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: t.TempDir(), "USER": "op",
	}), r)
	if err != nil {
		t.Fatalf("read the host credential: %v", err)
	}
	if token := theToken(got); token != "from-the-store" {
		t.Errorf("accessToken = %q, want the store's", token)
	}
	// The narrowing has to happen on this path too, not only on the file one.
	if _, ok := got.Fields["toolOauth.scopes"]; !ok {
		t.Error("scopes were dropped, and a run without them reads as logged out")
	}
}

func TestAHostWithNoCredentialAnywhereIsReportedRatherThanReturnedEmpty(t *testing.T) {
	// The measured failure this pitch exists for: a session handed nothing exits
	// in about a second with "Not logged in", zero tokens, and the arm reads in a
	// score exactly like a model that had nothing to say. It has to be an error
	// here, in the attended parent, before anything spawns.
	r := aRoute()
	answeringKeychain(t, "", true)

	_, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: t.TempDir(), "USER": "op",
	}), r)
	if err == nil {
		t.Fatal("a host with no credential read as authenticated")
	}
	if !strings.Contains(err.Error(), "credential") {
		t.Errorf("error = %q, want it to name what is missing", err)
	}
}

func TestAToolWithNoPlatformStoreIsNotAskedForOne(t *testing.T) {
	// Codex keeps no keychain item: measured 2026-08-18, its login is a file in
	// CODEX_HOME and nowhere else. Shelling out for a store would fail with a
	// message about something that tool does not have.
	r := aRoute()
	r.Keychain = ""
	answeringKeychain(t, storeDocument(t, "from-the-store", aCredential()), false)

	_, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: t.TempDir(), "USER": "op",
	}), r)
	if err == nil {
		t.Fatal("a credential arrived from a store the tool declares it does not have")
	}
}

func TestAStoreThatCannotBeAddressedIsReported(t *testing.T) {
	r := aRoute()
	answeringKeychain(t, storeDocument(t, "from-the-store", aCredential()), false)

	// No USER, so there is no account to ask the store about.
	if _, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: t.TempDir(),
	}), r); err == nil {
		t.Fatal("the store was read without an account to read it for")
	}
}

func TestAStoreThatAnswersSomethingElseIsNotTakenAsACredential(t *testing.T) {
	for _, tc := range []struct{ what, answer string }{
		{"not JSON at all", "The specified item could not be found in the keychain."},
		{"a document with no credential in it", `{"somethingElse": {}}`},
		{"a credential with no scopes", `{"toolOauth": {"accessToken": "t", "expiresAt": 1799999999000}}`},
	} {
		t.Run(tc.what, func(t *testing.T) {
			r := aRoute()
			answeringKeychain(t, tc.answer, false)

			if _, err := HostCredential(context.Background(), hostWith(map[string]string{
				r.ConfigDirVar: t.TempDir(), "USER": "op",
			}), r); err == nil {
				t.Fatal("a store answer that is not a usable credential was accepted as one")
			}
		})
	}
}

func TestAnUnusableFileFallsThroughToTheStoreRatherThanFailing(t *testing.T) {
	// A stale or half-written file on the host must not stop a machine whose real
	// login is in the platform store.
	r := aRoute()
	dir := t.TempDir()
	if err := os.WriteFile(r.CredentialPath(dir), []byte(`{"toolOauth": {"accessToken": "t"}}`), 0o600); err != nil {
		t.Fatalf("set up: %v", err)
	}
	answeringKeychain(t, storeDocument(t, "from-the-store", aCredential()), false)

	got, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: dir, "USER": "op",
	}), r)
	if err != nil {
		t.Fatalf("read the host credential: %v", err)
	}
	if token := theToken(got); token != "from-the-store" {
		t.Errorf("accessToken = %q, want the store's", token)
	}
}

// A route that names no config directory at all cannot say where the
// operator's credential is, and inventing a path would provision a run from
// nowhere. The failure has to arrive here, in the attended parent.
func TestAToolWhoseConfigDirectoryCannotBeFoundYieldsNoCredential(t *testing.T) {
	r := aRoute()
	r.ConfigDir, r.ConfigDirVar = "", ""

	if _, err := HostCredential(context.Background(), hostWith(map[string]string{"HOME": "/home/op"}), r); err == nil {
		t.Fatal("a credential arrived from a tool that declares nowhere to keep one")
	}
}

// The store can answer a document that decodes and still cannot be used: an
// expiry of zero reads as a credential that died in 1970, and a run given one
// spends a full wall discovering it is logged out.
func TestAStoreAnswerThatDecodesButCannotBeUsedIsRefused(t *testing.T) {
	r := aRoute()
	answeringKeychain(t, `{"toolOauth":{"accessToken":"t","expiresAt":0,"scopes":["s"]}}`, false)

	if _, err := HostCredential(context.Background(), hostWith(map[string]string{
		r.ConfigDirVar: t.TempDir(), "USER": "op",
	}), r); err == nil {
		t.Fatal("a credential with no usable expiry was accepted from the store")
	}
}
