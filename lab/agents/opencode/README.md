# opencode

The human half of `agent.json`: operational facts about someone else's product.
Dated and tied to a version. **Nothing validates these dates and nothing warns
when one is old — a stale date here is not a guarantee of anything.**

Last verified: 2026-08-15, against the model set in `lab/models/`.

## Write a model id in provider/model form, always

This is the quirk that has already cost a whole arm.

An arm was written as `mistral-large-3:cloud`. The runner mapped a bare
`<id>:cloud` to `ollama-cloud/<id>`, which **ate the size tag the model needed**:
the provider offers only `mistral-large-3:675b`, so the id resolved to something
that does not exist. Every run of that arm came back **empty, zero tokens, exit
1** — and it failed as a bad result rather than as a crash, which is why it took
a campaign to notice.

`provider/model` form passes through untouched. The catalog now carries the
full id (`ollama-cloud/mistral-large-3:675b`), and the shorter form is recorded
as an alias so nothing silently re-derives it.

## Files it touches

`$HOME/.config/opencode` and `$HOME/.local/share/opencode`, both HOME-relative,
so a disposable home contains them.

## Auth

API key only. It cannot reach a model that is available under a subscription
alone, and the catalog validator rejects that pairing at load time rather than
at spawn time.
