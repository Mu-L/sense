// Package csharp is the dedicated C# extractor. C# is served today by the
// table-driven langspec registration, which declares no field or property
// node kind: a class holding `private readonly IFoo _foo;` records no
// relationship to IFoo, and a call through that field (`_foo.Run()`) carries
// no receiver type, so it degrades to the bare name and binds to whichever
// same-named method the resolver reaches first.
//
// This package is the walker that fixes those three shapes — the holder
// edge, the typed receiver, and `new X(...)` — mirroring the PHP lane
// (internal/extract/php). ASP.NET Core framework inference is model/resolve
// side work (internal/model/aspnetcore.go), not part of the extractor: this
// package stays a pure bytes-in, data-out walker.
//
// It does not register itself: langspec still owns the "csharp" key until
// the walker is complete.
package csharp

import (
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

// Extract walks one parsed C# file and streams symbols and edges to emit.
func (Extractor) Extract(tree *sitter.Tree, _ []byte, _ string, _ extract.Emitter) error {
	if tree == nil {
		return nil
	}
	return nil
}
