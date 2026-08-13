package csharp

import (
	"testing"

	"github.com/luuuc/sense/internal/model"
)

// Worklist row 3 — `new X(...)` emits a calls edge to X. langspec lists
// object_creation_expression in CallTypes, but the generic callTarget reads
// the `function` then `name` field and the C# grammar names neither on a
// `new` expression: it uses `type`. The edge is dropped outright.
//
// The target is the class, matching the PHP lane (emitCreation ->
// callEdge(typ)), so the instantiation shows up under the class's called_by.
//
// Source:  bitwarden-server/src/Api/AdminConsole/Controllers/PoliciesController.cs:66
//
//	return new PolicyStatusResponseModel(policy);
func TestObjectCreationEmitsCallEdgeToTheType(t *testing.T) {
	em := mustRun(t, `
public class PoliciesController
{
    public PolicyStatusResponseModel Get()
    {
        return new PolicyStatusResponseModel(null);
    }
}
`)
	em.edge(t, model.EdgeCalls, "PoliciesController.Get", "PolicyStatusResponseModel")
}

// Worklist row 3, throw form — `throw new X()` is the same instantiation in a
// different statement position, and it is how the exception types in this
// stack are reached at all. Split out so a walker that handles only the
// return position fails on its own line.
//
// Source:  bitwarden-server/src/Api/AdminConsole/Controllers/PoliciesController.cs:76
//
//	throw new NotFoundException();
func TestThrownObjectCreationEmitsCallEdgeToTheType(t *testing.T) {
	em := mustRun(t, `
public class PoliciesController
{
    public void Delete()
    {
        throw new NotFoundException();
    }
}
`)
	em.edge(t, model.EdgeCalls, "PoliciesController.Delete", "NotFoundException")
}
