package csharp

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/model"
)

// collectMembers reads a class's declared fields and properties, emits the
// holder edge for each, and returns the member -> declared-type table the call
// walk types its receivers from.
//
// Both facts come from the same declaration, so it is read once:
// `private readonly IPolicyQuery _policyQuery;` is both "PoliciesController
// composes IPolicyQuery" and "_policyQuery is an IPolicyQuery".
func (w *walker) collectMembers(class *sitter.Node, name, qualified string) (map[string]string, error) {
	body := class.ChildByFieldName("body")
	if body == nil {
		return nil, nil
	}
	members := map[string]string{}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		typ, names := declaredMember(child, w.source)
		if typ == "" || len(names) == 0 {
			continue
		}
		for _, n := range names {
			members[n] = typ
		}
		if err := w.emitHolds(child, name, qualified, typ); err != nil {
			return nil, err
		}
	}
	return members, nil
}

// emitHolds records that a class holds a value of another type — the
// composition edge Go gets for free from a struct field. Confidence is Static:
// the type is written in the declaration, never inferred. A class holding its
// own type is not a relationship worth an edge.
func (w *walker) emitHolds(decl *sitter.Node, name, qualified, typ string) error {
	if typ == name {
		return nil
	}
	line := extract.Line(decl.StartPosition())
	return w.emit.Edge(extract.EmittedEdge{
		SourceQualified: qualified,
		TargetQualified: typ,
		Kind:            model.EdgeComposes,
		Line:            &line,
		Confidence:      extract.ConfidenceStatic,
	})
}

// declaredMember returns the declared type and the member names of one field or
// property declaration, or "" for anything else in a class body. A field
// declares several names at one type (`private IFoo _a, _b;`); a property
// declares exactly one.
func declaredMember(n *sitter.Node, source []byte) (string, []string) {
	switch n.Kind() {
	case "property_declaration":
		name := extract.Text(n.ChildByFieldName("name"), source)
		if name == "" {
			return "", nil
		}
		return typeName(n.ChildByFieldName("type"), source), []string{name}
	case "field_declaration":
		for i := uint(0); i < n.NamedChildCount(); i++ {
			decl := n.NamedChild(i)
			if decl.Kind() != "variable_declaration" {
				continue // a modifier: private, readonly, static
			}
			return typeName(decl.ChildByFieldName("type"), source), declaratorNames(decl, source)
		}
	}
	return "", nil
}

// declaratorNames lists the names a variable declaration binds.
func declaratorNames(decl *sitter.Node, source []byte) []string {
	var names []string
	for i := uint(0); i < decl.NamedChildCount(); i++ {
		child := decl.NamedChild(i)
		if child.Kind() != "variable_declarator" {
			continue
		}
		if name := extract.Text(child.ChildByFieldName("name"), source); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// typeName reduces a written type to the simple name a symbol is indexed
// under: `IFoo?`, `List<IFoo>` and `IFoo[]` all name their base type, and
// `System.Guid` names Guid. A predefined type (int, string, void) or an
// inferred one (var) names nothing and returns "" — a primitive is not a
// relationship.
func typeName(n *sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case "identifier":
		return extract.Text(n, source)
	case "generic_name", "nullable_type", "array_type":
		return typeName(n.NamedChild(0), source)
	case "qualified_name":
		return typeName(n.ChildByFieldName("name"), source)
	}
	return ""
}
