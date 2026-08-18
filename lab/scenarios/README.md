# scenarios

One directory per scenario, holding three files that are read together:

| file | what it is |
|---|---|
| `<id>.yaml` | `name`, `repo`, `contract_symbol`, `description` and the `steps` |
| `<id>.gold.yaml` | the `discriminator` group, and the rows a good answer cites |
| `<id>.rubric.yaml` | the judge's weighted criteria, per step |

A gold row's `relation` opens with its authoritative `path:line` and then says
why that location matters. That leading location is what the scorer matches: a
row without one can never be matched, and a group where no row has one is
refused rather than scored zero.

Audit gold before spending anything:

```
sense-lab validate -scenario scenarios/<id>/<id>.yaml -checkout <clone> -commit <sha>
```

Empty on purpose. The scenarios this instrument was built against were removed
once it was finished; they were how it was crafted rather than what it is for.
