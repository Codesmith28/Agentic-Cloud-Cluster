#!/usr/bin/env bash
set -euo pipefail
MONGO=${MONGO:-mongodb://localhost:27017}
DB=${DB:-cloudai}

# Uses mongo shell to run aggregation to see assignments per worker
mongo --quiet "$MONGO/$DB" --eval '
  db.results.aggregate([
    { $match: { status: "completed" } },
    { $group: { _id: "$worker_id", count: { $sum: 1 } } },
    { $sort: { count: -1 } }
  ]).toArray()' | jq .
