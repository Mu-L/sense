# repos

One file per repository the lab benches: `<id>.json` with the repository's url,
the commit it is pinned at, its languages and its stack.

```json
{"id": "discourse", "url": "https://github.com/discourse/discourse.git",
 "commit": "d73e4484b4b9dcffa1e75e2ceff6e0a005d479c6",
 "languages": ["ruby"], "stack": "ruby"}
```

Empty on purpose. The repositories this instrument was built against were
removed once it was finished, because they were how it was crafted rather than
what it is for. See `../README.md` for the path from here to a scored cell.
