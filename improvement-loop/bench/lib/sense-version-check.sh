#!/usr/bin/env bash
# sense-version-check.sh - the installed binary must BE the latest release.
#
#   bash bench/lib/sense-version-check.sh          # exit 0 = current
#
# Every index the pipeline builds is only as good as the binary that built it,
# and an index built by an older binary is indistinguishable from a fresh one
# once it is on disk. So this is a precondition, not a warning.
#
# The comparison is deliberately the simplest one that cannot lie: the latest
# GitHub RELEASE tag against `sense --version`. Source timestamps were tried and
# rejected - mtime answers "which files were touched", not "which files changed",
# so a branch switch alone makes a current binary look stale.
#
# Needs `gh`. Offline, it reports UNKNOWN and exits 0: this gate exists to catch
# a stale binary, not to make the pipeline require a network.

set -uo pipefail
REPO="${SENSE_GH_REPO:-luuuc/sense}"
SENSE_BIN="${SENSE_BIN:-$(command -v sense || echo "$HOME/.local/bin/sense")}"

if [ ! -x "$SENSE_BIN" ]; then
  echo "[sense-version] NOT INSTALLED: no sense binary at $SENSE_BIN"
  echo "                build it:  go build -o $SENSE_BIN ./cmd/sense"
  exit 1
fi

installed="$("$SENSE_BIN" --version 2>/dev/null | awk '{print $2}')"
if [ -z "$installed" ]; then
  echo "[sense-version] cannot read a version from $SENSE_BIN --version"
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "[sense-version] UNKNOWN: gh not installed, cannot read the latest release (installed $installed)"
  exit 0
fi

# gh prints the error BODY to stdout on a 404, so a failed call must be caught
# by its exit code: trusting stdout reports the error JSON as the "latest
# version" and calls a current binary stale.
latest="$(gh api "repos/$REPO/releases/latest" --jq .tag_name 2>/dev/null)"
if [ $? -ne 0 ] || [ -z "$latest" ] || case "$latest" in *"{"*|*" "*) true ;; *) false ;; esac; then
  echo "[sense-version] UNKNOWN: no readable release tag from $REPO (installed $installed)"
  exit 0
fi
latest="${latest#v}"

if [ "$installed" = "$latest" ]; then
  echo "[sense-version] OK - $installed is the latest release"
  exit 0
fi

echo "[sense-version] STALE: installed $installed, latest release is $latest"
echo "                every index built now would carry the old scan engine."
echo "                update:  rm $SENSE_BIN && go build -o $SENSE_BIN ./cmd/sense"
echo "                (go build -o, never cp: copying breaks macOS code signing)"
exit 1
