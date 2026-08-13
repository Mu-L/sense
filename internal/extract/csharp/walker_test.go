package csharp

import (
	"errors"
	"testing"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/model"
)

// noEdge asserts the emitter produced no edge of kind from source to target.
func (r *rec) noEdge(t *testing.T, kind model.EdgeKind, source, target string) {
	t.Helper()
	for _, e := range r.edges {
		if e.Kind == kind && e.SourceQualified == source && e.TargetQualified == target {
			t.Fatalf("unexpected %s edge %s -> %s in %v", kind, source, target, r.edges)
		}
	}
}

// A namespace is a scope both passes qualify through, so an edge emitted here
// binds to the symbol langspec emitted for the same class.
func TestNamespaceQualifiesBothEndsOfAnEdge(t *testing.T) {
	em := mustRun(t, `
namespace Api.Controllers
{
    public class PoliciesController
    {
        private readonly IPolicyQuery _policyQuery;

        public void Get()
        {
            _policyQuery.RunAsync();
        }
    }
}
`)
	em.edge(t, model.EdgeComposes, "Api.Controllers.PoliciesController", "IPolicyQuery")
	em.edge(t, model.EdgeCalls, "Api.Controllers.PoliciesController.Get", "IPolicyQuery.RunAsync")
}

// One declaration, several names, one type: both members type their receiver
// and the class holds the type once per declaration.
func TestMultipleDeclaratorsShareOneDeclaredType(t *testing.T) {
	em := mustRun(t, `
public class Svc
{
    private IFoo _a, _b;

    public void Run()
    {
        _a.One();
        _b.Two();
    }
}
`)
	em.edge(t, model.EdgeComposes, "Svc", "IFoo")
	em.edge(t, model.EdgeCalls, "Svc.Run", "IFoo.One")
	em.edge(t, model.EdgeCalls, "Svc.Run", "IFoo.Two")
}

// The written type is reduced to the simple name a symbol is indexed under.
// A primitive names nothing: `int` is not a relationship, and a class holding
// its own type is not one either.
func TestDeclaredTypesReduceToTheIndexedName(t *testing.T) {
	em := mustRun(t, `
public class Holder
{
    private List<IFoo> _many;
    private IBar? _maybe;
    private IBaz[] _several;
    private System.Guid _id;
    private int _count;
    private Holder _self;
    public string Name { get; set; }
}
`)
	for _, typ := range []string{"List", "IBar", "IBaz", "Guid"} {
		em.edge(t, model.EdgeComposes, "Holder", typ)
	}
	for _, typ := range []string{"int", "string", "Holder"} {
		em.noEdge(t, model.EdgeComposes, "Holder", typ)
	}
}

// A receiver with no declared type keeps the text the generic pass emitted, so
// nothing this lane does not understand is lost. `this.` is the same receiver
// as the bare member — the grammar leaves `this` unnamed.
func TestUntypedReceiversKeepTheWrittenTarget(t *testing.T) {
	em := mustRun(t, `
public class Svc
{
    private IFoo _foo;

    public void Run()
    {
        Console.WriteLine("x");
        Helper();
        this._foo.Typed();
        this.Own();
        other.Thing.Deep();
    }
}
`)
	em.edge(t, model.EdgeCalls, "Svc.Run", "Console.WriteLine")
	em.edge(t, model.EdgeCalls, "Svc.Run", "Helper")
	em.edge(t, model.EdgeCalls, "Svc.Run", "IFoo.Typed")
	em.edge(t, model.EdgeCalls, "Svc.Run", "this.Own")
	em.edge(t, model.EdgeCalls, "Svc.Run", "other.Thing.Deep")
}

// A constructor is a call source like any method, and a generic instantiation
// targets the base type.
func TestConstructorBodyAndGenericInstantiation(t *testing.T) {
	em := mustRun(t, `
public class Svc
{
    public Svc()
    {
        var x = new ListResponseModel<Policy>(null);
    }
}
`)
	em.edge(t, model.EdgeCalls, "Svc.Svc", "ListResponseModel")
}

// A declaration the grammar hands back nameless, and one with no body at all,
// are walked without emitting a half-named edge. `new string(...)` names a
// primitive, which is not a relationship either.
func TestDeclarationsWithNoNameAndNoBodyEmitNothing(t *testing.T) {
	em := mustRun(t, `
public class { private IFoo _a; }
public record R(int X);
public interface I { void (); }
public class P { public int { get; } public void M() { var s = new string('a', 3); } }
`)
	if len(em.edges) != 0 {
		t.Errorf("expected no edges from nameless/bodyless declarations, got %v", em.edges)
	}
}

var errBoom = errors.New("boom")

// errAfter accepts a number of edges and then fails, so each error return on
// the emit path can be reached in turn.
type errAfter struct {
	after int
	seen  int
}

func (errAfter) Symbol(extract.EmittedSymbol) error { return nil }

func (e *errAfter) Edge(extract.EmittedEdge) error {
	if e.seen >= e.after {
		return errBoom
	}
	e.seen++
	return nil
}

// An emitter failure aborts the walk and surfaces unchanged, so the scan
// harness sees the write error rather than a silently truncated file.
func TestEmitterErrorsAbortTheWalk(t *testing.T) {
	src := `
public class Svc
{
    private IFoo _foo;

    public void Run()
    {
        _foo.One();
    }
}
`
	// Fail on the first edge (the holder), then on the second (the call), so
	// both the collect path and the call path surface the emitter's error.
	for _, after := range []int{0, 1} {
		em := &errAfter{after: after}
		if err := (Extractor{}).Extract(parse(t, src), []byte(src), "f.cs", em); !errors.Is(err, errBoom) {
			t.Errorf("after %d edges: err = %v, want errBoom", after, err)
		}
	}
}
