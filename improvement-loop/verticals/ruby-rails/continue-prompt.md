Read improvement-loop/docs/pickup.md first - it carries the current numbers, the open
questions and the last ruling. Then re-render and read
improvement-loop/verticals/ruby-rails/STATUS.md
(bash improvement-loop/bench/lib/render-status.sh ruby-rails) for the loop position on disk.

STATUS.md's matrix section currently prints "_No model results yet._" for both banked cells.
That is a READER gap, not a missing result: `matrix.py` walks
`results/<model>/<arm>/<repo>/run-*` and the one-root-per-question migration moved the tree
to `results/<model>/<version>/<arm>/<repo>/run-*`. The authoritative numbers come from
`pergroup.py` at a versioned root, e.g.

    RESULTS_DIR=verticals/ruby-rails/results/claude-opus-5/27dfc6000a5e98f3 \
      python3 bench/lib/pergroup.py mastodon
    RESULTS_DIR=verticals/ruby-rails/results/claude-opus-5/3f210bcde96c18e1 \
      python3 bench/lib/pergroup.py rails

rails and mastodon are both `done` and banked. The two open repo slots are `discourse` (big)
and `chatwoot` (medium): both cloned and indexed, neither has a scenario. Start one with
`VERTICAL=ruby-rails bash bench/drivers/vertical-loop.sh discourse`, which enters at `index`
and spawns the author agent. One repo at a time, to a verdict.
