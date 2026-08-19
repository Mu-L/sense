# benches

One file per repository, `<repo-id>.json`: how that repository is measured —
the subjects, the arms and the judge.

```json
{"repo": "discourse", "judge": "claude-opus-4-7",
 "subjects": ["untreated", "sense-main"],
 "arms": [{"role": "headline", "model": "claude-opus-5", "runs": 2}]}
```

`sense-lab plan -repo <id>` shows every cell it would run and every rejection
with its reason, before anything spawns.

It is a separate file from `repos/<id>.json` because the two have different
authors and different lifetimes: what a repository IS is written by admission,
and how it is measured is written by a person.

Empty on purpose. The benches this instrument was built against were removed
once it was finished; they were how it was crafted rather than what it is for.
