package langspec

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/luuuc/sense/internal/extract"
	"github.com/luuuc/sense/internal/extract/csharp"
	"github.com/luuuc/sense/internal/grammars"
)

// csharpExtractor is the registered C# extractor: the table-driven generic
// pass for symbols, namespaces, base lists, usings, visibility and attributes,
// then the dedicated C# walker (internal/extract/csharp) for the two shapes no
// table describes — the holder edge a field or property declares, and a call
// through a member receiver carrying that member's declared type.
//
// The composition lives here, in C#'s own spec file, rather than as a hook on
// the generic engine: nothing about it is shared with the other langspec
// grammars, and the C# heuristics themselves stay in the csharp package. The
// spec below therefore declares no CallTypes — a call has exactly one owner,
// and typing the receiver is the whole point of the walker.
type csharpExtractor struct {
	generic extract.Extractor
	walker  csharp.Extractor
}

func (e csharpExtractor) Grammar() *sitter.Language { return e.generic.Grammar() }
func (e csharpExtractor) Language() string          { return e.generic.Language() }
func (e csharpExtractor) Extensions() []string      { return e.generic.Extensions() }
func (e csharpExtractor) Tier() extract.Tier        { return e.generic.Tier() }

func (e csharpExtractor) Extract(tree *sitter.Tree, source []byte, path string, emit extract.Emitter) error {
	if err := e.generic.Extract(tree, source, path, emit); err != nil {
		return err
	}
	return e.walker.Extract(tree, source, path, emit)
}

func init() {
	extract.Register(csharpExtractor{generic: New(langSpec{
		Name:      "csharp",
		Exts:      []string{".cs"},
		Grammar:   grammars.CSharp(),
		Tier:      extract.TierStandard,
		Separator: ".",

		FuncTypes:   []string{"method_declaration", "constructor_declaration"},
		ClassTypes:  []string{"class_declaration", "interface_declaration", "struct_declaration", "enum_declaration", "record_declaration", "namespace_declaration"},
		ImportTypes: []string{"using_directive"},

		InheritKinds: []string{"base_list"},

		NameField: "name",

		VisibilityFn:    csharpVisibility,
		AnnotationKinds: []string{"attribute_list"},
	})})
}
