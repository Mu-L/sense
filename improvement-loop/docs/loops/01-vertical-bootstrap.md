# Loop 1 - Vertical bootstrap

Stand up the scaffold a new stack needs, between "Loop 0 picked the stack" and "Loop 2 starts
admitting repos". One command, no model in the path, no repos touched.

**Loop 1 never touches repos.** Candidate pools, pinning, cloning and indexing are Loop 2's, and the
scaffold it stamps is deliberately empty of them.

## Run it

    bash bench/drivers/loop1-bootstrap.sh <key> --lang <lang> [--title "T"] [--reason "..."]

`--lang` is the `internal/extract/<lang>` directory name. `--reason` states why this vertical was
opened; it is the one thing in the record a script cannot measure.

Progress goes to **stderr**, the result JSON is the only thing on **stdout**, so
`result=$(loop1-bootstrap.sh ...)` is the whole integration. Every exit path returns a JSON,
failures included: a caller never parses prose to learn what happened.

| status | exit | means |
|---|---|---|
| `BOOTSTRAPPED` | 0 | done; the JSON carries the next loop and its command |
| `USAGE` | 64 | no `--lang` |
| `EXTRACTOR-NOT-READY` | 65 | `internal/extract/<lang>` missing or empty - see Stop below |
| `PREFLIGHT-FAILED` | 70 | names each missing piece |
| `STAMP-FAILED` | 71 | `new-vertical.sh` failed |
| `NOT-BOOTSTRAPPED` | evaluator's | the stamp is wrong; the check says which of the four ways |
| `RECORD-FAILED` | 72 | the LEDGER entry could not be written |

Environment: this repo and python3. No clone root, no GitHub token, no `sense` binary - and the
preflight says so out loud rather than failing three steps later.

## What it does

1. **Preflight** - python3, git, the sense repo, and `internal/extract/<lang>` with production files.
2. **Stamp** - `new-vertical.sh` writes `verticals/<key>/`: `repos.txt`, `scenarios/`,
   `PINNED_COMMITS.json`, `arms.txt`, a README tracker, a `repos.md` slate, an empty `findings/`.
   Idempotent and non-destructive: it skips every file that exists, so a re-run creates nothing.
3. **Evaluate** - `bootstrap_check.py <key> --lang <lang> --strict`, a separate pass and never the
   stamping step's self-report. Four file-level facts: every template element present, zero dangling
   path references, zero stale previous-stack references, extractor exists with production files.
4. **Record** - `loop1_ledger.py` appends `loop1/scaffold` to `verticals/<key>/LEDGER.md` from the
   facts just measured (stamped elements, extractor files, arms), and skips if the entry is already
   there. The LESSON field stays empty on a clean walk on purpose: a loop that reports a lesson
   every time it runs is manufacturing them.
5. **Return** - the JSON above.

Three things the stamp is checked FOR, because they are product duties and not paperwork:
`internal/extract/<lang>/` covers the stack's dispatch idioms; `detectors_<lang>.go` and the
framework model exist (absence is a stack-maturity item for the conventions ledger, not a blocker);
and the previous vertical's misuse lessons are carried into the arm wiring here, not debugged in
Loop 4.

## Stop

- **Done:** status `BOOTSTRAPPED`. Move to Loop 2, which writes its own `pool.txt`.
- **Extractor not ready:** status `EXTRACTOR-NOT-READY`. This is a **sequencing decision, not a
  bootstrap task**: the lane parks until language support lands, or the gap becomes a Loop 7 work
  item first. Verticals are parallel lanes, so only paid cells block on extractor readiness and $0
  work in the lane continues. **Loop 1 never builds product code.**
- **Budget:** one session. More than that means something upstream is wrong, usually an extractor
  gap found late.

## Where it stops and asks

| Event | Fires when | Blocking? | Demotable? |
|---|---|---|---|
| Stack confirm | entry, the Loop 0 handoff | yes | never (Loop 0 anchor) |
| Extractor verdict | the existence check passes but coverage is doubtful | yes | after clean history |

The second is a real gap in the mechanical check: `bootstrap_check.py` asks whether
`internal/extract/<lang>/` exists with production files, not whether it covers this stack's dispatch
idioms. Those differ, so the ambiguous case is a human call.

## State

- The scaffold IS the state - `new-vertical.sh` is idempotent, so "where the loop is" is readable
  from what exists on disk. **No `.loop-state.json`; do not add one.**
- A lesson from the previous vertical graduates into its durable home (a prompt, a one-pager, a
  gate, a memory) at the moment it is learned. There is no staging file and none is to be added;
  `STATUS.md` is the one pickup render.

## Fixture test (standalone, $0)

- **Known-answer control, both directions:** `bootstrap_check.py <stamped-key> --lang <lang>` exits
  0; `not-a-vertical` exits 1 with `structure: no verticals/not-a-vertical/ directory`. A check that
  only ever passes is not a check.
- **Every exit path returns JSON:** run with no `--lang`, with a `--lang` that has no extractor, and
  from outside the sense repo. All three print a parseable verdict and a distinct exit code. A loop
  that fails silently cannot be orchestrated.
- **Idempotence:** a second run creates nothing and leaves exactly one `loop1/scaffold` entry.
- **The dangling-ref check has teeth:** citing a path that does not exist FAILS; citing a real one
  PASSES.
- **The stale-ref scan has teeth:** plant a previous stack's key in a stamped `repos.md` and
  `--stale <that-key> --strict` FAILS naming the file and count; remove it and it goes green.

## Built vs missing

- **Built:** `bench/drivers/loop1-bootstrap.sh` (the loop), `bench/drivers/new-vertical.sh` (the
  stamp), `bench/lib/bootstrap_check.py` (the evaluator), `bench/lib/loop1_ledger.py` (the record).
- **Missing:** the arm plan has no mechanical check - it is prose in `verticals/<key>/README.md`
  that a human reads. Small wiring, not design work; if it recurs as a miss it becomes an
  enforcement item.
- **Missing:** the extractor verdict's coverage half, per the human event above.
