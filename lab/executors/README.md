# executors

Where and how a run happens. Two facts about each one are read before anything
spawns, and both come from the design docs rather than from imagination.

**Neither executor exists yet.** Cycle 03 builds isolated-home; cycle 08 builds
the container. They are declared here because the planner must be able to refuse
an impossible combination before either exists — a job that cannot authenticate
is a burned arm, and its partner goes with it.

That means these are the one place in the catalog that describes code rather
than an ecosystem fact, and they are the one place most likely to be wrong. Both
files carry their source.

## isolated-home

`preserves_auth: ["subscription", "api_key"]`

- *subscription* is verbatim from `02-architecture.md`: isolated-home "keeps the
  host subscription usable".
- *api_key* comes from `03-01`, which sets the environment allowlist: "Start
  from empty, add what the agent tool genuinely needs (`PATH`, `TERM`,
  **credentials the tool requires**)".

The second one was questioned on review, because `02-architecture.md` mentions
only the subscription and describes the environment as scrubbed. It is recorded
here because it decides real money: four of the five arms in the frozen
csharp-aspnet campaign reach their model by API key, and dropping it would
reject arms that genuinely ran. **If cycle 03's allowlist turns out not to pass
credentials, this line is wrong in the expensive direction — a false accept —
and it is the first thing to check.**

`isolates_global_config: true` is verbatim: "Strong config isolation".

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
