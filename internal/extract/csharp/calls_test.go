package csharp

import (
	"testing"

	"github.com/luuuc/sense/internal/model"
)

// Worklist row 2 — a call through a field receiver carries the field's
// declared type, so it binds to that type's method and not to the bare name.
//
// Source:  bitwarden-server/src/Api/AdminConsole/Controllers/PoliciesController.cs:35 (the
// declaration) and :60 (the call):
//
//	private readonly IPolicyQuery _policyQuery;
//	var policy = await _policyQuery.RunAsync(orgId, type);
//
// Measured today over MCP, the untyped receiver degrades to `RunAsync` and the
// resolver picks BulkAutomaticallyConfirmOrganizationUsersCommand.RunAsync at
// confidence 0.3 — an unrelated class in a different feature area.
func TestCallThroughFieldReceiverCarriesDeclaredType(t *testing.T) {
	em := mustRun(t, `
public class PoliciesController
{
    private readonly IPolicyQuery _policyQuery;

    public async Task<PolicyStatus> Get(Guid orgId, PolicyType type)
    {
        var policy = await _policyQuery.RunAsync(orgId, type);
        return policy;
    }
}
`)
	em.edge(t, model.EdgeCalls, "PoliciesController.Get", "IPolicyQuery.RunAsync")
	// The bare-name target is the mis-binding itself: it is what lets a
	// same-named method in an unrelated class win the edge.
	if em.hasEdgeTarget("RunAsync") {
		t.Errorf("typed receiver still emitted a bare-name target: %v", em.edges)
	}
}

// Worklist row 2, property receiver — the same shape spelled as a property.
func TestCallThroughPropertyReceiverCarriesDeclaredType(t *testing.T) {
	em := mustRun(t, `
public class SessionFactory
{
    public ICacheClient Cache { get; }

    public void Clear()
    {
        Cache.Remove("session");
    }
}
`)
	em.edge(t, model.EdgeCalls, "SessionFactory.Clear", "ICacheClient.Remove")
}
