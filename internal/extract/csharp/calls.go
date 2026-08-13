package csharp

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/model"
)

// walkCalls streams every call in a method or constructor body. C# has no
// nested method or class declarations inside a body — a local function is a
// local_function_statement, and its calls belong to the method that holds it —
// so the whole subtree is one call scope.
func (w *walker) walkCalls(n *sitter.Node, src string, members map[string]string) error {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		child := n.NamedChild(i)
		if err := w.emitCall(child, src, members); err != nil {
			return err
		}
		if err := w.walkCalls(child, src, members); err != nil {
			return err
		}
	}
	return nil
}

// emitCall writes the calls edge for one invocation or instantiation, and
// nothing for any other node.
func (w *walker) emitCall(n *sitter.Node, src string, members map[string]string) error {
	var target string
	switch n.Kind() {
	case "invocation_expression":
		target = w.invocationTarget(n, members)
	case "object_creation_expression":
		// `new X(...)` targets the class, not its constructor, so the
		// instantiation shows up under X's called_by — the PHP lane's shape.
		target = typeName(n.ChildByFieldName("type"), w.source)
	default:
		return nil
	}
	if target == "" {
		return nil
	}
	line := extract.Line(n.StartPosition())
	return w.emit.Edge(extract.EmittedEdge{
		SourceQualified: src,
		TargetQualified: target,
		Kind:            model.EdgeCalls,
		Line:            &line,
		Confidence:      extract.ConfidenceStatic,
	})
}

// invocationTarget names what an invocation calls. When the receiver is a
// field or property of the enclosing class, the target carries that member's
// declared type (`IPolicyQuery.RunAsync`) — the bare name is what lets a
// same-named method in an unrelated class win the edge. Anything else keeps
// the written text, which is what the generic pass emitted before this lane.
func (w *walker) invocationTarget(n *sitter.Node, members map[string]string) string {
	fn := n.ChildByFieldName("function")
	if fn != nil && fn.Kind() == "member_access_expression" {
		name := extract.Text(fn.ChildByFieldName("name"), w.source)
		if typ := w.receiverType(fn.ChildByFieldName("expression"), members); typ != "" && name != "" {
			return typ + "." + name
		}
	}
	return strings.TrimSpace(extract.Text(fn, w.source))
}

// receiverType resolves a call's receiver to a declared type, or "" when there
// is no witness. `_policyQuery.Run()` and `this._policyQuery.Run()` are the
// same receiver, so an access rooted at `this` reads the member table too.
func (w *walker) receiverType(expr *sitter.Node, members map[string]string) string {
	if expr == nil {
		return ""
	}
	switch expr.Kind() {
	case "identifier":
		return members[extract.Text(expr, w.source)]
	case "member_access_expression":
		if inner := expr.ChildByFieldName("expression"); inner != nil && inner.Kind() == "this" {
			return members[extract.Text(expr.ChildByFieldName("name"), w.source)]
		}
	}
	return ""
}
