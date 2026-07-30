#!/usr/bin/env python3
"""arms.py - read a vertical's arms (the tools/LLMs it benches) from ONE file.

The single source of truth is `verticals/<key>/arms.txt`, stamped from
`docs/arms.default.txt`. Nothing else names a model id, so superseding a release is one
edit instead of a grep across drivers, analysis tools and docs. The shell side of this is
`arms.sh`; keep the two in step.

    import arms
    arms.headline("php-laravel")            # 'claude-opus-4-8'
    arms.models("php-laravel")              # every arm, headline first
    arms.models("php-laravel", "confirmation")
    arms.judge("php-laravel")               # the pinned judge
    arms.runs("php-laravel", "gpt-5.5")     # 2

`vertical` defaults to $VERTICAL. A missing arms.txt raises: a silent fallback to a
hardcoded id is the rot this module exists to remove.
"""

import os

IL_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def _path(vertical=None):
    v = vertical or os.environ.get("VERTICAL", "")
    if not v:
        raise ValueError("arms: no vertical given and VERTICAL is unset")
    return os.path.join(IL_ROOT, "verticals", v, "arms.txt")


def _rows(vertical=None):
    p = _path(vertical)
    if not os.path.isfile(p):
        raise FileNotFoundError(f"arms: no arms file at {p}")
    out = []
    with open(p, encoding="utf-8") as fh:
        for line in fh:
            line = line.split("#")[0].strip()
            if not line:
                continue
            parts = line.split()
            if len(parts) < 2:
                continue
            out.append((parts[0], parts[1], int(parts[2]) if len(parts) > 2 else 2))
    return out


def models(vertical=None, role=None):
    rows = _rows(vertical)
    if role:
        return [m for r, m, _ in rows if r == role]
    # headline first, then confirmation; the judge is never an arm under test
    return ([m for r, m, _ in rows if r == "headline"]
            + [m for r, m, _ in rows if r == "confirmation"])


def headline(vertical=None):
    got = models(vertical, "headline")
    if not got:
        raise ValueError(f"arms: no headline arm in {_path(vertical)}")
    return got[0]


def judge(vertical=None):
    got = models(vertical, "judge")
    if not got:
        raise ValueError(f"arms: no judge in {_path(vertical)}")
    return got[0]


def runs(vertical=None, model=None):
    for _r, m, n in _rows(vertical):
        if m == model:
            return n
    return 2
