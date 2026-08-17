// Package mine reads runs that already happened and reports where Sense was
// called and came up short.
//
// A margin says Sense helped. It does not say what to fix, and a bench that
// produces numbers with no route back to a product surface is a bench that banks
// wins and teaches the product nothing.
//
// # Post-run only, and structurally so
//
// Every detector here takes a [Completed] run, and the only way to build one is
// to hand over the terminal outcome the run recorded. A run that never reached a
// terminal state has no outcome to pass, because reading its directory refuses.
// So the miner cannot be consulted before spending, and therefore cannot become
// a screen.
//
// That is not caution. Two things in the retired tree were called an oracle and
// both failed. The correctness oracle scored a citation as on-target when the
// audited token appeared on the pinned line, and inverted the arms: 67.4%
// baseline against 48.4% Sense over 10 transcripts, because it was scoring "did
// you find this by grepping for that token". The resolution oracle asked Sense a
// question the gold already answers, and on one repository, one anchor, 33
// minutes apart, it reported 16 of 16 gold dependents resolved and zero failures
// while the miner over the paid runs reported six cited-not-returned rows, four
// of them in the discriminator group.
//
// The oracle asks Sense a question in the words the gold is written in. The run
// asks whatever the agent actually asks.
//
// # Three detectors, and each has a real example
//
// Cited-not-returned, nondeterministic returns and empty returns. Four more are
// described in the retired tree — abandoned-on-empty, ignored-hint,
// contract-misled and wrong-tool-shape — and are deliberately absent: their
// catches are described rather than shown, and a detector with no real example
// does not ship. They arrive when a transcript demands one, with that transcript
// as the fixture.
//
// # The miner reports, it does not interpret
//
// "Blast was called and did not return this file" is a fact. "Blast has a
// resolver bug" is a hypothesis, and hypotheses belong to a human with the
// transcript open. Nothing here scores or ranks a finding either: a severity
// number invites treating the top of a list as the work queue, and the
// interesting finding is often the rare one.
package mine

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Surface is the Sense surface a finding names. Every finding names one: the
// route from a number to a fix runs through a surface, and a finding that names
// none is a number again.
type Surface string

// The query surfaces, and the meta-surfaces where misuse is born.
const (
	Graph       Surface = "graph"
	Blast       Surface = "blast"
	Search      Surface = "search"
	Conventions Surface = "conventions"
	Status      Surface = "status"

	Setup         Surface = "setup and onboarding"
	Contracts     Surface = "tool contracts and schemas"
	ResponseShape Surface = "response shape"
)

// surfaces maps a tool to the surface it belongs to. A call to a tool that is
// not here yields no surface and therefore no finding: a finding with no surface
// is unrepresentable rather than merely discouraged.
var surfaces = map[string]Surface{
	"sense_graph":       Graph,
	"sense_blast":       Blast,
	"sense_search":      Search,
	"sense_conventions": Conventions,
	"sense_status":      Status,
}

// resolvers are the surfaces that answer "which files touch this". Only these
// can miss a file the agent then cites; a search that does not return a file is
// not a resolver miss.
var resolvers = []Surface{Blast, Graph}

// Call is one MCP call and what came back.
type Call struct {
	// Tool is the tool the agent asked for.
	Tool string
	// Key is what the call was about: the symbol, or the query. It is what a
	// determinism check groups on, because the same symbol asked twice is the
	// comparison, and the same symbol asked with different options is still the
	// same symbol.
	Key string
	// Args is the argument object exactly as the agent sent it. Two calls with
	// the same key and different args are how a parameter turns out to matter.
	Args string
	// Returned is every repository path the reply named, deduplicated and
	// sorted.
	Returned []string
}

// Surface is the surface this call reached, and whether it reached one at all.
func (c Call) Surface() (Surface, bool) {
	s, ok := surfaces[c.Tool]
	return s, ok
}

// Cited is one gold row the answer cited.
//
// The miner is handed these rather than deriving them: matching an answer
// against gold is the scorer's job, and a miner that re-implemented it would
// report misses against a second, differently wrong matcher.
type Cited struct {
	ID    string
	Group string
	Path  string
	// Discriminator says this row is in the group that carries the margin. A
	// miss there is worth more than a miss elsewhere.
	Discriminator bool
}

// Completed is one run that reached a terminal state.
//
// Its fields are unexported and [Complete] is the only way to build one, which
// is what makes "no detector runs before a run finishes" a property of the type
// rather than a rule someone has to remember. The outcome is required because a
// run with no terminal record has none to give: reading such a directory refuses
// rather than returning a blank one.
type Completed struct {
	id    string
	calls []Call
	cited []Cited
}

// Complete records a finished run for mining.
//
// The outcome is checked and not kept. It is not data the miner uses: it is the
// proof that a run happened, and demanding it is what makes a pre-run detector
// impossible to write against this type.
func Complete(id, outcome string, calls []Call, cited []Cited) (Completed, error) {
	if id == "" {
		return Completed{}, fmt.Errorf("a run with no id; a finding that cannot name its run cannot be checked")
	}
	if outcome == "" {
		return Completed{}, fmt.Errorf("run %s reached no terminal state, so there is nothing to mine: detection is post-run", id)
	}
	return Completed{id: id, calls: calls, cited: cited}, nil
}

// Finding is one thing the miner saw. It carries no score and no rank.
type Finding struct {
	Detector string
	Surface  Surface
	// Subject is what the finding is about: a gold row's id, or a tool and the
	// symbol it was asked for.
	Subject string
	Detail  string
	// Runs is how many runs showed it, out of Total. The denominator travels
	// with the numerator: three runs out of five and three out of twenty are
	// different findings.
	Runs  int
	Total int
	// Discriminator marks a miss on the group that carries the margin.
	Discriminator bool
}

func (f Finding) String() string {
	mark := " "
	if f.Discriminator {
		mark = "*"
	}
	return fmt.Sprintf("%s %-24s %d/%d runs  [%s] %s", mark, f.Subject, f.Runs, f.Total, f.Surface, f.Detail)
}

// Findings runs every detector over a set of runs.
func Findings(runs []Completed) []Finding {
	var out []Finding
	out = append(out, CitedNotReturned(runs)...)
	out = append(out, Nondeterministic(runs)...)
	out = append(out, EmptyReturns(runs)...)
	return out
}

// CitedNotReturned reports gold rows the answer cited that a resolver was asked
// for and never returned.
//
// The condition is precise, and each half of it matters. The row was CITED, so
// the agent got there somehow. A resolver WAS called in that run, so the tool had
// its chance. And the file never appeared in any resolver reply, so it got there
// without the tool.
func CitedNotReturned(runs []Completed) []Finding {
	type miss struct {
		row  Cited
		runs int
	}
	seen := map[string]*miss{}
	var order []string
	for _, r := range runs {
		returned, asked := r.resolverReturns()
		if !asked {
			// No resolver was called, so nothing could have missed. Counting
			// this run would report the agent's choice as a tool defect.
			continue
		}
		for _, row := range r.cited {
			if returnedIt(returned, row.Path) {
				continue
			}
			if _, ok := seen[row.ID]; !ok {
				seen[row.ID] = &miss{row: row}
				order = append(order, row.ID)
			}
			seen[row.ID].runs++
		}
	}

	total := len(runs)
	var out []Finding
	for _, id := range order {
		m := seen[id]
		out = append(out, Finding{
			Detector:      "cited-not-returned",
			Surface:       Blast,
			Subject:       m.row.ID,
			Detail:        fmt.Sprintf("%s, in %s, was cited and never returned by a resolver", m.row.Path, m.row.Group),
			Runs:          m.runs,
			Total:         total,
			Discriminator: m.row.Discriminator,
		})
	}
	return out
}

// resolverReturns collects every path a resolver returned in this run, and says
// whether a resolver was called at all.
func (c Completed) resolverReturns() (map[string]bool, bool) {
	returned := map[string]bool{}
	var asked bool
	for _, call := range c.calls {
		s, ok := call.Surface()
		if !ok || !slices.Contains(resolvers, s) {
			continue
		}
		asked = true
		for _, path := range call.Returned {
			returned[path] = true
		}
	}
	return returned, asked
}

// returnedIt reports whether a resolver returned the gold row's file.
//
// The gold's match is a path fragment, because gold is authored against the
// part of a path that identifies the file rather than against a full path that a
// repository layout change would break.
func returnedIt(returned map[string]bool, path string) bool {
	if path == "" {
		return false
	}
	for got := range returned {
		if strings.Contains(got, path) {
			return true
		}
	}
	return false
}

// Nondeterministic reports a symbol whose resolver returned different numbers of
// files across runs.
//
// It counts distinct files rather than rows, because that is what the answer can
// use: a reply that returns the same twelve files in a different shape is not
// the finding this is looking for.
func Nondeterministic(runs []Completed) []Finding {
	counts, order := gatherCounts(runs)
	var out []Finding
	for _, key := range order {
		got := counts[key]
		if len(got) < 2 {
			continue
		}
		slices.Sort(got)
		tool, sym, _ := strings.Cut(key, ":")
		out = append(out, Finding{
			Detector: "nondeterministic-returns",
			Surface:  surfaces[tool],
			Subject:  key,
			Detail:   fmt.Sprintf("%s returned %v files across runs", sym, got),
			Runs:     len(got),
			Total:    len(runs),
		})
	}
	return out
}

// gatherCounts collects the distinct return sizes per tool and symbol, in the
// order the symbols were first asked for.
func gatherCounts(runs []Completed) (map[string][]int, []string) {
	counts := map[string][]int{}
	var order []string
	for _, r := range runs {
		for _, call := range r.calls {
			s, ok := call.Surface()
			if !ok || !slices.Contains(resolvers, s) {
				continue
			}
			key := call.Tool + ":" + call.Key
			if _, seen := counts[key]; !seen {
				order = append(order, key)
			}
			if n := len(call.Returned); !slices.Contains(counts[key], n) {
				counts[key] = append(counts[key], n)
			}
		}
	}
	return counts, order
}

// EmptyReturns reports a resolver call that returned nothing.
//
// It groups on the arguments rather than the symbol: an empty return that
// happens only with one option set is a different finding from one that happens
// however the call is made, and folding them together loses which.
func EmptyReturns(runs []Completed) []Finding {
	seen := map[string]int{}
	subject := map[string]Call{}
	var order []string
	for _, r := range runs {
		for _, call := range r.calls {
			s, ok := call.Surface()
			if !ok || !slices.Contains(resolvers, s) || len(call.Returned) > 0 {
				continue
			}
			key := call.Tool + ":" + call.Args
			if _, ok := seen[key]; !ok {
				order = append(order, key)
				subject[key] = call
			}
			seen[key]++
		}
	}

	var out []Finding
	for _, key := range order {
		call := subject[key]
		s, _ := call.Surface()
		out = append(out, Finding{
			Detector: "empty-returns",
			Surface:  s,
			Subject:  call.Tool + ":" + call.Key,
			Detail:   fmt.Sprintf("returned nothing for %s", call.Args),
			Runs:     seen[key],
			Total:    len(runs),
		})
	}
	return out
}

// Exercise is how hard one surface was pressed.
type Exercise struct {
	Surface Surface
	Calls   int
	Runs    int
	// Params is every distinct argument object the surface was called with, so
	// an unexercised option is visible rather than merely absent from a count.
	Params []string
}

// Coverage reports which surfaces the runs actually exercised.
//
// It shares its input with the detectors and nothing else. A coverage report
// derived from what the detectors happened to inspect would answer "what did we
// look at" while appearing to answer "what was exercised", and the difference
// only shows up as a surface nobody improves.
//
// Every declared surface appears, including the ones with a zero, because an
// unexercised surface is an unimproved surface and a row that is missing reads
// as a row that is fine.
func Coverage(runs []Completed) []Exercise {
	calls := map[Surface]int{}
	inRuns := map[Surface]map[string]bool{}
	params := map[Surface]map[string]bool{}
	for _, r := range runs {
		for _, call := range r.calls {
			s, ok := call.Surface()
			if !ok {
				continue
			}
			calls[s]++
			add(inRuns, s, r.id)
			add(params, s, call.Args)
		}
	}

	var out []Exercise
	for _, s := range AllSurfaces() {
		out = append(out, Exercise{
			Surface: s, Calls: calls[s], Runs: len(inRuns[s]), Params: sortedKeys(params[s]),
		})
	}
	return out
}

// AllSurfaces is every surface a finding may name, query surfaces first.
func AllSurfaces() []Surface {
	return []Surface{Graph, Blast, Search, Conventions, Status, Setup, Contracts, ResponseShape}
}

func add(m map[Surface]map[string]bool, s Surface, v string) {
	if m[s] == nil {
		m[s] = map[string]bool{}
	}
	m[s][v] = true
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Capture reads a recorded MCP capture into the calls it holds.
//
// The capture is newline-delimited JSON, one frame per line, in both directions.
// A line that cannot be read is skipped rather than fatal: the capture is
// telemetry that was written beside a paid run, and refusing to mine a run
// because one frame was truncated would throw away the run's whole value.
func Capture(log []byte) []Call {
	pending := map[string]params{}
	var out []Call
	for _, line := range strings.Split(string(log), "\n") {
		var e frame
		if line == "" || json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		var msg message
		if json.Unmarshal(e.Msg, &msg) != nil {
			continue
		}
		switch {
		case e.Dir == "c2s" && msg.Method == "tools/call":
			pending[string(msg.ID)] = msg.Params
		case e.Dir == "s2c" && len(msg.Result) > 0:
			p, ok := pending[string(msg.ID)]
			if !ok {
				continue
			}
			delete(pending, string(msg.ID))
			out = append(out, call(p, msg.Result))
		}
	}
	return out
}

type frame struct {
	Dir string          `json:"dir"`
	Msg json.RawMessage `json:"msg"`
}

type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params params          `json:"params"`
	Result json.RawMessage `json:"result"`
}

type params struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"arguments"`
}

// call pairs a request with its reply.
func call(p params, result json.RawMessage) Call {
	c := Call{Tool: p.Name, Args: string(p.Args), Key: keyOf(p.Args)}
	c.Returned = sortedKeys(pathsIn(result))
	return c
}

// keyOf is what the call was about. A call with neither a symbol nor a query is
// keyed on its arguments, so it still groups with an identical one.
func keyOf(args json.RawMessage) string {
	var a struct {
		Symbol string `json:"symbol"`
		Query  string `json:"query"`
	}
	if json.Unmarshal(args, &a) == nil {
		if a.Symbol != "" {
			return a.Symbol
		}
		if a.Query != "" {
			return a.Query
		}
	}
	return string(args)
}

// pathsIn collects every repository path a reply named.
//
// Sense writes a location as `file` or as `ref`, which carries a line, and both
// appear at every depth of a reply. Walking for the keys rather than decoding
// each tool's own shape is what keeps this from needing a change every time a
// reply grows a field.
func pathsIn(result json.RawMessage) map[string]bool {
	var reply struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	found := map[string]bool{}
	if json.Unmarshal(result, &reply) != nil {
		return found
	}
	for _, part := range reply.Content {
		if part.Type != "text" {
			continue
		}
		var body any
		if json.Unmarshal([]byte(part.Text), &body) != nil {
			continue
		}
		collect(body, found)
	}
	return found
}

func collect(node any, into map[string]bool) {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			if s, ok := v.(string); ok && (k == "file" || k == "ref") {
				into[strings.SplitN(s, ":", 2)[0]] = true
			}
			collect(v, into)
		}
	case []any:
		for _, v := range n {
			collect(v, into)
		}
	}
}
