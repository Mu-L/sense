package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// budget caps how many nodes one file may expand to.
//
// Aliases are expanded, so the node graph is not a tree and its expansion is
// not bounded by the file's size: `a: &x [*x]` is a cycle that recurses until
// the stack dies, and ten chained `&a [*b,*b]` pairs expand to 2^10 copies
// without ever being a cycle. Both are anchor typos rather than attacks, but a
// stack overflow is runtime.throw — no panic, no recover, no test output, the
// whole process gone. One counter refuses both.
//
// The largest checked-in scenario expands to about nine thousand nodes, so this
// is three orders of magnitude of headroom.
const budget = 10_000_000

// maxDepth bounds the recursion separately, because the two bad shapes fail
// differently: a self-referential anchor is one node deep per frame, so it
// overflows the stack long before ten million nodes are counted, while an
// exponential chain is wide and shallow and only the node count catches it.
//
// The deepest checked-in scenario nests eight levels.
const maxDepth = 1000

var errTooBig = errors.New("expands to more nodes than any real scenario, which usually means an anchor refers to itself")

// Hash is the content hash of one scenario, gold or rubric file.
//
// It is taken over PARSED, canonicalized content and never over file bytes.
// Reformatting the YAML changes nothing; changing any value changes the cell.
// These files carry seventy-line prose headers by design, and byte hashing
// would mean a typo fix in one invalidates a banked cell.
//
// Keys are sorted. Values are preserved byte-exactly, because whitespace inside
// a prompt reaches the model: canonicalization is for key order and formatting,
// never for values.
//
// Only the FIRST YAML document in the file is hashed, because only the first is
// read: appending `---` and a second document changes neither the hash nor
// anything the agent, scorer or judge sees.
func Hash(b []byte) (string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return "", fmt.Errorf("parse for hashing: %w", err)
	}
	// Comments never reach the agent, the scorer or the judge, so they cannot
	// change a result. yaml.Node carries them and canon ignores them, which is
	// what makes a comment-only edit free.
	c := &canon{h: sha256.New(), left: budget}
	c.node(&doc)
	if c.over {
		return "", fmt.Errorf("hash: %w", errTooBig)
	}
	return hex.EncodeToString(c.h.Sum(nil)), nil
}

// canon writes nodes to h in a form that depends on values and structure and on
// nothing else.
//
// Writes are unchecked because a hash.Hash never fails one; that is its
// documented contract, and the field is hash.Hash rather than io.Writer so that
// this is true by construction rather than by comment.
type canon struct {
	h     hash.Hash
	left  int
	depth int
	over  bool
}

// node writes one node and its children.
//
// Every piece is length-prefixed so that two different shapes cannot serialize
// to the same bytes: without it, a mapping of one key "ab" and a sequence of
// "a" then "b" would hash alike.
func (c *canon) node(n *yaml.Node) {
	c.left--
	c.depth++
	defer func() { c.depth-- }()
	if c.left <= 0 || c.depth > maxDepth {
		c.over = true
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		c.mapping(n)
	case yaml.ScalarNode:
		// The tag goes in as well as the text: YAML's bare true is a bool and
		// its quoted form is a string, and a hash that could not tell them
		// apart would call two different scenarios the same.
		c.header("scalar:"+n.Tag, len(n.Value))
		_, _ = io.WriteString(c.h, n.Value)
	case yaml.AliasNode:
		// An anchor reused elsewhere in the file is the value it points at, so
		// a scenario that factors a repeated block into an anchor hashes the
		// same as one that spells it out twice. The budget above is what stops
		// an anchor that refers to itself from recursing forever.
		c.node(n.Alias)
	default:
		// Document and sequence, plus anything a future parser adds: the kind
		// and the child count go in, so two shapes cannot serialize alike.
		c.header("node:"+strconv.Itoa(int(n.Kind)), len(n.Content))
		for _, kid := range n.Content {
			c.node(kid)
		}
	}
}

// mapping writes a mapping with its keys in sorted order, so that moving a key
// up or down a file changes nothing.
func (c *canon) mapping(n *yaml.Node) {
	type pair struct{ k, v *yaml.Node }
	pairs := make([]pair, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		pairs = append(pairs, pair{n.Content[i], n.Content[i+1]})
	}
	slices.SortStableFunc(pairs, func(a, b pair) int { return strings.Compare(a.k.Value, b.k.Value) })

	c.header("map", len(pairs))
	for _, p := range pairs {
		c.node(p.k)
		c.node(p.v)
	}
}

// header writes a length-prefixed marker. Named for what it writes, not for
// yaml.Node's Tag field, which is a different thing that also goes in.
func (c *canon) header(kind string, n int) {
	_, _ = io.WriteString(c.h, kind+":"+strconv.Itoa(n)+"\x00")
}
