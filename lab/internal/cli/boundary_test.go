package cli

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sense-lab boundary is two rules: nothing outside lab/ may import lab's
// packages, and lab may never import sense/internal. The Go compiler enforces
// the first for free and depguard enforces the second, but the compiler's
// protection rests on a fact about the tree that a refactor can remove
// silently, and neither failure is visible in a bench result. These tests
// assert the facts, and they run in `make test` rather than only under the
// linter.

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}

// labFile is one Go source file under lab/, by module-relative path.
type labFile struct {
	path    string // e.g. "lab/internal/cli/cli.go"
	pkg     string // the package clause
	imports []string
}

// labFiles parses every .go file under lab/, test files included.
func labFiles(t *testing.T) []labFile {
	t.Helper()
	root := repoRoot(t)
	fset := token.NewFileSet()
	var out []labFile

	err := filepath.WalkDir(filepath.Join(root, "lab"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// testdata holds fixtures the Go toolchain ignores, and this
			// walk must ignore them too: they are not always valid Go.
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		lf := labFile{pkg: f.Name.Name}
		for _, spec := range f.Imports {
			lf.imports = append(lf.imports, strings.Trim(spec.Path.Value, `"`))
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		lf.path = filepath.ToSlash(rel)
		out = append(out, lf)
		return nil
	})
	if err != nil {
		t.Fatalf("walk lab/: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("found no Go files under lab/; the walk is not looking where it thinks")
	}
	return out
}

// The only reason nothing outside lab/ can import lab's packages is Go's
// internal rule, and that rule only covers packages under lab/internal/. A
// package added at, say, lab/scoring/ would compile fine and silently lose the
// protection. lab/cmd/ is the one other allowed location, and only for main
// packages: a lab/cmd/shared/ library would be importable from anywhere, which
// is exactly the leak this test exists to catch.
func TestEveryLabPackageLivesWhereTheCompilerProtectsIt(t *testing.T) {
	for _, f := range labFiles(t) {
		switch {
		case strings.HasPrefix(f.path, "lab/internal/"):
			// Protected by the compiler's internal rule.
		case strings.HasPrefix(f.path, "lab/cmd/"):
			if f.pkg != "main" {
				t.Errorf("%s declares package %s under lab/cmd/; only main packages belong there, "+
					"because nothing stops code outside lab/ from importing a library here",
					f.path, f.pkg)
			}
		default:
			t.Errorf("%s is a lab Go file outside lab/internal/ and lab/cmd/; "+
				"nothing outside lab/ is stopped from importing it", f.path)
		}
	}
}

// The MCP-only law: sense-lab may reach Sense through the MCP server and
// nowhere else. Importing sense/internal would let the bench query the index on
// a channel the benched agent never had, and the result would look normal.
// depguard states this rule too; it is repeated here so the law does not depend
// on anyone having run the linter.
func TestLabNeverImportsSenseInternals(t *testing.T) {
	for _, f := range labFiles(t) {
		for _, imp := range f.imports {
			if strings.HasPrefix(imp, "github.com/luuuc/sense/internal/") {
				t.Errorf("%s imports %s; sense-lab must go through the MCP server", f.path, imp)
			}
		}
	}
}
