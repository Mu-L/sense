package setup

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectClinePath(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	_ = os.WriteFile(filepath.Join(binDir, "cline"), []byte("#!/bin/sh\n"), 0o755)

	result := detectCline()
	if !result.Found || result.Evidence != "cline on PATH" {
		t.Errorf("detectCline() = %+v, want found on PATH", result)
	}
}

func TestDetectClineHomeDir(t *testing.T) {
	t.Setenv("PATH", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".cline"), 0o755)

	result := detectCline()
	if !result.Found || result.Evidence != "~/.cline/ directory" {
		t.Errorf("detectCline() = %+v, want found in home dir", result)
	}
}

func TestDetectClineVSCodeExtension(t *testing.T) {
	t.Setenv("PATH", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".vscode", "extensions", "saoudrizwan.claude-dev-3.20.0"), 0o755)

	result := detectCline()
	if !result.Found || result.Evidence != "VS Code extension installed" {
		t.Errorf("detectCline() = %+v, want found via the extension", result)
	}
}

func TestDetectClineNoHome(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", "")

	if result := detectCline(); result.Found {
		t.Errorf("detectCline() = %+v, want not found without a home dir", result)
	}
}

func TestDetectClineNotFound(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	if result := detectCline(); result.Found {
		t.Errorf("detectCline() = %+v, want not found", result)
	}
}

// Cline has no project-level MCP config, so setup writes only the guidance
// and tells the user how to register the server globally.
func TestClineWritesAgentsMDAndMCPNote(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if _, err := Run(root, &buf, &Options{Tools: []Tool{ToolCline}}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(readFile(t, root, "AGENTS.md"), markerStart) {
		t.Error("AGENTS.md should carry the Sense section")
	}
	if !strings.Contains(buf.String(), clineMCPNote) {
		t.Errorf("summary should carry the MCP note, got:\n%s", buf.String())
	}
	for _, rel := range []string{".mcp.json", ".clinerules"} {
		if exists(root, rel) {
			t.Errorf("%s should not be written for Cline", rel)
		}
	}
}

func TestClineIdempotentOverUserContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "AGENTS.md", "# My rules\n")

	for range 2 {
		if _, err := Run(root, io.Discard, &Options{Tools: []Tool{ToolCline}}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	got := readFile(t, root, "AGENTS.md")
	if !strings.HasPrefix(got, "# My rules\n") {
		t.Errorf("user heading lost:\n%s", got)
	}
	if n := strings.Count(got, markerStart); n != 1 {
		t.Errorf("AGENTS.md has %d Sense sections, want 1", n)
	}
}

func TestClineUndoRestoresProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "AGENTS.md", "# My rules\n")
	if _, err := Run(root, io.Discard, &Options{Tools: []Tool{ToolCline}}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := unconfigureCline(root); err != nil {
		t.Fatalf("unconfigureCline: %v", err)
	}
	if got := readFile(t, root, "AGENTS.md"); got != "# My rules\n" {
		t.Errorf("AGENTS.md after undo = %q, want the user's content only", got)
	}
}

func TestConfigureClineAgentsMDError(t *testing.T) {
	root := t.TempDir()
	// AGENTS.md as a directory makes the write fail.
	if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := configureCline(root); err == nil || !strings.Contains(err.Error(), "write AGENTS.md") {
		t.Errorf("configureCline err = %v, want a write AGENTS.md error", err)
	}
}
