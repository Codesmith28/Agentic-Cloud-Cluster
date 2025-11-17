#!/usr/bin/env bash
set -euo pipefail

API=${API:-http://localhost:8080}

# Register helper using the CLI endpoint if available, otherwise use HTTP registration
# Using HTTP: POST /api/workers (if implemented)

workers=(
  "Shehzada:10.1.133.148:50052"
  "kiwi:10.1.174.169:50052"
  "NullPointer:10.1.186.172:50052"
  "Tessa:10.1.129.143:50052"
)

for w in "${workers[@]}"; do
  name="$(echo $w | cut -d: -f1)"
  host_port="$(echo $w | cut -d: -f2-)
"
  echo "Registering $name -> $host_port"

  # Try HTTP register endpoint
  resp=$(curl -s -X POST "$API/api/workers" \
    -H "Content-Type: application/json" \
    -d "{\"worker_id\": \"$name\", \"addr\": \"$host_port\"}" || true)

  if [ -n "$resp" ]; then
    echo "HTTP register response: $resp"
  else
    echo "No HTTP register endpoint detected, try CLI registration"
    echo "register $name $host_port" | nc localhost 7000 || true
  fi
  sleep 0.3
done

echo "Done registering workers (note: CLI or HTTP must be available)"
