#!/usr/bin/env python3
"""spawn_cost.py: $0 subscription-cost measurement from the session transcripts.

Answers the one question the ledger's Cost field could not: how much of a session
segment was the agent FLEET. Reads Claude Code's own transcripts (JSONL, one file
per session) and reports, per day:

  * Agent spawns by subagent_type (the fleet: council seats, evaluator, probes)
  * main-session effective tokens by model (cache reads weighted, see EFF below)

Same shape and station as transcript_miss.py: a read-only detector over recorded
artifacts, no API spend, no state.

CORRECTION 2026-07-16: the original honest-gap claim ("subagent turns carry NO
usage record anywhere on disk") was WRONG. Subagent transcripts live in
<project>/<session-id>/subagents/agent-*.jsonl with a .meta.json carrying the
agentType, and every turn has a full usage record. The first survey only read the
top-level *.jsonl files. The fleet section below prices them; main-session and
fleet tokens are reported separately because they come from different files.

Usage: python3 spawn_cost.py [--since YYYY-MM-DD] [--until YYYY-MM-DD]
       defaults: the last 7 days.
"""

import argparse
import collections
import datetime
import json
import pathlib

PROJECT = pathlib.Path.home() / ".claude" / "projects" / "-Users-luc-Developer-luuuc-oss-sense"

# Cache reads bill at a fraction of input, cache writes at a premium; weight them so
# one "effective token" is comparable across turns instead of dominated by cache reads.
EFF = {"input_tokens": 1.0, "output_tokens": 1.0, "cache_read_input_tokens": 0.1, "cache_creation_input_tokens": 1.25}


def effective(usage):
    return sum(w * usage.get(k, 0) for k, w in EFF.items())


def fleet(since, until):
    """Return per-day effective tokens by agentType from the subagent transcripts."""
    burn = collections.defaultdict(collections.Counter)
    for meta_path in PROJECT.glob("*/subagents/*.meta.json"):
        transcript = meta_path.with_suffix("").with_suffix(".jsonl")
        if not transcript.exists():
            continue
        try:
            agent_type = json.loads(meta_path.read_text()).get("agentType", "(unknown)")
        except ValueError:
            continue
        for line in transcript.open(encoding="utf-8", errors="replace"):
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            stamp = rec.get("timestamp", "")[:10]
            if not (since <= stamp <= until):
                continue
            usage = (rec.get("message") or {}).get("usage")
            if usage:
                burn[stamp][agent_type] += effective(usage)
    return burn


def scan(since, until):
    """Return (spawns, tokens, sidechain_seen) keyed by day."""
    spawns = collections.defaultdict(collections.Counter)
    tokens = collections.defaultdict(collections.Counter)
    sidechain_seen = 0
    for path in PROJECT.glob("*.jsonl"):
        for line in path.open(encoding="utf-8", errors="replace"):
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            stamp = rec.get("timestamp", "")[:10]
            if not (since <= stamp <= until):
                continue
            if rec.get("isSidechain"):
                sidechain_seen += 1
            message = rec.get("message") or {}
            usage = message.get("usage")
            if usage:
                tokens[stamp][message.get("model", "?")] += effective(usage)
            content = message.get("content")
            if isinstance(content, list):
                for block in content:
                    if isinstance(block, dict) and block.get("type") == "tool_use" and block.get("name") in ("Agent", "Task"):
                        spawns[stamp][(block.get("input") or {}).get("subagent_type", "(default)")] += 1
    return spawns, tokens, sidechain_seen


def report(spawns, tokens, fleet_burn):
    days = sorted(set(spawns) | set(tokens) | set(fleet_burn))
    for day in days:
        n_spawns = sum(spawns[day].values())
        main = sum(tokens[day].values())
        agents = sum(fleet_burn[day].values())
        print(f"{day}  {n_spawns:3d} spawns   {main / 1e6:6.1f}M main-session + {agents / 1e6:6.1f}M fleet effective tokens")
        for agent, n in spawns[day].most_common():
            print(f"      spawn  {agent:28s} {n}")
        for model, n in tokens[day].most_common():
            print(f"      token  {model:28s} {n / 1e6:6.1f}M")
        for agent, n in fleet_burn[day].most_common():
            print(f"      fleet  {agent:28s} {n / 1e6:6.1f}M")
    spawn_total = sum(sum(c.values()) for c in spawns.values())
    main_total = sum(sum(c.values()) for c in tokens.values())
    fleet_total = sum(sum(c.values()) for c in fleet_burn.values())
    print(f"\nTOTAL over {len(days)} day(s): {spawn_total} spawns, "
          f"{main_total / 1e6:.1f}M main-session + {fleet_total / 1e6:.1f}M fleet effective tokens")
    print("Fleet tokens come from <session>/subagents/agent-*.jsonl; spawns whose transcript")
    print("was cleaned up are counted above but priced at zero.")


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    today = datetime.date.today()
    parser.add_argument("--since", default=str(today - datetime.timedelta(days=7)))
    parser.add_argument("--until", default=str(today))
    args = parser.parse_args()
    if not PROJECT.is_dir():
        print(f"spawn_cost: no transcript store at {PROJECT}")
        return 1
    spawns, tokens, _ = scan(args.since, args.until)
    report(spawns, tokens, fleet(args.since, args.until))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
