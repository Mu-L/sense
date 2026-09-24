package setup

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectWindsurfPath(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	_ = os.WriteFile(filepath.Join(binDir, "windsurf"), []byte("#!/bin/sh\n"), 0o755)

	result := detectWindsurf()
	if !result.Found || result.Evidence != "windsurf on PATH" {
		t.Errorf("detectWindsurf() = %+v, want found on PATH", result)
	}
}

func TestDetectWindsurfHomeDir(t *testing.T) {
	t.Setenv("PATH", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = os.MkdirAll(filepath.Join(home, ".codeium", "windsurf"), 0o755)

	result := detectWindsurf()
	if !result.Found || result.Evidence != "~/.codeium/windsurf/ directory" {
		t.Errorf("detectWindsurf() = %+v, want found in home dir", result)
	}
}

func TestDetectWindsurfNotFound(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	if result := detectWindsurf(); result.Found {
		t.Errorf("detectWindsurf() = %+v, want not found", result)
	}
}

// Windsurf has no project-level MCP config, so setup writes only the guidance
// and tells the user how to register the server globally.
func TestWindsurfWritesAgentsMDAndMCPNote(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if _, err := Run(root, &buf, &Options{Tools: []Tool{ToolWindsurf}}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(readFile(t, root, "AGENTS.md"), markerStart) {
		t.Error("AGENTS.md should carry the Sense section")
	}
	if !strings.Contains(buf.String(), windsurfMCPNote) {
		t.Errorf("summary should carry the MCP note, got:\n%s", buf.String())
	}
	for _, rel := range []string{".mcp.json", ".windsurf"} {
		if exists(root, rel) {
			t.Errorf("%s should not be written for Windsurf", rel)
		}
	}
}

func TestWindsurfIdempotentOverUserContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "AGENTS.md", "# My rules\n")

	for range 2 {
		if _, err := Run(root, io.Discard, &Options{Tools: []Tool{ToolWindsurf}}); err != nil {
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

func TestWindsurfUndoRestoresProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "AGENTS.md", "# My rules\n")
	if _, err := Run(root, io.Discard, &Options{Tools: []Tool{ToolWindsurf}}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := unconfigureWindsurf(root); err != nil {
		t.Fatalf("unconfigureWindsurf: %v", err)
	}
	if got := readFile(t, root, "AGENTS.md"); got != "# My rules\n" {
		t.Errorf("AGENTS.md after undo = %q, want the user's content only", got)
	}
}

func TestConfigureWindsurfAgentsMDError(t *testing.T) {
	root := t.TempDir()
	// AGENTS.md as a directory makes the write fail.
	if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := configureWindsurf(root); err == nil || !strings.Contains(err.Error(), "write AGENTS.md") {
		t.Errorf("configureWindsurf err = %v, want a write AGENTS.md error", err)
	}
}
