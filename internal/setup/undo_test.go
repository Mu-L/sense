package setup

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// treeOf lists every path under root, relative and sorted, so a test can
// assert that undo left the project exactly as it found it.
func treeOf(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(paths)
	return paths
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

// TestUndoRestoresVirginProject is the headline guarantee: on a project that
// had nothing of its own, setup then undo leaves no trace behind.
func TestUndoRestoresVirginProject(t *testing.T) {
	root := t.TempDir()
	before := treeOf(t, root)

	if _, err := Run(root, io.Discard, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(treeOf(t, root)) == 0 {
		t.Fatal("setup wrote nothing; nothing to undo")
	}

	var buf bytes.Buffer
	if _, err := Undo(root, &buf, nil); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	after := treeOf(t, root)
	if !slices.Equal(after, before) {
		t.Errorf("project not restored after undo:\n before: %v\n  after: %v", before, after)
	}
	if !strings.Contains(buf.String(), "binary is untouched") {
		t.Errorf("summary should say the binary survives, got:\n%s", buf.String())
	}
}

// TestUndoAllToolsRemovesEveryFile covers the four tools together: every file
// setup writes for any of them is gone afterwards.
func TestUndoAllToolsRemovesEveryFile(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, io.Discard, &Options{Tools: AllTools()}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := Undo(root, io.Discard, nil); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	for _, rel := range []string{
		".mcp.json", ".claude/settings.json", "CLAUDE.md",
		".claude/skills/sense-explore.md", ".claude/agents/deep-explore.md", ".claude",
		".cursor/mcp.json", ".cursorrules", ".cursor",
		".codex/config.toml", ".codex", "AGENTS.md",
		"opencode.json", ".opencode/plugin/sense.js",
		".opencode/skills/sense-explore/SKILL.md", ".opencode",
	} {
		if exists(root, rel) {
			t.Errorf("%s survived undo", rel)
		}
	}
}

// TestUndoPreservesUserContent is the reason this is a command and not an rm:
// every file Sense merged into keeps what the user put there.
func TestUndoPreservesUserContent(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, ".mcp.json", `{"mcpServers":{"other":{"command":"other"}}}`)
	writeFile(t, root, ".claude/settings.json",
		`{"hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"my-linter"}]}]},`+
			`"permissions":{"allow":["Bash(ls:*)"]},"model":"opus"}`)
	writeFile(t, root, "CLAUDE.md", "# My rules\n\nDo the thing.\n")
	writeFile(t, root, ".cursorrules", "my cursor rules\n")
	writeFile(t, root, ".cursor/mcp.json", `{"mcpServers":{"other":{"command":"other"}}}`)
	writeFile(t, root, ".codex/config.toml", "model = \"gpt-5\"\n")
	writeFile(t, root, "AGENTS.md", "# Agents\n\nMine.\n")
	writeFile(t, root, "opencode.json", `{"mcp":{"other":{"type":"local"}},"theme":"dark"}`)

	if _, err := Run(root, io.Discard, &Options{Tools: AllTools()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := Undo(root, io.Discard, nil); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	for _, tc := range []struct{ rel, want string }{
		{"CLAUDE.md", "Do the thing."},
		{".cursorrules", "my cursor rules"},
		{"AGENTS.md", "Mine."},
		{".codex/config.toml", "gpt-5"},
	} {
		got := readFile(t, root, tc.rel)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s lost user content: %q", tc.rel, got)
		}
		if strings.Contains(got, "sense:start") || strings.Contains(got, "Sense index for codebase") {
			t.Errorf("%s still carries Sense's section: %q", tc.rel, got)
		}
	}

	for _, rel := range []string{".mcp.json", ".cursor/mcp.json", "opencode.json"} {
		var m map[string]any
		if err := json.Unmarshal([]byte(readFile(t, root, rel)), &m); err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		key := "mcpServers"
		if rel == "opencode.json" {
			key = "mcp"
		}
		servers, _ := m[key].(map[string]any)
		if _, ok := servers["sense"]; ok {
			t.Errorf("%s still declares the sense server", rel)
		}
		if _, ok := servers["other"]; !ok {
			t.Errorf("%s lost the user's own server: %v", rel, m)
		}
	}

	settings := map[string]any{}
	if err := json.Unmarshal([]byte(readFile(t, root, ".claude/settings.json")), &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	raw := readFile(t, root, ".claude/settings.json")
	if strings.Contains(raw, "sense hook") || strings.Contains(raw, "mcp__sense__") {
		t.Errorf("settings.json still carries Sense entries: %s", raw)
	}
	if !strings.Contains(raw, "my-linter") || !strings.Contains(raw, "Bash(ls:*)") {
		t.Errorf("settings.json lost the user's hook or permission: %s", raw)
	}
	if settings["model"] != "opus" {
		t.Errorf("settings.json lost unrelated keys: %v", settings)
	}
}

// TestUndoIsIdempotent: running it twice, or on a project that was never set
// up, removes nothing and errors not at all.
func TestUndoIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, io.Discard, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := Undo(root, io.Discard, nil); err != nil {
		t.Fatalf("first Undo: %v", err)
	}

	var buf bytes.Buffer
	res, err := Undo(root, &buf, nil)
	if err != nil {
		t.Fatalf("second Undo: %v", err)
	}
	for _, tr := range res.Tools {
		if len(tr.Files) != 0 {
			t.Errorf("second undo reported removals: %v", tr.Files)
		}
	}
	if !strings.Contains(buf.String(), "Nothing to remove") {
		t.Errorf("expected a nothing-to-remove summary, got:\n%s", buf.String())
	}
}

func TestUndoRemovesIndexDirAndGitignoreEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".sense/index.db", strings.Repeat("x", 2048))
	writeFile(t, root, ".gitignore", "node_modules/\n\n# Sense index (auto-generated by sense scan)\n.sense/\n")

	var buf bytes.Buffer
	res, err := Undo(root, &buf, nil)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}

	if exists(root, ".sense") {
		t.Error(".sense/ survived undo")
	}
	gi := readFile(t, root, ".gitignore")
	if strings.Contains(gi, ".sense") || strings.Contains(gi, "# Sense index") {
		t.Errorf(".gitignore still mentions Sense: %q", gi)
	}
	if !strings.Contains(gi, "node_modules/") {
		t.Errorf(".gitignore lost the user's entries: %q", gi)
	}
	if len(res.Project) != 2 {
		t.Errorf("Project = %v, want the index dir and the gitignore entry", res.Project)
	}
	if !strings.Contains(buf.String(), "2.0 KB") {
		t.Errorf("expected the index size in the summary, got:\n%s", buf.String())
	}
}

func TestUndoNotesHandWrittenConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".sense/config.yml", "embeddings:\n  enabled: false\n")

	res, err := Undo(root, io.Discard, nil)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}

	var found bool
	for _, n := range res.Notes {
		if strings.Contains(n, "config.yml") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a note about the hand-written config, got %v", res.Notes)
	}
}

func TestUndoNotesSharedCache(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, io.Discard, claudeCodeOnly()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var buf bytes.Buffer
	if _, err := Undo(root, &buf, nil); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if !strings.Contains(buf.String(), "~/.sense/cache") {
		t.Errorf("expected the shared cache note, got:\n%s", buf.String())
	}
}

// A run that removed nothing has nothing to explain: the notes describe what a
// teardown spares, so they are noise when no teardown happened.
func TestUndoNoOpPrintsNoNotes(t *testing.T) {
	var buf bytes.Buffer
	res, err := Undo(t.TempDir(), &buf, nil)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if len(res.Notes) == 0 {
		t.Error("the note is still part of the result")
	}
	if strings.Contains(buf.String(), "note:") {
		t.Errorf("a no-op run should print no notes, got:\n%s", buf.String())
	}
}

// A teardown that stops halfway must still say what it already removed, or the
// user cannot tell which half of the project they are in.
func TestUndoReportsWhatItRemovedBeforeFailing(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, io.Discard, &Options{Tools: AllTools()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Claude Code tears down first and cleanly; Cursor then trips on a config
	// that will not parse.
	writeFile(t, root, ".cursor/mcp.json", "{not json")

	var buf bytes.Buffer
	res, err := Undo(root, &buf, &Options{Tools: []Tool{ToolClaudeCode, ToolCursor}})
	if err == nil {
		t.Fatal("expected the unparseable config to fail the teardown")
	}
	if res == nil {
		t.Fatal("a failed teardown must still return what it removed")
	}

	got := buf.String()
	if !strings.Contains(got, "Removing Claude Code integration") {
		t.Errorf("summary lost the work that succeeded, got:\n%s", got)
	}
	if !strings.Contains(got, "Stopped early") {
		t.Errorf("summary should say it did not finish, got:\n%s", got)
	}
	if strings.Contains(got, "Done. Sense is removed") {
		t.Errorf("a failed teardown must not claim to be done, got:\n%s", got)
	}
}

// Nothing removed and an error to report: the error is the whole story.
func TestUndoSilentWhenNothingRemovedAndFailed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".mcp.json", "{not json")

	var buf bytes.Buffer
	if _, err := Undo(root, &buf, &Options{Tools: []Tool{ToolClaudeCode}}); err == nil {
		t.Fatal("expected an error")
	}
	if buf.String() != "" {
		t.Errorf("expected no summary, got:\n%s", buf.String())
	}
}

// TestUndoWithToolsSparesIndex: a narrowed teardown unwires one tool and
// leaves the index, which belongs to no single tool.
func TestUndoWithToolsSparesIndex(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(root, io.Discard, &Options{Tools: AllTools()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	writeFile(t, root, ".sense/index.db", "db")

	res, err := Undo(root, io.Discard, &Options{Tools: []Tool{ToolCursor}})
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}

	if !exists(root, ".sense/index.db") {
		t.Error("a --tools undo removed the index")
	}
	if len(res.Project) != 0 {
		t.Errorf("Project = %v, want empty for a narrowed undo", res.Project)
	}
	if exists(root, ".cursorrules") {
		t.Error(".cursorrules survived a cursor undo")
	}
	if !exists(root, "CLAUDE.md") {
		t.Error("a cursor undo removed Claude Code's files")
	}
}

// TestUndoDefaultsToAllToolsNotDetectedOnes: teardown must not depend on what
// happens to be installed on this machine now.
func TestUndoDefaultsToAllTools(t *testing.T) {
	got := resolveUndoTools(nil)
	if len(got) != len(AllTools()) {
		t.Errorf("resolveUndoTools(nil) = %v, want every tool", got)
	}
	if named := resolveUndoTools(&Options{Tools: []Tool{ToolCursor}}); len(named) != 1 {
		t.Errorf("resolveUndoTools(cursor) = %v, want just cursor", named)
	}
	if !IndexInScope(&Options{}) || IndexInScope(&Options{Tools: []Tool{ToolCursor}}) {
		t.Error("IndexInScope misreads a narrowed teardown")
	}
	// Naming every tool is still a narrowed run: the caller asked for tools,
	// not for the project, so the index stays.
	if IndexInScope(&Options{Tools: AllTools()}) {
		t.Error("IndexInScope should treat an explicit tool list as narrowed")
	}
}

func TestUndoLeavesHandWrittenCodexTable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".codex/config.toml", "[mcp_servers.sense]\ncommand = \"sense\"\n")

	if _, err := Undo(root, io.Discard, &Options{Tools: []Tool{ToolCodexCLI}}); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	got := readFile(t, root, ".codex/config.toml")
	if !strings.Contains(got, "[mcp_servers.sense]") {
		t.Errorf("undo removed a table it did not write: %q", got)
	}
}

func TestUndoUnparseableJSONErrors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".mcp.json", "{not json")

	if _, err := Undo(root, io.Discard, &Options{Tools: []Tool{ToolClaudeCode}}); err == nil {
		t.Fatal("expected an error on unparseable .mcp.json")
	}
}

func TestUnconfigureToolUnknown(t *testing.T) {
	if _, err := unconfigureTool(t.TempDir(), Tool("nope")); err == nil {
		t.Fatal("expected an error for an unknown tool")
	}
}

func TestRemoveMarkerSectionEdgeCases(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		o, err := removeMarkerSection(filepath.Join(t.TempDir(), "none.md"), markerStart, markerEnd)
		if err != nil || o != outcomeUnchanged {
			t.Errorf("outcome=%v err=%v, want unchanged/nil", o, err)
		}
	})

	t.Run("no markers", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "CLAUDE.md", "mine only\n")
		o, err := removeMarkerSection(filepath.Join(root, "CLAUDE.md"), markerStart, markerEnd)
		if err != nil || o != outcomeUnchanged {
			t.Errorf("outcome=%v err=%v, want unchanged/nil", o, err)
		}
	})

	t.Run("missing end marker truncates", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "CLAUDE.md", "keep me\n\n"+markerStart+"\nsense stuff\n")
		o, err := removeMarkerSection(filepath.Join(root, "CLAUDE.md"), markerStart, markerEnd)
		if err != nil || o != outcomeStripped {
			t.Fatalf("outcome=%v err=%v, want stripped/nil", o, err)
		}
		if got := readFile(t, root, "CLAUDE.md"); got != "keep me\n" {
			t.Errorf("content = %q, want %q", got, "keep me\n")
		}
	})

	t.Run("a delete that fails is reported, not swallowed", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root: directory permissions are not enforced")
		}
		root := t.TempDir()
		writeFile(t, root, "sub/CLAUDE.md", markerStart+"\nstuff\n"+markerEnd+"\n")
		sub := filepath.Join(root, "sub")
		if err := os.Chmod(sub, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

		o, err := removeMarkerSection(filepath.Join(sub, "CLAUDE.md"), markerStart, markerEnd)
		if err == nil || o != outcomeUnchanged {
			t.Errorf("outcome=%v err=%v, want unchanged and an error", o, err)
		}
	})

	t.Run("sense-only file is deleted", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "CLAUDE.md", markerStart+"\nstuff\n"+markerEnd+"\n")
		if _, err := removeMarkerSection(filepath.Join(root, "CLAUDE.md"), markerStart, markerEnd); err != nil {
			t.Fatalf("removeMarkerSection: %v", err)
		}
		if exists(root, "CLAUDE.md") {
			t.Error("a file holding only Sense's section should be deleted")
		}
	})
}

func TestPruneJSONFileMissingIsNoOp(t *testing.T) {
	o, err := pruneJSONFile(filepath.Join(t.TempDir(), "none.json"), stripMCPServers)
	if err != nil || o != outcomeUnchanged {
		t.Errorf("outcome=%v err=%v, want unchanged/nil", o, err)
	}
}

func TestPruneJSONFileDeleteFailureSurfaces(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, "sub/.mcp.json", `{"mcpServers":{"sense":{}}}`)
	sub := filepath.Join(root, "sub")
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	o, err := pruneJSONFile(filepath.Join(sub, ".mcp.json"), stripMCPServers)
	if err == nil || o != outcomeUnchanged {
		t.Errorf("outcome=%v err=%v, want unchanged and an error", o, err)
	}
}

// The outcome a primitive reports is what the summary says happened, so the
// two shapes must not blur: a stripped file is still on disk.
func TestPruneJSONFileReportsStrippedVersusDeleted(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "shared.json", `{"mcpServers":{"sense":{},"other":{}}}`)
	o, err := pruneJSONFile(filepath.Join(root, "shared.json"), stripMCPServers)
	if err != nil || o != outcomeStripped {
		t.Errorf("outcome=%v err=%v, want stripped/nil", o, err)
	}
	if !exists(root, "shared.json") {
		t.Error("a stripped file must survive")
	}

	writeFile(t, root, "senseonly.json", `{"mcpServers":{"sense":{}}}`)
	o, err = pruneJSONFile(filepath.Join(root, "senseonly.json"), stripMCPServers)
	if err != nil || o != outcomeDeleted {
		t.Errorf("outcome=%v err=%v, want deleted/nil", o, err)
	}
	if exists(root, "senseonly.json") {
		t.Error("a file holding only Sense's entry should be gone")
	}
}

func TestRemoveSenseHooksLeavesOtherEvents(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"hooks": []any{map[string]any{"command": "sense hook pre-tool-use"}}},
				map[string]any{"hooks": []any{map[string]any{"command": "mine"}}},
			},
			"Stop":  []any{map[string]any{"hooks": []any{map[string]any{"command": "sense hook stop"}}}},
			"Other": "not an array",
		},
	}

	if !removeSenseHooks(settings) {
		t.Fatal("removeSenseHooks reported no change")
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if _, ok := hooks["Stop"]; ok {
		t.Error("an event holding only Sense entries should be dropped")
	}
	pre, _ := hooks["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Errorf("PreToolUse = %v, want the user's entry alone", pre)
	}
	if _, ok := hooks["Other"]; !ok {
		t.Error("a non-array event should be left alone")
	}

	if removeSenseHooks(map[string]any{}) {
		t.Error("removeSenseHooks on empty settings reported a change")
	}
}

func TestRemoveSensePermissionsPrunesEmptyBlock(t *testing.T) {
	settings := map[string]any{
		"permissions": map[string]any{"allow": []any{claudePermissionPattern}},
	}
	if !removeSensePermissions(settings, []string{claudePermissionPattern}) {
		t.Fatal("reported no change")
	}
	if _, ok := settings["permissions"]; ok {
		t.Errorf("an empty permissions block should be pruned: %v", settings)
	}

	untouched := map[string]any{"permissions": map[string]any{"allow": []any{"Bash(ls:*)"}}}
	if removeSensePermissions(untouched, []string{claudePermissionPattern}) {
		t.Error("reported a change with nothing of Sense's to remove")
	}
	if removeSensePermissions(map[string]any{}, []string{claudePermissionPattern}) {
		t.Error("reported a change on settings with no permissions block")
	}
}

func TestRemoveMCPServerAbsent(t *testing.T) {
	if removeMCPServer(map[string]any{}, "mcpServers") {
		t.Error("reported a change with no servers declared")
	}
}

func TestRemoveSenseFromGitignore(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		removed, err := removeSenseFromGitignore(t.TempDir())
		if err != nil || removed {
			t.Errorf("removed=%v err=%v, want false/nil", removed, err)
		}
	})

	t.Run("no sense entry", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".gitignore", "dist/\n")
		removed, err := removeSenseFromGitignore(root)
		if err != nil || removed {
			t.Errorf("removed=%v err=%v, want false/nil", removed, err)
		}
	})

	t.Run("bare entry without comment", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".gitignore", "dist/\n.sense\n")
		if _, err := removeSenseFromGitignore(root); err != nil {
			t.Fatalf("removeSenseFromGitignore: %v", err)
		}
		if got := readFile(t, root, ".gitignore"); got != "dist/\n" {
			t.Errorf("content = %q, want %q", got, "dist/\n")
		}
	})

	t.Run("sense-only file is emptied not deleted", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".gitignore", "# Sense index (auto-generated by sense scan)\n.sense/\n")
		if _, err := removeSenseFromGitignore(root); err != nil {
			t.Fatalf("removeSenseFromGitignore: %v", err)
		}
		if got := readFile(t, root, ".gitignore"); got != "" {
			t.Errorf("content = %q, want empty", got)
		}
	})
}

func TestHumanSize(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	} {
		if got := humanSize(tc.n); got != tc.want {
			t.Errorf("humanSize(%d) = %s, want %s", tc.n, got, tc.want)
		}
	}
}

func TestDirSizeMissing(t *testing.T) {
	size, existed := dirSize(filepath.Join(t.TempDir(), "none"))
	if existed || size != 0 {
		t.Errorf("dirSize = (%d, %v), want (0, false)", size, existed)
	}
}

func TestRemoveOwnedFileMissing(t *testing.T) {
	o, err := removeOwnedFile(filepath.Join(t.TempDir(), "none.md"))
	if err != nil || o != outcomeUnchanged {
		t.Errorf("outcome=%v err=%v, want unchanged/nil", o, err)
	}
}

func TestRemoveDirIfEmptyKeepsNonEmpty(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude/mine.md", "mine")
	removeDirIfEmpty(filepath.Join(root, ".claude"))
	if !exists(root, ".claude/mine.md") {
		t.Error("removeDirIfEmpty took a directory holding the user's file")
	}
}

// The teardown's own error paths: a project root that will not let go of its
// files must fail loudly rather than report a removal that did not happen.
func TestUndoReportsRemovalFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sense"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A read-only root leaves every per-tool step a no-op (nothing to find)
	// but makes removing .sense/ fail, which is the path under test.
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	if _, err := Undo(root, io.Discard, nil); err == nil {
		t.Fatal("expected an error when .sense cannot be removed")
	}
}

func TestUndoReportsGitignoreFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, ".gitignore", ".sense/\n")
	if err := os.Chmod(filepath.Join(root, ".gitignore"), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, ".gitignore"), 0o644) })

	if err := undoProject(root, &Result{}); err == nil {
		t.Fatal("expected an error when .gitignore cannot be rewritten")
	}
}

func TestRemoveFileReportsRealErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, "sub/skill.md", "x")
	sub := filepath.Join(root, "sub")
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	if err := removeFile(filepath.Join(sub, "skill.md")); err == nil {
		t.Fatal("expected an error removing from a read-only directory")
	}
}

func TestRemoveOwnedFileReportsStatErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, "sub/skill.md", "x")
	sub := filepath.Join(root, "sub")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	if o, err := removeOwnedFile(filepath.Join(sub, "skill.md")); err == nil || o != outcomeUnchanged {
		t.Fatal("expected an error stating a file in an unreadable directory")
	}
}

func TestRemoveSenseFromGitignoreReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, ".gitignore", ".sense/\n")
	if err := os.Chmod(filepath.Join(root, ".gitignore"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, ".gitignore"), 0o644) })

	if _, err := removeSenseFromGitignore(root); err == nil {
		t.Fatal("expected an error reading an unreadable .gitignore")
	}
}

// Every registry entry must carry both halves. A tool wired with a configurer
// and no teardown would write files that `sense setup --undo` then walks past
// (and unconfigureTool would call a nil function), so the pairing is enforced
// here rather than discovered by the user who tried to remove Sense.
func TestRegistryPairsConfigureWithUnconfigure(t *testing.T) {
	for _, e := range registry() {
		if e.detect == nil {
			t.Errorf("%s: no detector", e.id)
		}
		if e.configure == nil {
			t.Errorf("%s: no configurer", e.id)
		}
		if e.unconfigure == nil {
			t.Errorf("%s: has a configurer but no teardown; add unconfigure%s to its file", e.id, e.displayName)
		}
	}
}

// The summary must not claim a deletion that did not happen: a config Sense
// shared with the user is still on disk afterwards, so it is reported as
// entries removed, not as a file removed.
func TestUndoSummaryDistinguishesStrippedFromDeleted(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".mcp.json", `{"mcpServers":{"other":{"command":"other"}}}`)
	if _, err := Run(root, io.Discard, claudeCodeOnly()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var buf bytes.Buffer
	if _, err := Undo(root, &buf, nil); err != nil {
		t.Fatalf("Undo: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "removed Sense entries from .mcp.json") {
		t.Errorf("a file that survives should be reported as stripped, got:\n%s", got)
	}
	if !exists(root, ".mcp.json") {
		t.Error(".mcp.json should still hold the user's server")
	}
	// CLAUDE.md held nothing but Sense's section, so it really is gone.
	if !strings.Contains(got, "removed CLAUDE.md") {
		t.Errorf("a deleted file should be reported plainly, got:\n%s", got)
	}
}

// breakFile makes one teardown step fail, without touching the others.
func unparseable(rel string) func(*testing.T, string) {
	return func(t *testing.T, root string) { writeFile(t, root, rel, "{not json") }
}

func unreadable(rel string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		chmodForTest(t, filepath.Join(root, rel), 0o000)
	}
}

func readOnlyDir(rel string) func(*testing.T, string) {
	return func(t *testing.T, root string) {
		chmodForTest(t, filepath.Join(root, rel), 0o555)
	}
}

// chmodForTest changes a mode for the length of a test and puts back the one
// it found, so a file left readable-only does not come back executable.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, original) })
}

// Every teardown step touches a file it does not own, and every one of them can
// fail. Each case breaks exactly one step and asserts the error names the file
// it tripped on, so a step that swallowed its own failure shows up here as a
// teardown that wrongly reported success.
func TestUnconfigureStepFailures(t *testing.T) {
	cases := []struct {
		name      string
		tool      Tool
		breakStep func(*testing.T, string)
		want      string
	}{
		{"claude mcp json", ToolClaudeCode, unparseable(".mcp.json"), "update .mcp.json"},
		{"claude settings", ToolClaudeCode, unparseable(".claude/settings.json"), "update .claude/settings.json"},
		{"claude guidance", ToolClaudeCode, unreadable("CLAUDE.md"), "update CLAUDE.md"},
		{"claude skills", ToolClaudeCode, readOnlyDir(".claude/skills"), "remove .claude/skills"},
		{"claude agents", ToolClaudeCode, readOnlyDir(".claude/agents"), "remove .claude/agents"},

		{"cursor mcp json", ToolCursor, unparseable(".cursor/mcp.json"), "update .cursor/mcp.json"},
		{"cursor rules", ToolCursor, unreadable(".cursorrules"), "update .cursorrules"},

		{"codex toml", ToolCodexCLI, unreadable(".codex/config.toml"), "update .codex/config.toml"},
		{"codex mcp json", ToolCodexCLI, unparseable(".mcp.json"), "update .mcp.json"},
		{"codex agents md", ToolCodexCLI, unreadable("AGENTS.md"), "update AGENTS.md"},

		{"opencode json", ToolOpencode, unparseable("opencode.json"), "update opencode.json"},
		{"opencode agents md", ToolOpencode, unreadable("AGENTS.md"), "update AGENTS.md"},
		{"opencode skills", ToolOpencode, readOnlyDir(".opencode/skills/sense-explore"), "remove .opencode/skills"},
		{"opencode plugin", ToolOpencode, readOnlyDir(".opencode/plugin"), "remove .opencode/plugin"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("running as root: permissions are not enforced")
			}
			root := t.TempDir()
			if _, err := Run(root, io.Discard, &Options{Tools: []Tool{tc.tool}}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			tc.breakStep(t, root)

			tr, err := unconfigureTool(root, tc.tool)
			if err == nil {
				t.Fatal("expected the broken step to fail the teardown")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %q", err, tc.want)
			}
			if tr == nil {
				t.Error("a failed teardown must still return what it removed")
			}
		})
	}
}

// Stat succeeds and the removal still fails: a file in a directory that will
// not give it up.
func TestRemoveOwnedFileReportsRemovalErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, "sub/skill.md", "x")
	chmodForTest(t, filepath.Join(root, "sub"), 0o555)

	o, err := removeOwnedFile(filepath.Join(root, "sub", "skill.md"))
	if err == nil || o != outcomeUnchanged {
		t.Errorf("outcome=%v err=%v, want unchanged and an error", o, err)
	}
}

func TestPruneJSONFileReportsStatErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, "sub/.mcp.json", "{}")
	chmodForTest(t, filepath.Join(root, "sub"), 0o000)

	o, err := pruneJSONFile(filepath.Join(root, "sub", ".mcp.json"), stripMCPServers)
	if err == nil || o != outcomeUnchanged {
		t.Errorf("outcome=%v err=%v, want unchanged and an error", o, err)
	}
}

// Readable but not writable: the section comes out in memory and the rewrite
// fails, which must surface rather than look like a clean strip.
func TestRemoveMarkerSectionReportsWriteErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	root := t.TempDir()
	writeFile(t, root, "CLAUDE.md", "mine\n\n"+markerStart+"\nsense\n"+markerEnd+"\n")
	chmodForTest(t, filepath.Join(root, "CLAUDE.md"), 0o444)

	o, err := removeMarkerSection(filepath.Join(root, "CLAUDE.md"), markerStart, markerEnd)
	if err == nil || o != outcomeUnchanged {
		t.Errorf("outcome=%v err=%v, want unchanged and an error", o, err)
	}
}
