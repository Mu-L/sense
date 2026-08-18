package catalog

import "testing"

// The credential route is all-or-nothing. A half-declared one cannot carry a
// credential, so the run falls back to key authentication and produces an arm
// that reads as a model with nothing to say — which is the failure the route
// exists to end, re-entering through the catalog.
func TestAHalfDeclaredCredentialRouteIsRefused(t *testing.T) {
	full := Agent{
		ID: "tool", Binary: "b", ModelFlag: "-m", ConfigDirs: []string{".tool"},
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"subscription"},
		ConfigDirVar: "TOOL_CONFIG_DIR", KeychainService: "Tool-credentials",
		CredentialFile: ".credentials.json", CredentialExpiry: "ms:toolOauth.expiresAt",
		CredentialFields: []string{"toolOauth.accessToken"},
	}
	for _, tc := range []struct {
		what  string
		drop  func(*Agent)
		wants bool
	}{
		{"the whole route declared", func(*Agent) {}, false},
		// A tool that takes no config directory authenticates by key, and
		// declaring none of the four is how it says so.
		{"no route at all", func(a *Agent) {
			a.ConfigDirVar, a.CredentialFile, a.CredentialExpiry, a.CredentialFields = "", "", "", nil
		}, false},
		// A platform store is a SECOND source only some tools have. Codex keeps
		// its login in a file and nowhere else, so requiring one here would
		// refuse a route that works.
		{"no platform store, which only some tools have", func(a *Agent) { a.KeychainService = "" }, false},
		{"no variable to point the run at its config", func(a *Agent) { a.ConfigDirVar = "" }, true},
		{"no file the credential lives in", func(a *Agent) { a.CredentialFile = "" }, true},
		{"no fields that may be copied", func(a *Agent) { a.CredentialFields = nil }, true},
		{"no way to read when it expires", func(a *Agent) { a.CredentialExpiry = "" }, true},
	} {
		t.Run(tc.what, func(t *testing.T) {
			a := full
			tc.drop(&a)

			problems := checkCredentialRoute(a)

			if tc.wants && len(problems) == 0 {
				t.Fatal("a half-declared credential route was accepted")
			}
			if !tc.wants && len(problems) != 0 {
				t.Fatalf("a legitimate route was refused: %v", problems)
			}
		})
	}
}

// A tool that takes its credential through the environment has no config
// directory to be pointed at and no file to be given, so the file-shaped route
// check would refuse it. What it still needs is a way to READ the operator's
// own login before handing it over.
func TestAnEnvironmentCredentialRouteIsCheckedOnItsOwnTerms(t *testing.T) {
	full := Agent{
		ID: "tool", Binary: "b", SetupTool: "tool-cli", TranscriptFormat: "assistant-events",
		ModelFlag: "-m", ConfigDirs: []string{".tool"},
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"},
		CredentialEnv: "TOOL_AUTH_CONTENT", CredentialFile: "auth.json",
		CredentialFields: []string{"provider.key"}, CredentialExpiry: "never",
	}
	for _, tc := range []struct {
		what  string
		drop  func(*Agent)
		wants bool
	}{
		{"the whole route declared, and no config variable in sight", func(*Agent) {}, false},
		{"nothing to read the operator's login from", func(a *Agent) { a.CredentialFile = "" }, true},
		{"no fields that may be copied", func(a *Agent) { a.CredentialFields = nil }, true},
		{"no way to know whether it expires", func(a *Agent) { a.CredentialExpiry = "" }, true},
	} {
		t.Run(tc.what, func(t *testing.T) {
			a := full
			tc.drop(&a)

			problems := checkAgent(a)

			if tc.wants && len(problems) == 0 {
				t.Fatal("a half-declared environment credential route was accepted")
			}
			if !tc.wants && len(problems) != 0 {
				t.Fatalf("a legitimate route was refused: %v", problems)
			}
		})
	}
}

// The capture shim rewrites the registration in place, so a shape it cannot act
// on is a sense arm running uncaptured. Catching it here costs nothing;
// catching it at run time costs the run.
func TestAnMCPRegistrationThatCannotBeRewrittenIsRefused(t *testing.T) {
	base := Agent{
		ID: "tool", Binary: "b", SetupTool: "tool-cli", TranscriptFormat: "assistant-events",
		ModelFlag: "-m", ConfigDirs: []string{".tool"},
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"},
	}
	for _, tc := range []struct {
		what  string
		reg   MCPRegistration
		wants bool
	}{
		{"a tool that declares none at all", MCPRegistration{}, false},
		{"a file, a key and a shape", MCPRegistration{File: "opencode.json", ServersKey: "mcp", CommandStyle: CommandArgv}, false},
		{"a file with no key the servers live under", MCPRegistration{File: "opencode.json", CommandStyle: CommandArgv}, true},
		{"a shape nothing writes", MCPRegistration{File: "x.json", ServersKey: "mcp", CommandStyle: "toml"}, true},
	} {
		t.Run(tc.what, func(t *testing.T) {
			a := base
			a.MCPRegistration = tc.reg

			problems := checkAgent(a)

			if tc.wants && len(problems) == 0 {
				t.Fatal("a registration the shim cannot rewrite was accepted")
			}
			if !tc.wants && len(problems) != 0 {
				t.Fatalf("a legitimate registration was refused: %v", problems)
			}
		})
	}
}
