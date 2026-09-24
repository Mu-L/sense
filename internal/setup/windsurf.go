package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// windsurfMCPNote is printed after setup because Windsurf reads MCP servers
// only from its machine-wide mcp_config.json, never from the project, and
// setup does not write outside the project.
const windsurfMCPNote = "Windsurf reads MCP servers only from its global mcp_config.json, not from the project. " +
	`Add Sense under "mcpServers" there: "sense": {"command": "sense", "args": ["mcp"]}.`

// detectWindsurf looks for evidence that Windsurf is installed.
func detectWindsurf() DetectResult {
	r := DetectResult{Tool: ToolWindsurf}
	if _, err := exec.LookPath("windsurf"); err == nil {
		r.Found = true
		r.Evidence = "windsurf on PATH"
		return r
	}
	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(home, ".codeium", "windsurf")); err == nil {
			r.Found = true
			r.Evidence = "~/.codeium/windsurf/ directory"
			return r
		}
	}
	return r
}

// configureWindsurf writes the AGENTS.md guidance Windsurf loads as an
// always-on rule, and notes the global MCP registration it needs.
func configureWindsurf(root string) (*ToolResult, error) {
	tr := &ToolResult{Tool: ToolWindsurf}

	if wrote, err := writeAgentsMD(root); err != nil {
		return nil, fmt.Errorf("write AGENTS.md: %w", err)
	} else if wrote {
		tr.Files = append(tr.Files, "AGENTS.md")
	}

	tr.Notes = append(tr.Notes, windsurfMCPNote)

	return tr, nil
}

// unconfigureWindsurf is the inverse of configureWindsurf: it strips Sense's
// section from AGENTS.md, leaving anything the user wrote.
func unconfigureWindsurf(root string) (*ToolResult, error) {
	tr := &ToolResult{Tool: ToolWindsurf}

	o, err := removeMarkerSection(filepath.Join(root, "AGENTS.md"), markerStart, markerEnd)
	if err != nil {
		return tr, fmt.Errorf("update AGENTS.md: %w", err)
	}
	tr.record(o, "AGENTS.md")

	return tr, nil
}
