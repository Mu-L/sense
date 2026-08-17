# claude-code

Last verified: 2026-08-15.

## Files it touches

`$HOME/.claude/` and `$HOME/.claude.json`, both HOME-relative. `.claude.json`
is a single large per-project state file, so **two runs against the same
repository path share it and race on it**. A per-run path is what separates
them; see cycle 03.

## Known side effects on the repository

Reading the repository's own `CLAUDE.md` and `AGENTS.md` is normal behaviour,
not a contamination bug — but it means the SUBJECT's instructions and the
REPOSITORY's instructions arrive through the same channel. `sense scan` writes
into both when it configures a repo, which is how the baseline arm can be handed
Sense without anyone doing it on purpose. Cycle 03 (03-02) carries the measured
list.

Measured 2026-08-15: `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` does **not** stop the
operator's own `~/.claude/CLAUDE.md` from being loaded. Asked directly, a
benched session quoted it back. Only a disposable HOME closes that.

## Untrusted workspaces fail quietly

A path the tool has not seen before is untrusted, and it drops
`permissions.allow` entries from `.claude/settings.json` with a line on stderr
and nothing else. The session still runs, the MCP server still loads, and part
of the arm's configured surface is silently missing. A per-run disposable path
makes every run untrusted, so this becomes the default rather than the
exception.

## Auth

Subscription or API key.

**A seat is not an environment variable.** It lives in the platform credential
store, which the tool locates through `HOME` — so a run with a disposable one
looks for a store that is not there and exits in about a second with "Not logged
in", at zero cost. Linking the host store in is worse than leaving it out: a
denied read returns a sentinel the strict read path short-circuits on, so it
never reaches the file fallback, while an absent store is unambiguously null and
always falls back. Measured 2026-08-17, a linked store passed twice and then
failed ten times in a row with a real-`HOME` control passing throughout.

The route the bench uses instead is `CLAUDE_CONFIG_DIR` per run, provisioned with
`.credentials.json` at mode `0600` holding `accessToken`, `expiresAt` and
`scopes`. `scopes` is the gate: measured field by field, a token with an expiry
and no scopes reads as logged out. **This file path is undocumented on macOS** —
the published docs name it for Linux and Windows only — so it is verified against
the shipped binary's credential store and re-verified when that binary moves.

**`CLAUDE_CODE_OAUTH_TOKEN` is not used, and the mechanism is why.** When it is
set the tool writes a plaintext credential, and its fallback combiner then
deletes the operator's keychain entry on exit
([#37512](https://github.com/anthropics/claude-code/issues/37512), closed as not
planned). The delete fires only when the keychain read returned non-null, the
keychain write then failed, and the plaintext write succeeded — so a disposable
`HOME`, which makes that first read null, cannot trigger it today. The variable
is still out of the allowlist: a bench that only fails to destroy the operator's
login by accident of another design decision is one refactor from destroying it.

`claude setup-token` mints a one-year token for non-interactive use, and it is
consumed through that same variable. It is held in reserve for a campaign that
outlives an access token, not used as the default door.

The provisioned credential deliberately carries no refresh token: a run that
cannot refresh cannot rotate the operator's login, so no number of unattended
cells can invalidate the host seat by succeeding. The cost is that a campaign
outliving the access token re-provisions rather than running on.

## Judging

`judge_args` drive the tool tool-less and single-turn, for grading. `-p` with
`--output-format text` is one turn and one reply; `--allowedTools` with an empty
value leaves the model nothing to call.

Both halves matter and neither is trusted on its own. The grading also runs in
an empty directory inside a disposable HOME, so there is no repository, no MCP
registration and no routing guidance to reach even if a tool were enabled. A
judge that can read the repository may verify claims itself, and whether it
chooses to varies run to run: that is invisible grading variance, and it changes
what is measured.

**Last verified:** 2026-08-16, against a stand-in rather than against the CLI.
The flags are an ecosystem fact and they move; if a release changes them, this
tool's fallback is a direct API call, decided here rather than globally.
