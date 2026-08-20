---
phase: preflight
reads: scenario.yaml
writes: preflight.json
emits: [AUTO]
wall: 15m
---

# preflight

## Task

Expand the repository's bench into every job it implies and answer, for each one, whether it can
run at all.

## Scope

You resolve configuration. **Out of scope:** running anything, spending anything, judging a
scenario, any other repository.

## Run

1. Expand the bench: every subject, every arm.
2. Resolve each job: does the model exist, can some tool drive it, is there an auth mode that
   reaches it, does the executor exist.
3. Reject the survivors of any cell that lost a subject.

## Decide

Nothing is judged. Every rejection carries a named reason, because a rejection without one is a
mystery someone will route around by guessing.

**A cell that lost a subject rejects its survivor.** This is the hazard the phase exists for, and
without it preflight CREATES the failure rather than preventing it: if one subject of a cell
resolves and the other does not, planning only the survivor guarantees a burned arm. The finished
side can never be paired, and the baseline's budget derives from its partner's wall, so there is
nothing to derive it from.

Rejecting the survivor is the right direction. **A run that never happens costs nothing; a run
that happens and cannot be paired costs its whole spend and yields no result.**

## Precedent

Two recorded failures have this shape:

- An arm whose model id resolved to nothing returned empty at zero tokens, byte-identical in shape
  to a session that ran and legitimately failed.
- A half-pair whose finished arm was burned because its partner was never going to run.

## Artifact

`preflight.json`: every job that will run, and every rejection with its reason.

## Done when

- Every job in the matrix appears exactly once, either as a job or as a rejection.
- Every rejection names its reason, and a survivor's rejection names why its partner failed rather
  than merely reporting that one did.
- A bench that is malformed rather than merely unsatisfiable is an error, not a rejection list:
  a bench naming a subject the catalog does not have is a typo, and a bench whose model no tool
  can drive is a rejection with a reason. Those are different answers and they send someone to
  different files.

## Do not

- Do not spend anything. This phase reads configuration.
- Do not spawn a subagent. Only the binary spawns.

## Verdict

Write `verdict.json` in this phase's directory, beside the artifact:

```json
{"phase": "preflight", "repo": "<repo id>", "cycle": <n>, "verdict": "<one of the emitted>",
 "table": "<why, in one or two sentences>"}
```

The crank reads that file and nothing else to route. A verdict left in prose is a verdict only a
person can act on, and the loop this phase belongs to runs unattended.
