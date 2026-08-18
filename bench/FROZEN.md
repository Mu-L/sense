# FROZEN

**This tree stopped moving on 2026-08-18**, at the commit that adds this record.
Apart from the record itself, the last commit to change anything under it is
`98a7789` (2026-08-13).

`bench/` was the global competitive bench: one board across many repositories,
Sense against a baseline and against other tools. It is replaced by the
comparison in `lab/`, which scores baseline, Sense and a competitor side by side
on one scenario at one budget, through the same measurement as everything else.

`improvement-loop/`, the vertical instrument, froze earlier in cycle 07 and
carries its own record.

## What frozen means

- **no new runs** here; all measurement happens in `lab/`
- **no edits, including fixes.** A frozen tree with a known defect is readable.
  A tree someone patched on the last day is a tree whose final state nobody can
  reason about
- **it stays in the repository** as history: 2,473 files, of which 464 are
  recorded transcripts, plus the incident write-ups that are the reason several
  rules in the new instrument exist
- **nothing reads it at runtime.** The only mentions anywhere are run-path
  strings inside two of the new instrument's test fixtures, which name a
  `bench/` directory *inside a campaign tree* and have nothing to do with this
  one. Checked rather than assumed: with this whole tree moved out of the
  repository, `go test -count=1 ./...` in `lab/` passes in every package
- its documents — `README.md`, `SCORING.md`, `end-goal.md`, the driver scripts —
  still read as live instructions. They are **historical**

**Deletion is a separate decision, later.** A frozen tree costs disk and nothing
else, and the write-ups under `results/` are the record of how several
measurement rules were learned.

## What is in it

| | |
|---|---|
| verticals | `ruby-rails` (246 transcripts), `python-django` (118), `go` (64) |
| global results | `results/`, with `baseline/`, `sense/`, `probe/` and one competitor tree (`gitnexus/`) |
| pinned repositories | 58, in `PINNED_COMMITS.json`, captured 2026-05-16 |
| write-ups kept | the judge calibration and variance notes, the citation-hallucination note, and the provenance incident of 2026-07-11 |

## Why it is retired rather than carried across

Its adoption layer measured code-intelligence tools against each other on axes
that only make sense between such tools. The replacement deliberately does not
carry that: a competitor is scored on **the same cited recall as everything
else**, so a competitor result can be checked against the rest of the corpus
rather than standing alone on a scale of its own.

**Nothing from this tree has been published, and nothing from its successor has
either.** That decision is taken separately, with the leakage question in view.
