#!/usr/bin/env python3
"""Bar 5 of the admission gate, mechanized: is this anchor MEMORIZED?

    memorization_probe.py <clone_dir> <Symbol> [--file HINT] [--model M]
                          [--json OUT] [--answer FILE] [--samples N]

WHY THIS EXISTS. Bar 5 was the last of Loop 2's human-checklist bars ("does the
discriminator live in obscure internals, or is it a famous public API the model
recites?"). Judgment cannot settle it: a frontier model's recall of a public API
is exactly the kind of thing humans guess wrong about in both directions. So
measure it - CLOSED BOOK.

THE PROBE. Ask the bench model, with no repo access and no tools, to name the
anchor's own members - its methods, constants and properties. Score against the
symbols the index holds for it. High recall with no access = the public API is
recited from weights, which is exactly what manifesto §7.1 warns about for a
framework anchor ("the baseline has MEMORIZED the public API, so the
discriminator MUST be the obscure non-memorized internals").

FIRST DESIGN, KILLED 2026-07-29: the probe asked for the DEPENDENT FILES instead.
`laravel-framework` `Model` - the most memorized class in PHP - answered UNKNOWN,
recall 0.0. Nobody memorizes another project's file tree, so that probe could
never reject anything. Members are what a model actually carries.

WHY THIS IS NOT AN OUTPUT-SHAPE PROXY. The probe measures the thing itself (can
the model produce the dependent set without the repo?), not a stand-in for it.
The failure it prevents is real and banked: a memorized public API ties because
BOTH arms already know the answer.

RUNS ON THE SUBSCRIPTION, not API credit - same `claude -p` path the bench arms
use, so it is unmetered. Tools are disabled and the cwd is a scratch dir, never
the clone: if the model could read the repo the probe would measure nothing.

CALIBRATION. `MEMORIZED_RECALL` is a floor, not a law. Run the probe on a known
famous anchor and a known obscure one in the same stack and put both numbers in
the vertical's `repos.md` before trusting a rejection. php-laravel 2026-07-29:
laravel `Model` 0.857 and `Dispatcher` 1.0 (recited) vs 0.0 on every admitted
anchor. `MIN_TRUTH_MEMBERS` exists because this bar calls a MODEL, so it is the
only non-deterministic step in Loop 2 - anchors with a tiny member set turned
that noise into flipped verdicts.

WHY MAX-OF-N, NOT ONE CALL. Measured: filament `HasTable` scored recall 0.0,
0.0, then 0.435 on three identical probes - the model sometimes answers UNKNOWN
and sometimes recites 20 of its 46 methods. Memorization is a CAPABILITY, so a
single UNKNOWN is a false negative and the max is the honest estimator: if the
model CAN produce the API once, both bench arms already know it. One sample per
anchor flipped verdicts between otherwise identical cold runs.
"""

import argparse
import json
import os
import re
import sqlite3
import subprocess
import sys
import tempfile

MEMORIZED_RECALL = 0.30   # recall at/above this with no repo access = recited
MIN_TRUTH_MEMBERS = 5     # below this the recall fraction is a coin flip, see below
SAMPLES = 3               # take the MAX recall over N calls, never one sample
MIN_HITS = 5              # ...and a fraction alone never rejects, see below
PROMPT = """You are being asked what you already know. You have NO access to any
repository, and you must not ask for one. This is a recall test, not a task.

In the open-source project `{repo}`, there is a `{symbol}` defined in `{path}`.
From memory alone, list its members: the methods, constants and properties
declared ON `{symbol}` itself.

Rules:
- One name per line, nothing else. No signatures, no prose, no apology.
- Partial knowledge IS the answer. List every name you have ANY recollection of,
  even if you are unsure and even if you can only manage two or three.
- Do NOT decline for lack of certainty. Uncertainty is expected and is fine.
- Do not pad with invented generic names; an unsure real name beats a made-up one.
- Only if you have never encountered this symbol at all, output UNKNOWN.

Answer now, with names."""


def truth_members(clone, symbol, file_hint):
    """The anchor's own members, straight from the index: children of the symbol,
    public surface only (a private helper is not what "memorized API" means)."""
    db = os.path.join(clone, ".sense", "index.db")
    if not os.path.exists(db):
        return []
    con = sqlite3.connect(f"file:{db}?mode=ro", uri=True)
    q = ("SELECT DISTINCT c.name FROM sense_symbols c "
         "JOIN sense_symbols p ON p.id = c.parent_id "
         "JOIN sense_files f ON f.id = p.file_id "
         "WHERE p.name = ? AND c.visibility != 'private'")
    args = [symbol]
    if file_hint:
        q += " AND f.path = ?"
        args.append(file_hint)
    rows = [r[0] for r in con.execute(q, args).fetchall()]
    con.close()
    return sorted({r for r in rows if r and not r.startswith("__")})


def ask_model(repo, symbol, path, model):
    """Closed book: scratch cwd, no tools, no repo. Subscription `claude -p`."""
    prompt = PROMPT.format(repo=repo, symbol=symbol, path=path)
    with tempfile.TemporaryDirectory() as scratch:
        cmd = ["claude", "-p", prompt, "--allowedTools", ""]
        if model:
            cmd += ["--model", model]
        p = subprocess.run(cmd, cwd=scratch, capture_output=True, text=True,
                           stdin=subprocess.DEVNULL, timeout=300)
    if p.returncode != 0:
        return None, (p.stderr or "").strip()[:300]
    return p.stdout, None


def score(answer, truth):
    """Recall over member NAMES. Credit is deliberately generous - any bare word
    in the answer counts as a claim - because this bar rejects, so it must not
    under-count recitation."""
    if len(truth) < MIN_TRUTH_MEMBERS:
        # Two cases, one verdict. (a) A marker interface declares no members, so
        # there is no API to recite (akaunting `ShouldCreate`). (b) A tiny member
        # set makes the recall FRACTION meaningless: snipe-it `Presentable` has
        # exactly one member, `present`, so the model using that ordinary English
        # word anywhere scores recall 1.0 and the anchor is rejected - measured,
        # it flipped 3 of 326 cells between two identical cold runs and changed
        # the slate's big slot. Below the floor the bar is INAPPLICABLE, not
        # failed: "unrun bar 5" must never admit, but this is neither unrun nor a
        # risk, and a bar that cannot measure must not reject.
        return {"recall": None, "hits": [], "applicable": False, "truth_n": len(truth),
                "reason": f"only {len(truth)} member(s) declared (< {MIN_TRUTH_MEMBERS}) - "
                          "too few to score recitation, bar inapplicable"}
    said = {w.lower() for w in re.findall(r"[A-Za-z_][A-Za-z0-9_]{2,}", answer or "")}
    hits = [t for t in truth if t.lower() in said]
    # hits_n is the FULL count; `hits` is truncated for readability, so never
    # measure off it (MIN_HITS above 20 would have silently stopped firing).
    return {"recall": round(len(hits) / len(truth), 3), "hits_n": len(hits),
            "hits": hits[:20], "truth_n": len(truth), "said_n": len(said)}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("clone")
    ap.add_argument("symbol")
    ap.add_argument("--file", dest="file_hint", default=None)
    ap.add_argument("--model", default=os.environ.get("BENCH_MODEL"))
    ap.add_argument("--json", dest="json_out", default=None)
    ap.add_argument("--samples", type=int, default=SAMPLES,
                    help=f"model calls per anchor; the MAX recall wins (default {SAMPLES})")
    ap.add_argument("--answer", default=None,
                    help="score a saved answer instead of calling the model ($0 replay)")
    args = ap.parse_args()

    repo = os.path.basename(os.path.abspath(args.clone))
    truth = truth_members(args.clone, args.symbol, args.file_hint)

    samples, err, best, answer = [], None, None, None
    if args.answer:
        answer = open(args.answer).read()
        best = score(answer, truth)
    else:
        for _ in range(max(1, args.samples)):
            a, err = ask_model(repo, args.symbol,
                              args.file_hint or "(unknown path)", args.model)
            if a is None:
                continue
            sc_i = score(a, truth)
            samples.append(sc_i.get("recall"))
            # MAX, not mean: see WHY MAX-OF-N above.
            if best is None or (sc_i.get("recall") or -1) > (best.get("recall") or -1):
                best, answer = sc_i, a
            if best.get("recall") is None:      # inapplicable - no point resampling
                break
    if best is None:
        out = {"ok": False, "ran": False, "reason": f"probe failed to run: {err}"}
    else:
        sc = best
        sc["samples"] = samples
        r = sc["recall"]
        if r is None:
            out = {"ok": sc.get("applicable") is False, "ran": True,
                   "reason": sc["reason"], **sc}
        else:
            # A FRACTION alone is not recitation. akaunting's `Jobs` trait has 5
            # members and the model named `dispatch` and `dispatchSync` - 0.4
            # recall from two words any model that has seen Laravel produces.
            # Reciting an API means naming SEVERAL of its members, so require an
            # absolute hit count too. Sanity against the knowns: laravel `Model`
            # ~121 hits, filament `HasTable` ~30 - both still reject.
            hits_n = sc.get("hits_n", len(sc.get("hits") or []))
            memorized = r >= MEMORIZED_RECALL and hits_n >= MIN_HITS
            if r >= MEMORIZED_RECALL and hits_n < MIN_HITS:
                reason = (f"recall {r} clears {MEMORIZED_RECALL} but only {hits_n} "
                          f"member(s) named (< {MIN_HITS}) - generic framework "
                          "vocabulary, not recitation of THIS symbol")
            elif memorized:
                reason = (f"closed-book recall {r} ({hits_n} members named) >= "
                          f"{MEMORIZED_RECALL} - recited, both arms already know it")
            else:
                reason = f"closed-book recall {r} < {MEMORIZED_RECALL} - not recited"
            out = {"ok": not memorized, "ran": True, "model": args.model,
                   "reason": reason, **sc}
        out["answer_head"] = (answer or "")[:400]

    out["symbol"], out["repo"] = args.symbol, repo
    print(json.dumps({k: v for k, v in out.items() if k != "answer_head"}, indent=1))
    # NEVER persist a probe that did not run. A transient `claude -p` failure
    # cached as a verdict file turns into a PERMANENT reject for that anchor:
    # the gate correctly refuses to admit on a failed bar, the driver sees a
    # memo on disk and never retries, and the anchor is silently dead. Measured
    # Measured: 1 of 18 memos failed mid-run and dropped october `Blueprint`
    # out of the slate's backup set with no visible error anywhere.
    if args.json_out and out.get("ran"):
        with open(args.json_out, "w") as f:
            json.dump(out, f, indent=2)
    return 0 if out.get("ran") else 1


if __name__ == "__main__":
    sys.exit(main())
