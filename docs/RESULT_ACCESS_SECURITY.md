# Result Access Security (Hard Cutover)

## Scope
This document defines backend authorization for task-generated results:
- task details that include result data,
- task logs,
- generated output files.

## Security Boundary
- `user` role:
  - Can submit tasks.
  - Can read only results for tasks they own.
  - Can operate (cancel/delete) only own tasks/files.
- `admin` role:
  - Can manage control-plane operations globally (workers and task operations).
  - Cannot read other users' result data by default.
  - Can read other users' result data only with break-glass.

## Owner Identity
- Owner identity is always derived from authenticated JWT email.
- Client-supplied identity controls are rejected:
  - Task submit payload field `user_id` is invalid.
  - File API query params `user_id` and `requesting_user` are invalid.

## Break-Glass Policy
- Header: `X-Breakglass-Reason`
- Required when admin reads non-owned result data.
- Every cross-user break-glass read attempt is audit logged with:
  - actor,
  - role,
  - target owner,
  - resource,
  - reason,
  - allow/deny result.

## Result Endpoints
- Owner-scoped endpoints:
  - `GET /api/tasks`
  - `GET /api/tasks/{task_id}`
  - `GET /api/tasks/{task_id}/logs`
  - `WS /ws/tasks/{task_id}/logs`
  - `GET /api/files`
  - `GET /api/files/{task_id}`
  - `GET /api/files/{task_id}/download/{file_path}`

- Admin break-glass endpoints:
  - `GET /api/admin/tasks/{task_id}`
  - `GET /api/admin/tasks/{task_id}/logs`
  - `WS /ws/admin/tasks/{task_id}/logs`
  - `GET /api/admin/files/{task_id}`
  - `GET /api/admin/files/{task_id}/download/{file_path}`

## Upload Ingress Hardening
- Worker-provided file upload ownership metadata is not trusted.
- Master resolves trusted task owner from `TASKS` by `task_id`.
- Upload stream must contain a single `task_id`.
- Relative upload paths are validated and traversal attempts are rejected.

## Out of Scope (Deferred)
- Worker↔Master transport hardening (mTLS and worker authentication) is deferred to the next phase.
