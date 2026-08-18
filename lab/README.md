# sense-lab

The bench instrument for Sense: it runs a coding agent twice over the same
repository — once with Sense and once without — and reports how much of what a
good answer would cite each arm actually cited.

The number it produces is only worth anything if the two arms differed in Sense
access and nothing else, so most of what is here exists to prove that. Read
[`LAWS.md`](LAWS.md) before changing how anything is measured, and
[`KILLERS.md`](KILLERS.md) before reporting a finding.

## Build

```bash
make build          # → bin/sense (the product) and bin/sense-lab (this)
./bin/sense-lab     # the command list
```

Every command takes `-config`, the directory holding `subjects/`, `agents/`,
`models/`, `repos/` and `executors/`. It defaults to `lab`, so run these from
the repository root.

## The shortest useful path

Start by looking at what is already declared:

```bash
./bin/sense-lab catalog
```

Then, to bench one repository:

**1. Declare the repository** — `lab/repos/<id>.json`:

```json
{"id": "discourse", "url": "https://github.com/discourse/discourse.git",
 "commit": "d73e4484b4b9dcffa1e75e2ceff6e0a005d479c6",
 "languages": ["ruby"], "stack": "ruby"}
```

Clone it locally and check it out at that commit. Every run takes a worktree
from your clone at the pinned commit, so the clone is the only thing that has to
exist on disk.

**2. Write the scenario** — three files beside each other in
`lab/scenarios/<id>/`:

| file | what it is |
|---|---|
| `<id>.yaml` | `name`, `repo`, `contract_symbol`, `description`, and `steps` (each a `name` and a `prompt`) |
| `<id>.gold.yaml` | `discriminator` (the group carrying the headline) and `rows`: `id`, `group`, `relation` |
| `<id>.rubric.yaml` | `audience`, and per step the judge's weighted `criteria` |

A gold row's `relation` opens with the row's authoritative `path:line` and then
says why it matters, in prose. That leading location is what the scorer matches
against — a row with no `path:line` can never be matched, and a group where no
row has one is refused rather than scored zero.

The shipped scenarios carry extra keys (`contract_file`, `system_prompt`,
`checks`, `scoring`) that the loader ignores. They are history, not schema.

**3. Audit the gold before spending anything:**

```bash
./bin/sense-lab validate -scenario lab/scenarios/<id>/<id>.yaml \
    -checkout /path/to/clone -commit <sha>
```

It reports what would be quarantined and why. Without `-checkout` and `-commit`
two of its checks report `NOT CHECKED`.

**4. Run one cell, both arms:**

```bash
./bin/sense-lab probe \
    -agent claude-code -model claude-opus-5 \
    -repo <id> -checkout /path/to/clone \
    -scenario lab/scenarios/<id>/<id>.yaml \
    -out /tmp/cell -sense ./bin/sense -wall 6m
```

It prints the pair's soundness: which routes each arm reached, whether persisted
memory was readable, whether the arms differed in anything but Sense access, how
many of each arm's tool calls reached Sense, and how long each took to say
anything. A pair that fails any of it is printed as `NOT A MEASUREMENT` and its
number may not be cited.

**5. Score it:**

```bash
./bin/sense-lab score -run /tmp/cell/sense/session \
    -scenario lab/scenarios/<id>/<id>.yaml -group <group> \
    -checkout /path/to/clone -commit <sha>
```

`-group all` scores every group. Passing the checkout is what lets it say
whether the citations resolve; without it, grounding reads `NOT VERIFIED`.

## Running a campaign rather than a cell

A campaign is `lab/campaigns/<key>/campaign.json`:

```json
{"key": "dotnet-jellyfin", "judge": "claude-opus-4-7",
 "subjects": ["untreated", "sense-main"], "repos": ["jellyfin"],
 "arms": [{"role": "headline", "model": "claude-opus-5", "runs": 2}]}
```

```bash
./bin/sense-lab plan -campaign dotnet-jellyfin     # every cell, and every rejection with its reason
./bin/sense-lab status -campaign runs/campaigns/dotnet-jellyfin
```

`plan` refuses impossible combinations before anything spawns — a model no
agent can drive, a subject that needs MCP on a tool that has none, an executor
that preserves no auth mode for a model that needs one. `status` derives where a
campaign stands from its run tree rather than from a state file anybody edits.

`gate` reads a pay decision as JSON on stdin and reports every rule that refuses
it, not just the first.

## Everything is config as data

Nothing in the binary knows the name of an agent tool, a model or a competitor.
Adding one is a file.

| directory | one file per | what it declares |
|---|---|---|
| `agents/` | agent tool | binary, headless args, how a model is selected, its transcript format, how it takes a wall note, where its credential lives and which fields of it a run may have, where it registers MCP servers |
| `models/` | model | provider, aliases, which auth modes reach it, which agent tools can drive it |
| `subjects/` | treatment | baseline, sense or competitor; what it installs, sets up and cleans up; the paths it may touch |
| `repos/` | repository | url, pinned commit, languages, stack |
| `executors/` | where a run happens | which auth modes survive into it, whether it isolates global config |
| `scenarios/` | scenario | the question, the gold, the rubric |
| `campaigns/` | campaign | subjects × repos × arms, and the judge |

Each `agents/<id>/` and `subjects/<id>/` also carries a `README.md` of measured
operational facts — what that tool actually does, dated and tied to a version.
Those are written from runs, never from a vendor's documentation.

## Before you spend a run

Each agent tool needs its own login present on this machine. The parent reads it
once and provisions a narrowed copy into the run; a session never reaches a
keychain.

| tool | where its login lives | what a run is given |
|---|---|---|
| claude-code | the platform keychain, or `~/.claude/.credentials.json` | an access token, its expiry and its scopes — no refresh token |
| codex | `~/.codex/auth.json` | the same document minus nothing: it was measured to need its refresh token |
| opencode | `~/.local/share/opencode/auth.json` | one provider's key, handed over in an environment variable |

The per-agent READMEs carry the measurements behind those rows.

## Things that will bite you once

- **The model id is the catalog id.** `kimi-for-coding/k3`, not the filename.
- **A gold row without a `path:line` can never be matched**, and a group where
  no row has one is refused rather than scored zero. That is why the shipped
  `rails` scenario is quarantined: not one of its 25 rows carries a line.
- **Pass `-checkout` and `-commit` to `score`**, or you will not know whether
  the citations you scored exist.
- **Run `git worktree prune` in your clone** if a run dies part way. The next one
  refuses a path git still has registered.
- **A cell's runs are immutable.** `-out` must not already hold a run; point it
  somewhere new rather than deleting evidence.

## Where the rest of it is written down

- [`LAWS.md`](LAWS.md) — the rules a measurement has to satisfy
- [`KILLERS.md`](KILLERS.md) — the checks a finding has to survive before it is
  reported
- [`plans/`](plans) — one file per phase of the authoring loop: what that phase
  reads, writes and may emit
- `executors/README.md`, and the README beside each agent and subject
- `../improvement-loop/FROZEN.md` and `../bench/FROZEN.md` — the two instruments
  this one replaced, and what they ended with
