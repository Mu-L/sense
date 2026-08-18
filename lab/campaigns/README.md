# campaigns

One directory per campaign, holding `campaign.json`: the subjects, the
repositories, the arms and the judge.

```json
{"key": "ruby-rails", "judge": "claude-opus-4-7",
 "subjects": ["untreated", "sense-main"], "repos": ["discourse"],
 "arms": [{"role": "headline", "model": "claude-opus-5", "runs": 2}]}
```

`sense-lab plan -campaign <key>` shows every cell it would run and every
rejection with its reason, before anything spawns. `sense-lab status` reads
where a campaign stands from its run tree.

Empty on purpose. The campaigns this instrument was built against were removed
once it was finished; they were how it was crafted rather than what it is for.
