package scenario

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func hashOf(t *testing.T, y string) string {
	t.Helper()
	h, err := Hash([]byte(y))
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	return h
}

// The hash is over parsed content, never file bytes. These files carry
// seventy-line prose headers by design, and byte hashing would mean a typo fix
// in one invalidates a banked cell.
func TestReformattingTheYAMLDoesNotChangeTheHash(t *testing.T) {
	const original = `name: audit
repo: discourse
steps:
  - name: Map
    prompt: Trace the path.
`
	for _, tc := range []struct{ name, y string }{
		{
			name: "keys in a different order",
			y: `repo: discourse
steps:
  - name: Map
    prompt: Trace the path.
name: audit
`,
		},
		{
			name: "different indentation",
			y: `name: audit
repo: discourse
steps:
    -   name: Map
        prompt: Trace the path.
`,
		},
		{
			name: "flow style instead of block",
			y:    `{name: audit, repo: discourse, steps: [{name: Map, prompt: Trace the path.}]}` + "\n",
		},
		{
			name: "quoted scalars",
			y: `name: "audit"
repo: 'discourse'
steps:
  - name: "Map"
    prompt: "Trace the path."
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if hashOf(t, tc.y) != hashOf(t, original) {
				t.Error("reformatting changed the hash, so a tidy-up would invalidate a banked cell")
			}
		})
	}
}

// Comments never reach the agent, the scorer or the judge, so they cannot
// change a result. The seventy-line headers explaining why an attempt exists
// are the most valuable prose in the tree, and they must be free to write.
func TestACommentOnlyEditDoesNotChangeTheHash(t *testing.T) {
	const bare = `name: audit
steps:
  - name: Map
    prompt: Trace the path.
`
	const commented = `# WHY THIS ATTEMPT EXISTS
# The previous one measured baseline 0.625, which capped the delta at +0.375
# before the pair ran. This one moves the anchor and keeps the question.
name: audit # the ask, not the inventory
steps:
  - name: Map
    prompt: Trace the path. # richness floor 8
`
	if hashOf(t, bare) != hashOf(t, commented) {
		t.Error("a comment changed the hash, so writing down why an attempt exists would cost a re-run")
	}
}

// And the other half: changing any value changes the cell.
func TestChangingAnyValueChangesTheHash(t *testing.T) {
	const original = `name: audit
repo: discourse
steps:
  - name: Map
    prompt: Trace the path.
`
	for _, tc := range []struct{ name, y string }{
		{"a different name", strings.Replace(original, "audit", "audit v2", 1)},
		{"a different repo", strings.Replace(original, "discourse", "mastodon", 1)},
		{"one word of a prompt", strings.Replace(original, "Trace the path.", "Trace the paths.", 1)},
		{"a step added", original + "  - name: Audit\n    prompt: Find them.\n"},
		{"a key added", original + "contract_symbol: Category\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if hashOf(t, tc.y) == hashOf(t, original) {
				t.Error("an edit to a value left the hash alone, so a changed scenario would score as the same cell")
			}
		})
	}
}

// Whitespace inside a prompt reaches the model, so it is content. Trimming it
// is not canonicalization, it is editing what the agent sees.
func TestWhitespaceInsideAPromptIsContent(t *testing.T) {
	a := "steps:\n  - prompt: |\n      Trace the path.\n      Then audit it.\n"
	b := "steps:\n  - prompt: |\n      Trace the path.\n\n      Then audit it.\n"

	if hashOf(t, a) == hashOf(t, b) {
		t.Error("a blank line inside a prompt did not change the hash; the model sees that line")
	}
}

// Two shapes that serialize to similar text must not hash alike. Without
// length prefixes a mapping of one key and a two-element sequence collide.
func TestDifferentShapesWithTheSameTextDoNotCollide(t *testing.T) {
	mapping := hashOf(t, "a: b\n")
	sequence := hashOf(t, "- a\n- b\n")

	if mapping == sequence {
		t.Error("a mapping and a sequence hashed alike")
	}
}

// The scenarios carry `required: true` on their checks, and a hash that could
// not tell a bool from its quoted form would call two different scenarios the
// same.
//
// Bare `yes` is NOT a bool under YAML 1.2, which this parser follows, so the
// obvious example for this test is the wrong one and was tried first.
func TestATypedScalarDoesNotHashLikeItsQuotedForm(t *testing.T) {
	if hashOf(t, "required: true\n") == hashOf(t, "required: 'true'\n") {
		t.Error("a bool and a string hashed alike")
	}
	if hashOf(t, "value: 3\n") == hashOf(t, "value: '3'\n") {
		t.Error("an int and a string hashed alike")
	}
}

func TestSomethingThatIsNotYAMLCannotBeHashed(t *testing.T) {
	if _, err := Hash([]byte("steps: [oops\n")); err == nil {
		t.Error("hashing malformed YAML succeeded")
	}
}

// An anchor reused elsewhere is the value it points at, so a scenario that
// factors a repeated block into an anchor hashes the same as one that spells it
// out twice. Otherwise a tidy-up that introduced an anchor would invalidate a
// banked cell.
func TestAnAnchorHashesAsTheValueItPointsAt(t *testing.T) {
	anchored := `common: &c
  weight: 1.0
a: *c
b: *c
`
	spelled := `common:
  weight: 1.0
a:
  weight: 1.0
b:
  weight: 1.0
`
	if hashOf(t, anchored) != hashOf(t, spelled) {
		t.Error("an anchor hashed differently from the value it points at")
	}
}

// The encoding has to be injective: two different documents must never produce
// the same bytes, or two different scenarios score as one cell and nothing says
// so. Every piece is length-prefixed and every scalar carries its type tag,
// which is what these pairs check.
func TestTwoDifferentDocumentsNeverHashAlike(t *testing.T) {
	for _, tc := range []struct {
		name, a, b string
	}{
		{"a mapping and a sequence", "a: b\n", "- a\n- b\n"},
		{"the same characters split differently", "a: bc\n", "ab: c\n"},
		{"a nested mapping and a nested sequence", "a:\n  b: c\n", "a:\n  - b\n  - c\n"},
		{"one scalar and two", "k: ab\n", "k:\n  - a\n  - b\n"},
		{"a newline inside a value", "x: \"a\\nb\"\n", "x: \"ab\"\n"},
		{"empty string and null", "a: ''\n", "a: null\n"},
		{"empty string and absent value", "a: ''\n", "a:\n"},
		// Sequences are ORDERED, unlike mappings. Two steps swapped is a
		// different session, and the agent is asked them in order.
		{"two steps in the other order", "steps:\n  - p: a\n  - p: b\n", "steps:\n  - p: b\n  - p: a\n"},
		{"two values swapped between keys", "a: 1\nb: 2\n", "a: 2\nb: 1\n"},
		{"an empty mapping and an empty sequence", "a: {}\n", "a: []\n"},
		{"a float and an int", "n: 1.0\n", "n: 1\n"},
		// Block chomping decides the trailing newline, and that newline is
		// inside a prompt the model reads.
		{"a block scalar chomped differently", "s: |\n  a\n", "s: |-\n  a\n"},
		// Same length, so the prefix cannot separate them: only the bytes can.
		// This is what stops a well-meant TrimSpace on a value, which would
		// silently edit what the agent is shown.
		{"whitespace at the front rather than the back", "p: \"a \"\n", "p: \" a\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if hashOf(t, tc.a) == hashOf(t, tc.b) {
				t.Errorf("%q and %q hashed alike", tc.a, tc.b)
			}
		})
	}
}

// An anchor that refers to itself used to take the whole process down: not a
// panic a test could catch but runtime.throw, no recover, no output. A typo in
// a hand-written scenario file must cost an error message.
func TestAnAnchorThatRefersToItselfIsRefusedNotFatal(t *testing.T) {
	if _, err := Hash([]byte("a: &x [*x]\n")); err == nil {
		t.Error("a self-referential anchor hashed successfully")
	}
}

// The same guard, in the shape that is not a cycle and so a cycle detector
// would miss: each anchor names the one above it twice, so the expansion
// doubles per level and the hash hangs rather than crashing.
func TestAnchorsThatDoubleEachLevelAreRefused(t *testing.T) {
	var b strings.Builder
	b.WriteString("a0: &a0 x\n")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "a%d: &a%d [*a%d, *a%d]\n", i, i, i-1, i-1)
	}
	if _, err := Hash([]byte(b.String())); err == nil {
		t.Error("an exponential anchor chain hashed successfully")
	}
}

// And the other side of that guard: the real scenarios must be nowhere near it,
// or a legitimate file would start failing as the corpus grows.
func TestAScenarioOfRealisticSizeIsFarInsideTheBudget(t *testing.T) {
	// The budget bounds how much of a document the canonical hash will walk,
	// and it exists so a pathological file cannot make hashing unbounded. What
	// it must never do is bite a real scenario: the ones this instrument was
	// built against used well under a thousandth of it, and the shape below is
	// theirs — seven steps of long prose, forty gold rows, each with the
	// relation paragraph the scorer reads a location out of.
	var b strings.Builder
	b.WriteString("name: a work session\nrepo: r\ncontract_symbol: Thing\ndescription: >\n")
	b.WriteString("  " + strings.Repeat("a sentence about the ticket. ", 60) + "\nsteps:\n")
	for i := 0; i < 7; i++ {
		fmt.Fprintf(&b, "  - name: step %d\n    prompt: >\n      %s\n",
			i+1, strings.Repeat("a paragraph of instructions for this step. ", 40))
	}
	b.WriteString("rows:\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "  - id: r%d\n    group: dependents\n    relation: \"app/models/thing.rb:%d %s\"\n",
			i, i*13+1, strings.Repeat("why this location matters. ", 12))
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(b.String()), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	c := &canon{h: sha256.New(), left: budget}
	c.node(&doc)

	if used := budget - c.left; used > budget/1000 {
		t.Errorf("a scenario of realistic size used %d nodes of %d; the headroom is gone", used, budget)
	}
}

// The pairs above are plausible ones, and injectivity is not a plausibility
// property: with the length prefixes deleted the whole table above still
// passes. These two are adversarial, and they are what makes the doc comment on
// canonical true rather than asserted.
func TestTheLengthPrefixesActuallyCarryTheirWeight(t *testing.T) {
	// A value that spells out the encoder's own delimiter text. Without the
	// lengths, the stream re-parses: one mapping of two pairs serializes to the
	// same bytes as one mapping whose single value contains the separators.
	t.Run("a value containing the encoders own delimiters", func(t *testing.T) {
		two := "a: b\nc: d\n"
		one := "a: \"bscalar:!!strcscalar:!!strd\"\n"
		if hashOf(t, two) == hashOf(t, one) {
			t.Error("a value spelling out the delimiters collided with the structure it imitates")
		}
	})
	// Same elements, same order, different grouping. Only the child COUNT
	// separates them, so zeroing that count alone collides these two.
	t.Run("the same elements grouped differently", func(t *testing.T) {
		if hashOf(t, "- - x\n- y\n") == hashOf(t, "- - x\n  - y\n") {
			t.Error("two different sequence groupings hashed alike")
		}
	})
}

// encoded returns the exact byte stream canon writes, so a test can assert the
// format itself rather than a property the format is supposed to imply.
type recorder struct{ b strings.Builder }

func (r *recorder) Write(p []byte) (int, error) { return r.b.Write(p) }
func (r *recorder) Sum(b []byte) []byte         { return b }
func (r *recorder) Reset()                      {}
func (r *recorder) Size() int                   { return 0 }
func (r *recorder) BlockSize() int              { return 1 }

func encoded(t *testing.T, y string) string {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(y), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := &recorder{}
	c := &canon{h: r, left: budget}
	c.node(&doc)
	return strings.ReplaceAll(r.b.String(), "\x00", "|")
}

// The encoding is pinned byte for byte, because every banked cell is a hash of
// it: changing the format silently re-keys the whole corpus, and no property
// test catches that — a different-but-still-injective encoding passes them all.
//
// This is also what makes the length prefixes testable. Injectivity tests can
// only ever check pairs someone thought of, and deleting every prefix leaves a
// table of plausible pairs completely green.
func TestTheEncodingIsExactlyThis(t *testing.T) {
	for _, tc := range []struct{ name, y, want string }{
		{
			name: "a mapping of two pairs",
			y:    "a: b\nc: d\n",
			want: "node:1:1|map:2|scalar:!!str:1|ascalar:!!str:1|bscalar:!!str:1|cscalar:!!str:1|d",
		},
		{
			name: "keys sorted, not in file order",
			y:    "c: d\na: b\n",
			want: "node:1:1|map:2|scalar:!!str:1|ascalar:!!str:1|bscalar:!!str:1|cscalar:!!str:1|d",
		},
		{
			name: "a nested sequence keeps its grouping",
			y:    "- - x\n- y\n",
			want: "node:1:1|node:2:2|node:2:1|scalar:!!str:1|xscalar:!!str:1|y",
		},
		{
			name: "a typed scalar carries its tag",
			y:    "n: 1.0\n",
			want: "node:1:1|map:1|scalar:!!str:1|nscalar:!!float:3|1.0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := encoded(t, tc.y); got != tc.want {
				t.Errorf("encoding changed, which re-keys every banked cell\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}
