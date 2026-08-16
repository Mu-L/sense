# transcript fixtures

Real captured runs, checked in byte for byte. They are shapes the recorded
corpus actually contains, not shapes someone imagined.

| Fixture | What it is |
|---|---|
| `claude-normal.jsonl` | a Claude Code session with text, tool calls and usage. 2 text blocks, 14 tool calls, 43 fresh input tokens against 845,014 cache reads |
| `result-usage-arm.jsonl` | usage on the CLOSING EVENT and nowhere else, which is how three of the five arms report. 27 text blocks, 81 tool calls, 1,790,899 in / 5,502 out |
| `errored-arm.jsonl` | a session the tool reports as having ended badly. 2 text blocks, 35 tool calls, parses cleanly, closes properly — every check except `is_error` passes it |
| `no-usage-arm.jsonl` | a session that genuinely reports no usage anywhere |
| `empty-arm.jsonl` | **the mistral arm.** One line: a `result` event with all-zero usage. The model id resolved to nothing and the session produced nothing |

## The corpus sweep

Run against every `transcript.json` under `improvement-loop/`. **It is
re-runnable**, and it was re-run after each change below; an earlier version of
this file called it unrepeatable, which was not true and was protecting nothing.

```
transcripts:        238
hard errors:        0
unparseable lines:  0
no session id:      0
no usage recorded:  16
provisional:        40
```

### The 40, by what marked them

| Reason | Count | Arm |
|---|---|---|
| the tool reported the session ended badly (`is_error`) | 27 | claude-opus-5 |
| the agent said nothing | 8 | mistral |
| the agent said nothing | 5 | kimi |

Three findings came out of this, and all three changed the code.

**The 13 silent runs are the two arms cycle 01 is named for.** Eight mistral and
five kimi. The normalizer rediscovered that incident from the corpus without
being told about it. Scored without the mark, those runs read as arms that found
nothing.

**The other 27 were scoring as clean results, and they are all on the HEADLINE
arm.** They have text, they parse with zero unreadable lines, and they close
properly — so the first three conditions all pass them. Only `is_error` on the
closing result event sees them. Checked against the runs' own recorded exit
codes: of the 104 transcripts whose sibling record carries one, **104 agree with
`is_error` and none disagree**, so it is a fact rather than a noisy heuristic.
Most of the 27 are wall timeouts.

**Usage was being lost on half the corpus, and this file said so as though it
were a fact about the tools.** An earlier sweep reported "133 of 238 carry no
usage; only the Claude arm reports it in this shape". That was wrong. 117 of
those 133 report usage on the **closing result event**, under the identical
field names, and the reader was skipping it because it only looked at assistant
turns. The true figure is **16 of 238**.

So `Usage.Reported` was doing precisely the thing it exists to prevent — saying
"the tool never told us" when the tool told us one line later — on 49% of the
corpus. The reader takes the closing event's totals when the turns carry none,
and never as well as them, since the closing event reports a session total and
adding both would double every figure on the arm that sends both.

The lesson is not about usage. **A number measured once and written into prose
cannot be wrong loudly.** Both of this file's original headline figures came
from a single sweep, and one of them was false. Every shape it now names has a
fixture and a test behind it.

### The condition the sweep could not find

A capture cut off **before** its closing event parses cleanly and has text, and
no count in the corpus reveals it, because every one of the 238 closed properly.
It is caught by requiring the closing result event, and tested against a real
complete capture with its last line removed — which is exactly what a killed
capture looks like.

### The signal that is not in the transcript at all

The strongest one. The runner knows whether it killed the session, and writes it
to `run-meta.json` beside the capture. The scorer reads that record and prefers
it, because it is a fact about the run rather than an inference from its bytes.
A capture can look perfect and still belong to a session that hit its wall.
