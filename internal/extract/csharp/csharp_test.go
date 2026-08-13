package csharp

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/model"
)

// rec is the recording emitter. It implements only the core Emitter — the
// C# lane's three shapes are all edges, so no optional extension is needed
// to assert them.
type rec struct {
	symbols []extract.EmittedSymbol
	edges   []extract.EmittedEdge
}

func (r *rec) Symbol(s extract.EmittedSymbol) error {
	r.symbols = append(r.symbols, s)
	return nil
}

func (r *rec) Edge(e extract.EmittedEdge) error {
	r.edges = append(r.edges, e)
	return nil
}

func parse(t *testing.T, src string) *sitter.Tree {
	t.Helper()
	parser := sitter.NewParser()
	t.Cleanup(parser.Close)
	if err := parser.SetLanguage(Extractor{}.Grammar()); err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}
	tree := parser.Parse([]byte(src), nil)
	t.Cleanup(tree.Close)
	return tree
}

func mustRun(t *testing.T, src string) *rec {
	t.Helper()
	em := &rec{}
	if err := (Extractor{}).Extract(parse(t, src), []byte(src), "f.cs", em); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return em
}

// edge finds the edge of kind from source to target, failing with the full
// edge set when it is absent. Asserting the relationship, never a count.
func (r *rec) edge(t *testing.T, kind model.EdgeKind, source, target string) extract.EmittedEdge {
	t.Helper()
	for _, e := range r.edges {
		if e.Kind == kind && e.SourceQualified == source && e.TargetQualified == target {
			return e
		}
	}
	t.Fatalf("no %s edge %s -> %s in %v", kind, source, target, r.edges)
	return extract.EmittedEdge{}
}

func (r *rec) hasEdgeTarget(target string) bool {
	for _, e := range r.edges {
		if e.TargetQualified == target {
			return true
		}
	}
	return false
}

func TestExtractorContract(t *testing.T) {
	ex := Extractor{}
	if ex.Language() != "csharp" {
		t.Errorf("Language = %q", ex.Language())
	}
	if got := ex.Extensions(); len(got) != 1 || got[0] != ".cs" {
		t.Errorf("Extensions = %v", got)
	}
	if ex.Tier() != extract.TierStandard {
		t.Errorf("Tier = %q", ex.Tier())
	}
	if ex.Grammar() == nil {
		t.Error("Grammar returned nil")
	}
	if err := ex.Extract(nil, nil, "f.cs", &rec{}); err != nil {
		t.Errorf("nil tree: %v", err)
	}
}
