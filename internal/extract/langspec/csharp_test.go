package langspec

import (
	"errors"
	"testing"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/model"
)

// edgeEmitter records edges alongside the harvest streams harvestEmitter
// already captures, so a test can assert what the registered extractor emits.
type edgeEmitter struct {
	harvestEmitter
	edges []extract.EmittedEdge
}

func (e *edgeEmitter) Edge(ed extract.EmittedEdge) error {
	e.edges = append(e.edges, ed)
	return nil
}

// The registered C# extractor runs both passes: the generic table for symbols,
// inheritance and usings, and the dedicated walker for the holder edge and the
// typed receiver. Neither half is reachable through the registry alone, so the
// composition is what this asserts.
func TestCSharpRegisteredExtractorRunsBothPasses(t *testing.T) {
	src := `using System;

namespace Api
{
    public class PoliciesController : Controller
    {
        private readonly IPolicyQuery _policyQuery;

        public void Get()
        {
            _policyQuery.RunAsync();
        }
    }
}`
	em := &edgeEmitter{}
	runReal(t, "csharp", ".cs", src, em)

	var names []string
	for _, s := range em.symbols {
		names = append(names, s.Qualified)
	}
	for _, want := range []string{"Api", "Api.PoliciesController", "Api.PoliciesController.Get"} {
		if !contains(names, want) {
			t.Errorf("generic pass lost symbol %q, got %v", want, names)
		}
	}

	want := []extract.EmittedEdge{
		{SourceQualified: "Api.PoliciesController", TargetQualified: "Controller", Kind: model.EdgeInherits},
		{SourceQualified: "Api.PoliciesController", TargetQualified: "IPolicyQuery", Kind: model.EdgeComposes},
		{SourceQualified: "Api.PoliciesController.Get", TargetQualified: "IPolicyQuery.RunAsync", Kind: model.EdgeCalls},
		{SourceQualified: "", TargetQualified: "System", Kind: model.EdgeImports},
	}
	for _, w := range want {
		if !hasEdge(em.edges, w) {
			t.Errorf("missing %s edge %q -> %q in %v", w.Kind, w.SourceQualified, w.TargetQualified, em.edges)
		}
	}
}

// The registered extractor reports C#'s identity, not the wrapper's own.
func TestCSharpRegisteredExtractorIdentity(t *testing.T) {
	ex := extract.ForExtension(".cs")
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
	if err := ex.Extract(nil, nil, "f.cs", &edgeEmitter{}); err != nil {
		t.Errorf("nil tree: %v", err)
	}
}

// symbolErrEmitter fails on the first symbol, which only the generic pass
// emits — the walker that follows never runs.
type symbolErrEmitter struct{ edgeEmitter }

var errSymbol = errors.New("symbol write failed")

func (symbolErrEmitter) Symbol(extract.EmittedSymbol) error { return errSymbol }

// A failure in the generic pass aborts before the C# walker runs, so the scan
// harness sees the first error rather than a file half-written by two passes.
func TestCSharpGenericPassErrorStopsTheWalker(t *testing.T) {
	src := `class C { private IFoo _foo; }`
	ex := extract.ForExtension(".cs")
	tree := parse(t, ex.Grammar(), src)
	em := &symbolErrEmitter{}
	if err := ex.Extract(tree, []byte(src), "f.cs", em); !errors.Is(err, errSymbol) {
		t.Fatalf("Extract err = %v, want errSymbol", err)
	}
	if len(em.edges) != 0 {
		t.Errorf("walker ran after the generic pass failed: %v", em.edges)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func hasEdge(edges []extract.EmittedEdge, want extract.EmittedEdge) bool {
	for _, e := range edges {
		if e.Kind == want.Kind && e.SourceQualified == want.SourceQualified &&
			e.TargetQualified == want.TargetQualified {
			return true
		}
	}
	return false
}
