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
