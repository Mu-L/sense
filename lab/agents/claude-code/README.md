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
