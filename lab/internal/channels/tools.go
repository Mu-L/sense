package channels

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

// handshake is the smallest exchange that gets a tool list out of an MCP
// server: initialise, say so, ask. It is written out rather than built with a
// client library because this is the only MCP conversation the bench ever has,
// and the lab may reach Sense only through this channel.
const handshake = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"sense-lab","version":"1"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`

// ToolNames asks the Sense MCP server what it offers, in an indexed repository.
//
// Derived rather than written down, for the same reason the repository channels
// are: a tool added to the product would leave the bench's list short, the
// baseline arm's transcript would be searched for the old names, and a run in
// which the baseline reached Sense through the new one would pass every check.
//
// The server needs an index to start, so repo must be a prepared sense arm's
// worktree. That is where a real server runs, which is the point.
func ToolNames(ctx context.Context, senseBin, repo string) ([]string, error) {
	cmd := exec.CommandContext(ctx, senseBin, "mcp")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(handshake)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ask the sense server for its tools: %w", err)
	}

	names, err := toolsInReply(out)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		// An empty list makes every transcript check pass, which is the shape
		// of a leak that reads as a clean bill of health.
		return nil, errors.New("the sense server offered no tools; a transcript check against an empty list proves nothing")
	}
	sort.Strings(names)
	return names, nil
}

// toolsInReply pulls the tool names out of the server's answers.
func toolsInReply(out []byte) ([]string, error) {
	var reply struct {
		ID     int `json:"id"`
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	r := bufio.NewReader(strings.NewReader(string(out)))
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			reply.Result.Tools = nil
			if json.Unmarshal(line, &reply) == nil && reply.ID == 2 {
				names := make([]string, 0, len(reply.Result.Tools))
				for _, t := range reply.Result.Tools {
					names = append(names, t.Name)
				}
				return names, nil
			}
		}
		if err != nil {
			return nil, errors.New("the sense server never answered the tool list")
		}
	}
}

// mcpPrefix is how an agent tool names an MCP tool in its own transcript. It is
// matched as well as the bare names, because a transcript that says
// `mcp__sense__sense_graph` and one that says `sense_graph` are the same event.
const mcpPrefix = "mcp__sense"

// UsedBy reports every sign that a transcript used Sense, by name.
//
// Configuration checks say what was set up. This says what was used, and it is
// the one that would reveal a channel nobody thought to enumerate: an arm that
// found the binary some other way leaves no configuration trace at all.
//
// An empty result for the baseline arm is necessary and not sufficient. An arm
// that did not use Sense in one run may still have been able to, which is why
// the channels are checked directly as well.
func UsedBy(transcript []byte, toolNames []string, binary string) []string {
	text := string(transcript)
	var used []string
	if strings.Contains(text, mcpPrefix) {
		used = append(used, "the transcript names an MCP tool from the sense server")
	}
	for _, name := range toolNames {
		if strings.Contains(text, name) {
			used = append(used, "the transcript names the tool "+name)
		}
	}
	if ranBinary(text, binary) {
		used = append(used, "the transcript invokes the "+binary+" binary")
	}
	return used
}

// commandValue finds the shell commands an agent tool recorded running. The
// transcript is searched for those rather than for the binary's name anywhere,
// because a repository under study may legitimately contain the word and a bare
// substring search would report every run against such a codebase.
var commandValue = regexp.MustCompile(`"command"\s*:\s*("(?:[^"\\]|\\.)*")`)

// shellOperator is where one command ends and the next begins.
var shellOperator = regexp.MustCompile(`[|;&()\n]+`)

// ranBinary reports whether any recorded command ran the binary.
//
// It is checked at every command position rather than only the first, because
// `git log | sense search ...` runs it just as surely as `sense search ...`. The
// subcommand is deliberately not part of the test: a list of subcommands here
// would be exactly the written-down list this package exists to avoid.
func ranBinary(text, binary string) bool {
	if binary == "" {
		return false
	}
	for _, match := range commandValue.FindAllStringSubmatch(text, -1) {
		var command string
		if json.Unmarshal([]byte(match[1]), &command) != nil {
			continue
		}
		for _, part := range shellOperator.Split(command, -1) {
			fields := strings.Fields(part)
			if len(fields) > 0 && path.Base(fields[0]) == binary {
				return true
			}
		}
	}
	return false
}
