// Package csharp is the dedicated C# walker. C# is served today by the
// table-driven langspec registration, which declares no field or property
// node kind: a class holding `private readonly IFoo _foo;` records no
// relationship to IFoo, and a call through that field (`_foo.Run()`) carries
// no receiver type, so it degrades to the bare name and binds to whichever
// same-named method the resolver reaches first.
//
// This package is the walker that fixes those three shapes — the holder
// edge, the typed receiver, and `new X(...)` — mirroring the PHP lane
// (internal/extract/php). ASP.NET Core framework inference is model/resolve
// side work, not part of the extractor: this package stays a pure
// bytes-in, data-out walker.
//
// It EXTENDS the langspec entry rather than replacing it. langspec already
// gets C# symbols, namespaces, base lists, usings, visibility and attributes
// right, and re-implementing those here would be a second copy of a correct
// generic pass. So langspec keeps the "csharp" key and composes this walker
// in (internal/extract/langspec/csharp.go): the generic pass emits symbols,
// inherits and imports, this one emits composes and calls. The spec's
// CallTypes are dropped there — a call has exactly one owner, and typing the
// receiver is the whole point of this lane.
package csharp

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/grammars"
)

// Extractor implements extract.Extractor for C#.
type Extractor struct{}

// Grammar returns the vendored tree-sitter C# grammar.
func (Extractor) Grammar() *sitter.Language { return grammars.CSharp() }

// Language returns the language key ("csharp") this extractor extracts.
func (Extractor) Language() string { return "csharp" }

// Extensions returns the file extensions this extractor claims.
func (Extractor) Extensions() []string { return []string{".cs"} }

// Tier reports C#'s current support tier.
func (Extractor) Tier() extract.Tier { return extract.TierStandard }

// Extract walks one parsed C# file and streams the composes and calls edges
// this lane owns to emit. Symbols, inheritance and imports come from the
// langspec pass that runs before it.
func (Extractor) Extract(tree *sitter.Tree, source []byte, _ string, emit extract.Emitter) error {
	if tree == nil {
		return nil
	}
	w := &walker{source: source, emit: emit}
	return w.walk(tree.RootNode(), nil, nil)
}

// walker carries the per-file state: the source bytes every node text is read
// from, and the emitter edges stream to.
type walker struct {
	source []byte
	emit   extract.Emitter
}

// classKinds are the declarations that open a naming scope, matching the
// langspec spec's ClassTypes so both passes qualify a symbol identically.
func isClass(kind string) bool {
	switch kind {
	case "class_declaration", "interface_declaration", "struct_declaration",
		"enum_declaration", "record_declaration", "namespace_declaration":
		return true
	}
	return false
}

// isFunc matches the declarations that own a body of calls, matching the
// langspec spec's FuncTypes.
func isFunc(kind string) bool {
	return kind == "method_declaration" || kind == "constructor_declaration"
}

// walk descends the tree carrying the enclosing scope and the member-type
// table of the innermost class, which is what lets a call through a field
// receiver name a type instead of a bare method.
func (w *walker) walk(n *sitter.Node, scope []string, members map[string]string) error {
	switch {
	case isClass(n.Kind()):
		return w.walkClass(n, scope, members)
	case isFunc(n.Kind()):
		return w.walkFunc(n, scope, members)
	}
	return w.walkChildren(n, scope, members)
}

func (w *walker) walkChildren(n *sitter.Node, scope []string, members map[string]string) error {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if err := w.walk(n.NamedChild(i), scope, members); err != nil {
			return err
		}
	}
	return nil
}

// nodeName reads a declaration's name token, "" when the grammar offers none.
func nodeName(n *sitter.Node, source []byte) string {
	return extract.Text(n.ChildByFieldName("name"), source)
}

// qualify joins a scope into the dotted qualified name langspec writes, so an
// edge emitted here binds to the symbol emitted there.
func qualify(scope []string) string { return strings.Join(scope, ".") }

// walkClass emits the class's holder edges, then walks its body with the
// member-type table it just built.
func (w *walker) walkClass(n *sitter.Node, scope []string, members map[string]string) error {
	name := nodeName(n, w.source)
	if name == "" {
		return w.walkChildren(n, scope, members)
	}
	inner := append(append([]string{}, scope...), name)
	own, err := w.collectMembers(n, name, qualify(inner))
	if err != nil {
		return err
	}
	return w.walkChildren(n, inner, own)
}

// walkFunc streams every call in one method or constructor body. Nested
// declarations are not descended into: langspec's call walk stops at them too.
func (w *walker) walkFunc(n *sitter.Node, scope []string, members map[string]string) error {
	inner := append(append([]string{}, scope...), nodeName(n, w.source))
	return w.walkCalls(n, qualify(inner), members)
}
