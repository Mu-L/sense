Re-render and read improvement-loop/verticals/ruby-rails/STATUS.md
(`bash improvement-loop/bench/lib/render-status.sh ruby-rails`).

It carries TWO positions, and they are separate lines of work:

- **Cycle 1** (`.loop-state.json`) crafts a scenario per repo until it wins on the
  headline arm. A repo at `done` is finished; there is nothing to resume there.
- **Cycle 2** (`.cycle2-state.json`) takes a banked win and puts it to the other
  models, then publishes the board. This is the live line while every repo in
  cycle 1 reads `done`.

Continue from whichever section shows unfinished work, using the resume command
printed under it. If both are finished, the vertical is done.
