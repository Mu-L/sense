// Package judge grades an answer against the authored gold.
//
// It exists because a reference-blind judge lies, and the old tree has the
// number. Ranked by a blind composite weighted 0.55 on the judge's quality
// score, a +0.44 cited-recall win was reported as a −0.018 tie, and Sense
// appeared to tie on 12 of 13 repositories. Across the set the blind composite
// showed +0.017 where objective recall showed +0.28: a sixteenfold
// understatement, running toward the baseline every time.
//
// The mechanism is simple. A blind judge is omission-blind. It cannot know what
// is missing, so it grades what is present, and it rates a 60%-recall answer at
// about 0.84. An arm that names half the dependents confidently and well reads
// as excellent.
//
// What replaces it is reference-aware: the authored `relation` on each gold row
// says why that row belongs, and the judge grades the answer's claim against
// it. No second call, no code shipped to a model, and no opinion about what
// might be missing — completeness is recall against authored gold, and a judge
// that opines on it is inventing a reference.
//
// This package is pure. It renders the instruction, reads the reply, and does
// the arithmetic; sending it is lab/internal/judgerun's.
package judge

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// State is how the answer claimed one gold row.
//
// The three are exclusive by construction: a row carries one state, so
// `contradicted` can never also be `related`. That is the property the recorded
// failure needs — a baseline arm called a non-rendering extractor a "renderer",
// which is a confident false claim rather than a vague one, and a scheme that
// let it count as both would have absorbed it into the vague bucket.
type State string

const (
	// Covered: the row is claimed, and the stated relation matches the
	// authored one.
	Covered State = "covered"
	// Related: the row is claimed with a relation that is not the authored one
	// and is not contradicted by it. Vagueness lands here.
	Related State = "related"
	// Contradicted: the row is claimed with a relation that conflicts with the
	// authored one. Only a confident wrong relation is a contradiction.
	Contradicted State = "contradicted"
)

// Gold is one row of the reference: what belongs, and why.
//
// The type is the judge's own rather than the scenario package's, for the same
// reason the scorer keeps its own: this package takes data and returns numbers,
// and a type that reaches a disk would put the whole measurement one file read
// away from being unreproducible.
type Gold struct {
	ID string
	// Relation is the authored reason the row belongs. It is the ground truth
	// for the semantic layer, which is what makes this reference-aware rather
	// than an opinion.
	Relation string
}

// Claim is the judge's verdict on one gold row.
type Claim struct {
	ID    string `json:"id"`
	State State  `json:"state"`
	// Why is the judge's own sentence. It is carried so a contradiction can be
	// read months later without re-running anything.
	Why string `json:"why"`
}

// Verdict is the judge's whole reply: one claim per gold row it found a claim
// about. A row the answer said nothing about is absent, and absence is
// omission.
type Verdict struct {
	Claims []Claim `json:"claims"`
}

// Result is the reference-aware measurement of one answer against one gold
// group.
//
// There is no quality axis here, and there is not going to be one. The retired
// composite was not a bad idea implemented badly: it measured the wrong thing,
// and it favoured the arm that wrote well over the arm that found more.
type Result struct {
	// Total is the gold rows in the group.
	Total int
	// Covered, Related and Contradicted partition the rows the answer claimed.
	Covered      int
	Related      int
	Contradicted int
	// RelatedRecall is the share of gold rows the answer reached with a
	// relation that is at least not contradicted. A contradicted row is not
	// credited: naming the right place for the wrong reason is not reaching it.
	RelatedRecall float64
	// GroundedPrecision is 1 − contradicted / claimed.
	//
	// Silence is omission, not fabrication: an answer that claims nothing makes
	// no false claim, so its precision is 1 and its recall is 0. Vagueness lands
	// in Related and does not move this at all.
	GroundedPrecision float64
}

// Grade turns a judge's verdict on one gold group into the two numbers.
//
// It refuses a verdict it cannot trust rather than scoring around it: a claim
// about a row that is not in the group, a state that is not one of the three,
// or the same row claimed twice. Each of those means the judge answered a
// different question from the one asked, and a number computed from it would
// look exactly like a number computed from a good answer.
func Grade(rows []Gold, v Verdict) (Result, error) {
	known := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.ID == "" {
			return Result{}, fmt.Errorf("a gold row has no id")
		}
		known[r.ID] = true
	}

	seen := make(map[string]State, len(v.Claims))
	r := Result{Total: len(rows)}
	for _, c := range v.Claims {
		if !known[c.ID] {
			return Result{}, fmt.Errorf("the judge claimed %q, which is not in this gold group", c.ID)
		}
		if was, dup := seen[c.ID]; dup {
			return Result{}, fmt.Errorf("the judge claimed %q twice, as %s and %s; the three states are exclusive",
				c.ID, was, c.State)
		}
		switch c.State {
		case Covered:
			r.Covered++
		case Related:
			r.Related++
		case Contradicted:
			r.Contradicted++
		default:
			return Result{}, fmt.Errorf("the judge gave %q the state %q, which is not one of covered, related or contradicted",
				c.ID, c.State)
		}
		seen[c.ID] = c.State
	}

	claimed := r.Covered + r.Related + r.Contradicted
	if r.Total > 0 {
		r.RelatedRecall = float64(r.Covered+r.Related) / float64(r.Total)
	}
	r.GroundedPrecision = 1
	if claimed > 0 {
		r.GroundedPrecision = 1 - float64(r.Contradicted)/float64(claimed)
	}
	return r, nil
}

// Instruction is what the judge is asked, rendered from the authored gold and
// the answer under grading.
//
// Everything the judge needs is here and nothing else is: the rows, their
// authored relations, the answer, and the three states. It is never asked what
// might be missing, and it is never asked whether the answer is any good.
func Instruction(rows []Gold, answer string) string {
	var b strings.Builder
	b.WriteString(`You are grading one answer against a reference.

The reference below lists items that belong, each with the authored reason it
belongs. For every item the answer makes a claim about, decide which ONE of
these describes the claim:

  covered       the answer claims this item, and the reason it gives matches
                the authored reason
  related       the answer claims this item with a reason that is not the
                authored one, but does not conflict with it. A vague or
                unstated reason belongs here
  contradicted  the answer claims this item with a reason that CONFLICTS with
                the authored one. Only a confident wrong reason belongs here

Say nothing about items the answer does not claim. An item the answer is silent
about is simply absent from your reply; that is not your concern and it is not a
fault to record.

Reply with JSON only, in this shape:

{"claims":[{"id":"<item id>","state":"covered|related|contradicted","why":"<one sentence>"}]}

## The reference

`)
	for _, row := range rows {
		fmt.Fprintf(&b, "- %s: %s\n", row.ID, strings.TrimSpace(row.Relation))
	}
	b.WriteString("\n## The answer\n\n")
	b.WriteString(strings.TrimSpace(answer))
	b.WriteString("\n")
	return b.String()
}

// Parse reads the judge's reply.
//
// A model's reply carries prose around its JSON often enough that refusing one
// would throw away a paid call, so the first balanced JSON object is taken and
// the rest ignored. What is NOT tolerated is a reply with no object in it: an
// empty verdict is indistinguishable from an answer that claimed nothing, and
// those are opposite results.
func Parse(reply []byte) (Verdict, error) {
	body := firstObject(string(reply))
	if body == "" {
		return Verdict{}, fmt.Errorf("the judge's reply carries no JSON object")
	}
	var v Verdict
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return Verdict{}, fmt.Errorf("read the judge's reply: %w", err)
	}
	return v, nil
}

// firstObject returns the first balanced {...} in the text, ignoring braces
// inside strings.
func firstObject(text string) string {
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return ""
	}
	depth, inString, escaped := 0, false, false
	for i := start; i < len(text); i++ {
		c := text[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

// Contradictions lists the rows the judge marked contradicted, with its reason,
// in a stable order. It is what a diagnosis reads: a number that fell tells
// nobody which claim was wrong.
func Contradictions(v Verdict) []Claim {
	var out []Claim
	for _, c := range v.Claims {
		if c.State == Contradicted {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
