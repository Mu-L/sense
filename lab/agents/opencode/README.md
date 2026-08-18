# opencode

Last verified: 2026-08-18, against **opencode 1.18.18**, by driving a scenario
end to end on both arms. Everything below was measured on this machine rather
than read out of a doc page.

## Why it exists

It is how the confirmation arms reach the models that are neither Claude nor
GPT. An earlier approach pointed the Claude CLI at an Anthropic-compatible
endpoint serving cloud models, and it drove them so poorly that they ignored
Sense almost entirely: **2 Sense calls against 97 native ones** in one session
that completed and looked fine. This tool has a native authed provider and
native MCP support, and the measured sense arm reached Sense through it.

## Files it touches

`$HOME/.local/share/opencode` and `$HOME/.config/opencode`, both HOME-relative,
and it keeps state in **both** — which is why `config_dirs` is a list. The
credential directory is listed **first**, because that is the one the credential
route reads.

## Auth: through the environment, and that is not a convenience

API keys, per provider, in `auth.json`. Measured: `OPENCODE_CONFIG_DIR` does
**not** move that file — pointed at a directory holding a valid `auth.json`, the
tool reported 0 credentials and named `~/.local/share/opencode/auth.json` as
where it was looking.

So the login lives inside HOME and nowhere else, and HOME is the disposable one.
Writing a credential there would put state in the exact place the contamination
proof reads as a dirty arm, and **every arm of every run would report as
contaminated** — a proof that has stopped proving anything.

`OPENCODE_AUTH_CONTENT` is the way through: the document arrives in a variable,
the tool reads it, and the disposable HOME stays empty. Measured working:
`auth list` in an isolated HOME with only that variable set reported all three
providers.

`credential_fields` names **only the providers the catalog's models actually
use** — currently `kimi-for-coding` and `ollama-cloud`. A run then holds only
the keys it needs rather than every key the operator has, and adding a provider
is two lines there.

**A provider missing from that list does not fail as an auth error.** Measured
2026-08-18: with only `kimi-for-coding` in the document, asking for
`ollama-cloud/glm-5.2` returns one `error` event reading `UnknownError:
Unexpected server error. Check server logs for details.` and nothing else —
byte-identical in shape to what asking for a model that does not exist returns,
so a missing key reads as a broken model id. It cost two arms of a real cell
before it was understood.

**Nothing has to be understood twice.** Each model declares the key it needs
(`credential_key`) and the cell is refused before either arm spawns, naming the
provider and the file to add it to. The message reads:

> `ollama-cloud/glm-5.2` is driven through `opencode`, whose credential carries
> [kimi-for-coding.type kimi-for-coding.key] and nothing for `"ollama-cloud"` …
> Add `"ollama-cloud"` fields to `credential_fields` in the opencode agent file

The expiry is declared `never`, because an API key has none. That is a stated
fact rather than a missing one: a tool that simply left it out would be
indistinguishable from one whose expiry nobody wired up, and a cell would be
planned against a credential nothing could check.

## Driving it headless

`run --format json --auto`, with the prompt on **stdin**. The model id passes
through untouched, in the `provider/model` form the model file states —
`kimi-for-coding/k3` reached the tool as `--model kimi-for-coding/k3`.

There is no flag that appends to a system prompt, so the wall note goes in front
of the prompt, the same as Codex.

## Cold start: measured, reported, and not corrected for

It spawns and initialises the MCP server before the first streamed event. That
was once misdiagnosed as "hangs on MCP" when it was premature kills at 35 to 60
seconds.

Time to first streamed event, same scenario, same repository, same day:

| tool | sense arm | baseline arm | the sense arm's handicap |
|---|---|---|---|
| claude-code | 2.3s | 1.6s | 0.7s |
| opencode (run 1) | 25.3s | 8.7s | 16.6s |
| opencode (run 2) | 18.6s | 6.8s | 11.8s |

**The handicap is real and it is one tool's, not the rule's.** On the 360-second
wall used here it is 3 to 5 per cent of the sense arm's budget. The wall starts
at spawn for both arms and for every tool, which is a definition every recorded
number rests on, and changing it is not this adapter's business: a wall rule
that differed by agent tool would make cross-tool results incomparable, which is
worse than a measured handicap somebody can read. **Reported here so a later
decision can be taken on numbers rather than on a hunch.**

## MCP registration

`sense setup --tools opencode` writes `opencode.json`, `AGENTS.md`,
`.opencode/skills/` and `.opencode/plugin/sense.js`. The registration lives
under a `mcp` key and states its command as a **single argv array**, which is a
different shape from the other JSON registration — hence `mcp_registration` in
the config. The capture shim rewrites it in place and recorded 8 frames in the
measured run.

## Judging

`judge_args` is empty: nothing here has been measured to drive it tool-less and
single-turn, and a judge with tools is a different instrument from one without.
