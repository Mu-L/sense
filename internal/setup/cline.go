package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// clineMCPNote is printed after setup because Cline reads MCP servers only
// from its machine-wide settings, never from the project, and setup does not
// write outside the project.
const clineMCPNote = "Cline reads MCP servers only from its global settings, not from the project. " +
	`In Cline, open MCP Servers > Configure and add under "mcpServers": "sense": {"command": "sense", "args": ["mcp"]}.`

// clineExtensionGlob matches the Cline VS Code extension install directory
// (publisher saoudrizwan, id claude-dev), relative to the home directory.
const clineExtensionGlob = ".vscode/extensions/saoudrizwan.claude-dev-*"

// detectCline looks for evidence that Cline is installed: the CLI on PATH,
// its ~/.cline data directory, or the VS Code extension.
func detectCline() DetectResult {
	r := DetectResult{Tool: ToolCline}
	if _, err := exec.LookPath("cline"); err == nil {
		r.Found = true
		r.Evidence = "cline on PATH"
		return r
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return r
	}
	if _, err := os.Stat(filepath.Join(home, ".cline")); err == nil {
		r.Found = true
		r.Evidence = "~/.cline/ directory"
		return r
	}
	if matches, _ := filepath.Glob(filepath.Join(home, clineExtensionGlob)); len(matches) > 0 {
		r.Found = true
		r.Evidence = "VS Code extension installed"
	}
	return r
}

// configureCline writes the AGENTS.md guidance Cline loads as a project rule,
// and notes the global MCP registration it needs.
func configureCline(root string) (*ToolResult, error) {
	tr := &ToolResult{Tool: ToolCline}

	if wrote, err := writeAgentsMD(root); err != nil {
		return nil, fmt.Errorf("write AGENTS.md: %w", err)
	} else if wrote {
		tr.Files = append(tr.Files, "AGENTS.md")
	}

	tr.Notes = append(tr.Notes, clineMCPNote)

	return tr, nil
}

// unconfigureCline is the inverse of configureCline: it strips Sense's section
// from AGENTS.md, leaving anything the user wrote.
func unconfigureCline(root string) (*ToolResult, error) {
	tr := &ToolResult{Tool: ToolCline}

	o, err := removeMarkerSection(filepath.Join(root, "AGENTS.md"), markerStart, markerEnd)
	if err != nil {
		return tr, fmt.Errorf("update AGENTS.md: %w", err)
	}
	tr.record(o, "AGENTS.md")

	return tr, nil
}
