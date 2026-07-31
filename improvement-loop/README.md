# improvement-loop

The loop that proves Sense helps an AI agent on a real stack, and turns what it finds into product
changes. Baseline vs baseline+Sense on a vertical of real repos, scored on cited recall, judged against
a reference.

This folder is **self-contained**: everything the loop runs and everything it writes lives under it.
No script here reaches into the private docs tree, and no script outside here is needed to run a vertical.

Three things, and each is one thing: the **scripts**, the **rule-book**, and one folder per
**vertical** holding everything about that stack.

```
improvement-loop/
├── bench/                all scripts (own scoring core: gold / scorer / scenario)
│   ├── bootstrap/        one command that stands a vertical up: bash bench/bootstrap/run.sh
│   ├── lib/              orchestration, gates, scoring
│   └── drivers/          the loop drivers
├── docs/                 the rule-book: manifesto, judging contract, loop one-pagers,
│                         cross-cutting fact-packs, scenario + findings method
└── verticals/<key>/      ONE home per vertical: repos.txt, scenarios/, results/ (private),
                          README, repos.md, articles/, LEDGER.md (private), STATUS.md
```

A vertical is not split across a data half and a docs half. `verticals/<key>/` is the whole thing,
so a campaign is one folder to open, one folder to archive, and one folder to delete.

## Public by default

The `.gitignore` default **inverts** here. Everything under `improvement-loop/` is tracked unless it
matches the small private allowlist:

| Kind | Example | Tracked? |
|---|---|---|
| Rule-book | manifesto, judging contract, scripts, the loop's own scorer | **yes** - this is the point of publishing |
| Render | `STATUS.md` | yes |
| Journal | `LEDGER.md`, `decision-errors.md` | **no** - candid self-directed reasoning; worth more than the transparency |
| Results | run cells, transcripts | **no** - large, and provider model output |

## Its relationship to the global bench

- `bench/global/` - competition analysis (Sense vs competitors on the held-out anchor). Separate
  purpose, separate scoring core, untouched by this folder. The two never compare numbers, so an
  independent scorer here costs no correctness.
- This folder is the vertical loop. There is no other.
