# Stack profiles - a bootstrap prerequisite

One file per queued vertical: `stacks/<key>.conf`, where `<key>` matches a line in
[`../verticals.txt`](../verticals.txt). **Written by hand** (by a human or an LLM) before
`prepare-vertical.sh` will touch that vertical, and checked as a precondition the same way the queue
itself is: a missing or incomplete profile stops the pipeline with a named reason, it never guesses.

It is not generated, on purpose. The queries are the search that a later "the pool is exhausted"
claim rests on, and the framework roles are a judgment about the ecosystem. Both belong in a file
someone reviewed and committed, not in whatever a session improvised.

## The three keys

### `stack:` - required, exactly one

    stack: <manifest>:<needle>|<needle>|...

The in-vertical screen. A repo is in this vertical when its own dependency manifest declares the
stack; the pool's say-so does not count. Needles are matched as **ANY**, as plain substrings, in the
manifest at the repo root or one/two levels down.

Give more than one needle. Two things break a single needle, and both are measured:

- **Apps and packages declare differently.** A Laravel APP requires `laravel/framework`; a Laravel
  PACKAGE requires `illuminate/*`. One needle rejected filament and flarum, which was the framework
  slot's entire candidate list.
- **A framework's components get named instead of the framework.** Discourse declares `railties` and
  `actionpack` and never `gem "rails"`.

Quote style is per-project in Ruby, so list both forms.

A non-matching manifest is **not** a rejection: the screen is three-state, and present-but-no-match
falls through to the clone, because a monorepo declares nothing at its root (filament's root
`composer.json` is empty of requirements). A thin needle list therefore costs clones, not
correctness.

### `hunt:` - required, one or more

    hunt: <raw gh search repos ARGV>

Raw `gh search repos` arguments, **not** a query string. A pure-qualifier query string collapses into
a single search TERM and gh answers "none of the search qualifiers apply" - measured, that silently
killed 3 of 4 queries and left the pool at whatever the one keyword query happened to find. Write
`--language php --size ">40000"`, never `"language:php size:>40000"`.

Cover more than one **axis**, not more than one wording. A name query and a topic query return the
same famous repos. The axis that pays is repo SIZE, because it finds large applications nobody can
name: the name-and-topic hunt produced exactly ONE big non-framework app across 28 php candidates,
and adding `--size` found three more, including the one that filled the big slot's backup.

The four axes that have earned their place so far:

    hunt: --language <lang> --topic <framework> --stars ">1000"
    hunt: <framework> --language <lang> --stars ">2000"
    hunt: --language <lang> --size ">40000" --stars ">300"
    hunt: --language <lang> --topic self-hosted --stars ">1000"

The hunt is allowed to be greedy and wrong. Everything it proposes is verified downstream by
`repo_screen.py` against the repo's own manifest, its API facts and its file count, so a bad
candidate costs a screen, never a bad slate.

### `framework:` - optional, one per line

    framework: <owner>/<name>

Framework-ROLE repos: something others build ON. Eligible for the §7.0 framework slot and never for
the big slot, because the pillar rule keeps the framework from being the campaign's sole win.
Anything the hunt finds that is not listed here is screened as an application.

This is declared because it is **not derivable**, and that was measured rather than assumed:
`composer.json` `type` is unset for laravel/framework, filament, statamic and flarum, and `project`
for october and winter. The manifest does not separate a framework from an application.

## Check it

    python3 bench/lib/stack_profile_check.py <key>

`prepare-vertical.sh` runs the same check before it stamps anything, so a bad profile fails before a
vertical is half-created rather than during the hunt.

## Worked example

[`php-laravel.conf`](php-laravel.conf) is the reference. [`ruby-rails.conf`](ruby-rails.conf) is the
second one written, and its `stack:` needles are the part to read critically - a framework's own repo
often does not declare itself the way its applications do.
