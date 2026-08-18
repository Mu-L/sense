# codex

Last verified: 2026-08-18, against **codex-cli 0.147.0**, by driving a scenario
end to end on both arms. Everything below was measured on this machine rather
than read out of a doc page, and the run it came from is recorded in cycle
08-01.

## Files it touches

`$HOME/.codex`, HOME-relative — and `CODEX_HOME` moves it, which is how a run
is given a config directory of its own. An earlier note here said Codex takes
no config-directory variable; that was wrong.

## Auth

Subscription (a ChatGPT login) or an API key. The login lives in
`auth.json` inside the config directory, and there is **no keychain item**: the
file is the only source.

**The refresh token is required, and that is a real cost.** With
`tokens.access_token`, `tokens.account_id` and `tokens.id_token` provisioned
and `tokens.refresh_token` withheld, every websocket connection came back
`401 Unauthorized` and the session produced nothing. Adding the refresh token
back, changing nothing else, the same session answered. So Codex cannot be run
on a credential narrowed the way Claude Code's is, and a Codex arm therefore
holds a token that could rotate the operator's login. It did not rotate one in
the measured run — `last_refresh` and both tokens were byte-identical
afterwards — but the capability is there and it is the reason this is declared
per tool in `credential_fields` rather than assumed.

The credential carries no expiry field of its own, so the expiry is read from
the `exp` claim of the access token (`"credential_expiry": "jwt:..."`).

## Driving it headless

`exec --json`, with the prompt on **stdin**. Two things bite:

- **stdin must be closed.** With a prompt passed as an argument and stdin left
  open, it prints `Reading additional input from stdin...` and waits forever. It
  is spawned with the prompt on stdin, like every other arm here, which is the
  shape that works.
- **`--full-auto` does not exist.** It was declared here before anything ran it
  and the binary rejects it outright. The flag that means what Claude Code's
  `--permission-mode bypassPermissions` means is
  `--dangerously-bypass-approvals-and-sandbox`, and both arms get it equally.

## The wall note reaches it through the prompt

There is no flag that appends to a system prompt, so the note goes in front of
the prompt (`"wall_note_delivery": "prompt"`). The wording is the measured one,
unchanged.

## What `sense setup` writes for it

`sense setup --tools codex-cli` — the product's name for this tool is
`codex-cli`, not `codex`, which is why `setup_tool` exists. It writes three
repository channels, and they are **not** Claude Code's set:

- `.codex/config.toml`
- `.mcp.json`
- `AGENTS.md`

## Known gap: the MCP capture does not see its calls

The capture shim rewrites `.mcp.json` to run the Sense server behind a tee.
Codex reads its registration from `.codex/config.toml`, so it reaches the real
server directly and the capture records **zero frames** — visible in the probe
report rather than silent, and the arm's own transcript still names every MCP
call it made. Closing it means rewriting a TOML registration as well, which is
conformance work (08-03) rather than this adapter's.

## Judging

`judge_args` is empty: nothing here has been measured to drive it tool-less and
single-turn, and a judge with tools is a different instrument from one without.
Until that is measured, Codex judges nothing.
