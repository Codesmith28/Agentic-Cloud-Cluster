#!/usr/bin/env bash
set -euo pipefail
GA_FILE=${GA_FILE:-master/config/ga_output.json}

if [ ! -f "$GA_FILE" ]; then
  echo "GA output file not found: $GA_FILE" >&2
  exit 1
fi

jq . "$GA_FILE"

echo "If AffinityMatrix has rows (for 6 types), GA has training data."
