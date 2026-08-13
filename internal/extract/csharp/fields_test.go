package csharp

import (
	"testing"

	"github.com/luuuc/sense/internal/model"
)

// Worklist row 1 — a class holding a declared-type field composes with that
// type. The idiom is constructor-injected dependency storage, the single most
// common shape in an ASP.NET Core codebase.
//
// Source:  bitwarden-server/src/Api/AdminConsole/Controllers/PoliciesController.cs:35
//
//	private readonly IPolicyQuery _policyQuery;
func TestFieldDeclarationComposesWithItsType(t *testing.T) {
	em := mustRun(t, `
public class PoliciesController
{
    private readonly IPolicyQuery _policyQuery;
}
`)
	em.edge(t, model.EdgeComposes, "PoliciesController", "IPolicyQuery")
}

// Worklist row 1, second shape — an auto-property holds its type exactly as a
// field does. Split from the field test so a lane that lands one and not the
// other fails as two lines, not one.
//
// Source:  ServiceStack/ServiceStack/src/ServiceStack/SessionFactory.cs:12
// declares the field form in the same class whose properties carry the same
// types; the property idiom is the C# spelling of the same holder.
func TestPropertyDeclarationComposesWithItsType(t *testing.T) {
	em := mustRun(t, `
public class SessionFactory
{
    public ICacheClient Cache { get; }
}
`)
	em.edge(t, model.EdgeComposes, "SessionFactory", "ICacheClient")
}
