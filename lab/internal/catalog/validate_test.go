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
		ConfigDirVar: "TOOL_CONFIG_DIR", KeychainService: "Tool-credentials", CredentialKey: "toolOauth",
	}
	for _, tc := range []struct {
		what  string
		drop  func(*Agent)
		wants bool
	}{
		{"the whole route declared", func(*Agent) {}, false},
		// A tool that takes no config directory authenticates by key, and
		// declaring none of the three is how it says so.
		{"no route at all", func(a *Agent) { a.ConfigDirVar, a.KeychainService, a.CredentialKey = "", "", "" }, false},
		{"no variable to point the run at its config", func(a *Agent) { a.ConfigDirVar = "" }, true},
		{"nowhere to read the operator's own login", func(a *Agent) { a.KeychainService = "" }, true},
		{"no key the credential lives under", func(a *Agent) { a.CredentialKey = "" }, true},
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
