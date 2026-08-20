# executors

Where and how a run happens. Two facts about each one are read before anything
spawns, and both come from the design docs rather than from imagination.

**Only isolated-home exists.** Cycle 03 built it, 03-06 finished its
authentication, and cycle 08 builds the container. Both are declared here because
the planner must be able to refuse an impossible combination before either
exists — a job that cannot authenticate is a burned arm, and its partner goes
with it.

That means these are the one place in the catalog that describes code rather
than an ecosystem fact, and they are the one place most likely to be wrong. Both
files carry their source.

## isolated-home

`preserves_auth: ["subscription", "api_key"]`

**Both are now true of the implementation, and neither was when this was
written.** The line claimed since cycle 03 that isolated-home "keeps the host
subscription usable"; it was never implemented and never tested, and the first
live cell produced two empty arms because of it. 03-06 made it true.

- *subscription* holds because the operator's credential is read once in the
  attended parent and provisioned into a per-run config directory that
  `CLAUDE_CONFIG_DIR` points the session at. The host keychain is never linked
  in, and the provisioned credential carries no refresh token, so no run can
  rotate the operator's login.
- *api_key* holds through the environment allowlist, which carries
  `ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` with their reasons. This was the
  entry questioned on review as a possible false accept in the expensive
  direction — four of the five arms in the frozen csharp-aspnet run set reach
  their model by API key. It was checked rather than argued: the allowlist does
  pass them, and there is a test per credential variable saying so.

**macOS only, and that is a decision rather than an omission** (2026-08-17). The
subscription route is measured there; nothing in the implementation branches on
the platform, and the file it writes is the store Linux uses as its primary one,
which is a reason to expect it to hold and not a reason to claim it does. 08-05
stands up a container, which is a Linux box with a credential story of its own,
and that is where the measurement happens. Until it exists, **no repository may
report a Linux result on this route.**

`isolates_global_config: true` is verbatim: "Strong config isolation". It is also
now literal: the run's config directory is its own, outside the disposable HOME,
and it is removed when the run is released.

## container

`preserves_auth: []` is verbatim from `02-architecture.md`: "Credentials stay on
the host; the container never receives them."

`isolates_global_config: true` is inferred from the same lines: the container is
the escalation "for a subject that mutates global state in ways the disposable
home cannot contain."

The container is declared without being referenced by any subject, because it is
the only executor that makes the auth question answer NO, and a rule with no
case that exercises it is a rule nobody can check.

## local

Deleted. It appears in the architecture doc's list, nothing references it, and
the judgment phases that would are cycle 05. It arrives with them.
