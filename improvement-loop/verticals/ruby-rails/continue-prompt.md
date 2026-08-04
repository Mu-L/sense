Re-render and read improvement-loop/verticals/ruby-rails/STATUS.md
(bash improvement-loop/bench/lib/render-status.sh ruby-rails), then continue from the loop
position it shows.

Two things STATUS.md will not tell you:

1. Its matrix section prints "_No model results yet._". That is a READER gap, not a missing
   result: `matrix.py` walks `results/<model>/<arm>/<repo>/run-*` and the
   one-root-per-question migration moved the tree to
   `results/<model>/<version>/<arm>/<repo>/run-*`. The banked cells are reported by
   `pergroup.py` at a versioned root:

       RESULTS_DIR=verticals/ruby-rails/results/claude-opus-5/27dfc6000a5e98f3 \
         python3 bench/lib/pergroup.py mastodon      # dependents +0.78, write-path +0.50
       RESULTS_DIR=verticals/ruby-rails/results/claude-opus-5/3f210bcde96c18e1 \
         python3 bench/lib/pergroup.py rails         # dependents +0.56

2. `rails` and `mastodon` are `done` and banked; the open repo slots are `discourse` (big)
   and `chatwoot` (medium), both cloned and indexed, neither with a scenario. Start one with
   `VERTICAL=ruby-rails bash bench/drivers/vertical-loop.sh discourse`, which enters at
   `index` and spawns the author agent. One repo at a time, to a verdict.

The open question the next repo answers is whether the rebuilt loop crafts a winning scenario
UNATTENDED, a second time. Mastodon did it once (minibench -> expand -> preflight -> validate
-> bench -> harvest, no human in the chain). Once is an anecdote.
