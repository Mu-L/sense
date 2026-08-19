# repos

One file per repository the lab benches: `<id>.json` with the repository's url,
the commit it is pinned at, its languages and its stack.

```json
{"id": "discourse", "url": "https://github.com/discourse/discourse.git",
 "commit": "d73e4484b4b9dcffa1e75e2ceff6e0a005d479c6",
 "languages": ["ruby"], "stack": "ruby"}
```

`sense-lab repo <handle|url|path>` writes one of these, and how that repository
is measured is declared beside it in `../benches/<id>.json`. The commit is read back
out of the clone rather than copied by hand, and `checkout` is present only for
a clone somebody handed in: its absence is what says the lab made this one and
may move it back to its pin.

Empty on purpose. The repositories this instrument was built against were
removed once it was finished, because they were how it was crafted rather than
what it is for. See `../README.md` for the path from here to a scored cell.
