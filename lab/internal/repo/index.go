package repo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Language is one language's share of an index, as Sense reports it.
type Language struct {
	Files   int    `json:"files"`
	Symbols int    `json:"symbols"`
	Tier    string `json:"tier"`
}

// Index is the index phase's artifact: what an index over this checkout holds,
// and the answer it was quoted from.
//
// The counts are quoted from `sense_status` rather than estimated, and the
// whole answer is kept beside them under Status. A field this record does not
// name is still readable by whoever reads the artifact next, which is the
// difference between a summary and a measurement.
type Index struct {
	Repo              string              `json:"repo"`
	URL               string              `json:"url"`
	Revision          string              `json:"revision"`
	Checkout          string              `json:"checkout"`
	ScannedAt         string              `json:"scanned_at"`
	SenseVersion      string              `json:"sense_version"`
	Files             int                 `json:"files"`
	Symbols           int                 `json:"symbols"`
	Edges             int                 `json:"edges"`
	Embeddings        int                 `json:"embeddings"`
	EmbeddingCoverage float64             `json:"embedding_coverage"`
	Languages         map[string]Language `json:"languages"`
	ProfileTier       string              `json:"profile_tier"`
	// Shortfall is present only when the scan indexed nothing, and it says so
	// in a sentence rather than by leaving a reader to notice two zeroes.
	//
	// The failure it exists to prevent is a misreading rather than a crash: an
	// index quietly short of its symbols is how a repository gets called dark
	// when the tool simply did not run, and the authoring phase reads this file
	// to decide exactly that.
	Shortfall string `json:"shortfall,omitempty"`
	// Status is the `sense_status` answer as it was returned.
	Status json.RawMessage `json:"sense_status"`
}

// Indexed reports whether the scan produced an index worth authoring against.
func (i Index) Indexed() bool { return i.Shortfall == "" }

// statusExchange is the smallest conversation that gets `sense_status` out of
// the Sense MCP server: initialise, say so, ask.
//
// The server is asked rather than the index file read, and that is the law
// rather than a preference: the MCP server is the channel a benched agent
// actually has, and a count taken from the SQLite file would measure a channel
// nobody has.
const statusExchange = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"sense-lab","version":"1"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sense_status","arguments":{}}}
`

// Scan indexes the checkout and records what the index holds.
//
// It runs the product binary the way a user would: `sense scan` over the
// checkout, then the server. If Sense is slow or wrong on a repository, that is
// a finding about the product and this records it rather than working around
// it.
func Scan(ctx context.Context, senseBin string, p Plan, at time.Time) (Index, error) {
	// -dir, not a positional path: sense scan silently ignores positionals.
	// -embed, because the count this artifact records has to be the final one:
	// embedding continues in the background otherwise, and the number written
	// here would be whatever it had reached by the time the process exited.
	scan := exec.CommandContext(ctx, senseBin, "scan", "-dir", p.Checkout, "-embed")
	if out, err := scan.CombinedOutput(); err != nil {
		return Index{}, fmt.Errorf("index %s: %w: %s", p.Checkout, err, bytes.TrimSpace(out))
	}

	raw, err := senseStatus(ctx, senseBin, p.Checkout)
	if err != nil {
		return Index{}, err
	}
	return record(raw, p, at)
}

// senseStatus asks the server what the index holds, in the checkout.
//
// The working directory is the whole measurement. The server reads the index of
// wherever it is started, so a status taken anywhere else answers about another
// repository entirely — with counts that look exactly as plausible as the right
// ones.
//
// Its stderr is carried into the error for the same reason git's is: "exit
// status 1" is not something anybody can act on, and the server's own message
// says which of the several things went wrong.
func senseStatus(ctx context.Context, senseBin, dir string) (json.RawMessage, error) {
	cmd := exec.CommandContext(ctx, senseBin, "mcp")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(statusExchange)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ask the sense server for the status of %s: %w: %s",
			dir, err, bytes.TrimSpace(errBuf.Bytes()))
	}
	return statusInReply(out)
}

// statusInReply pulls the tool's answer out of the server's replies. The answer
// is a document inside a text field, which is how MCP carries one.
func statusInReply(out []byte) (json.RawMessage, error) {
	var reply struct {
		ID     int `json:"id"`
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			reply.Result.Content = nil
			if json.Unmarshal(line, &reply) == nil && reply.ID == 2 && len(reply.Result.Content) > 0 {
				text := json.RawMessage(reply.Result.Content[0].Text)
				if !json.Valid(text) {
					return nil, fmt.Errorf("the sense server's status was not a document: %.120s", text)
				}
				return text, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("the sense server never answered sense_status")
		}
	}
}

// record turns the server's answer into the artifact.
func record(raw json.RawMessage, p Plan, at time.Time) (Index, error) {
	var s struct {
		Index struct {
			Files      int     `json:"files"`
			Symbols    int     `json:"symbols"`
			Edges      int     `json:"edges"`
			Embeddings int     `json:"embeddings"`
			Coverage   float64 `json:"coverage"`
		} `json:"index"`
		Languages map[string]Language `json:"languages"`
		Profile   struct {
			Tier string `json:"tier"`
		} `json:"profile"`
		Version struct {
			Binary string `json:"binary"`
		} `json:"version"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return Index{}, fmt.Errorf("read the sense server's status: %w", err)
	}
	i := Index{
		Repo: p.ID, URL: p.URL, Revision: p.Revision, Checkout: p.Checkout,
		ScannedAt:    at.UTC().Format(time.RFC3339),
		SenseVersion: s.Version.Binary,
		Files:        s.Index.Files,
		Symbols:      s.Index.Symbols,
		Edges:        s.Index.Edges,
		Embeddings:   s.Index.Embeddings,

		EmbeddingCoverage: s.Index.Coverage,
		Languages:         s.Languages,
		ProfileTier:       s.Profile.Tier,
		Status:            raw,
	}
	if i.Files == 0 || i.Symbols == 0 {
		i.Shortfall = fmt.Sprintf("the scan of %s indexed %d files and %d symbols, so there is no index "+
			"to author against. This is the tool failing to run, not a repository with nothing in it, "+
			"and nothing downstream may read it as either", p.Checkout, i.Files, i.Symbols)
	}
	return i, nil
}

// Write puts a record where its phase declares it, creating the directory it
// belongs in. Both files admission leaves behind go through here — the index
// artifact and the repository file — so they are indented the same way and a
// person diffing one against last month's is reading a diff of the content.
func Write(path string, record any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare %s: %w", filepath.Dir(path), err)
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("render %s: %w", path, err)
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Names is the languages an index found, sorted. It is what the repository file
// records, and it comes from the scan rather than from a judgement about the
// repository: a vertical query selects on it, so a guess here is a repository
// that silently never matches.
func (i Index) Names() []string {
	out := make([]string, 0, len(i.Languages))
	for name := range i.Languages {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
