#!/usr/bin/env bash
set -euo pipefail
MONGO=${MONGO:-mongodb://localhost:27017}
DB=${DB:-cloudai}

# Compute SLA success rate by task type
mongo --quiet "$MONGO/$DB" --eval '
  db.results.aggregate([
    { $match: { status: "completed" } },
    { $group: { _id: "$task_type", total: { $sum: 1 }, sla_success: { $sum: { $cond: ["$sla_success", 1, 0] } } } },
    { $project: { task_type: "$_id", total: 1, sla_success: 1, sla_rate: { $cond: [ { $eq: ["$total", 0] }, 0, { $divide: ["$sla_success", "$total"] } ] } } }
  ]).toArray()' | jq .
