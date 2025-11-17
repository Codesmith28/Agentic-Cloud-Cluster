# File API Quick Reference

**Quick lookup guide for CloudAI File Retrieval API**

## API Endpoints Summary

| Method | Endpoint | Description | Auth Required |
|--------|----------|-------------|---------------|
| `GET` | `/api/files` | List all files for a user | ✅ Yes |
| `GET` | `/api/files/{task_id}` | Get task file details | ✅ Yes |
| `GET` | `/api/files/{task_id}/download/{file_path}` | Download specific file | ✅ Yes |
| `DELETE` | `/api/files/{task_id}` | Delete all task files | ✅ Yes |

---

## Quick Examples

### List User Files
```bash
curl "http://localhost:8080/api/files?requesting_user=alice&user_id=alice"
```

### Get Task Files
```bash
curl "http://localhost:8080/api/files/task-123?requesting_user=alice&user_id=alice"
```

### Download File
```bash
curl -O "http://localhost:8080/api/files/task-123/download/output.txt?requesting_user=alice&user_id=alice"
```

### Delete Task Files
```bash
curl -X DELETE "http://localhost:8080/api/files/task-123?requesting_user=alice&user_id=alice"
```

---

## Common Parameters

| Parameter | Required | Location | Description |
|-----------|----------|----------|-------------|
| `requesting_user` | ✅ Yes | Query | User making the request |
| `user_id` | ✅ Yes | Query | Owner of the files |
| `task_id` | ✅ Yes | Path | Task identifier |
| `file_path` | ✅ Yes | Path | Relative file path |

---

## Response Codes

| Code | Meaning | Common Cause |
|------|---------|--------------|
| `200` | Success | Request completed successfully |
| `400` | Bad Request | Missing parameters or invalid format |
| `403` | Forbidden | Access denied or path traversal |
| `404` | Not Found | Task or file doesn't exist |
| `503` | Service Unavailable | File storage not initialized |

---

## Access Control Rules

✅ **Allowed**:
- Users accessing their own files
- Admin accessing any files
- Valid relative paths

❌ **Blocked**:
- Users accessing other users' files
- Path traversal (`../`, absolute paths)
- Unauthenticated requests

---

## File Structure

```
~/.cloudai/files/
└── <user_id>/
    └── <task_name>/
        └── <timestamp>/
            └── <task_id>/
                └── files...
```

---

## Complete Workflow

```bash
# 1. Submit task
TASK_ID=$(curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"docker_image":"alpine","command":"echo hi > /output/file.txt","cpu_required":1,"memory_required":512,"user_id":"alice"}' \
  | jq -r '.task_id')

# 2. Wait for completion
sleep 10

# 3. List files
curl "http://localhost:8080/api/files/${TASK_ID}?requesting_user=alice&user_id=alice"

# 4. Download file
curl -O "http://localhost:8080/api/files/${TASK_ID}/download/file.txt?requesting_user=alice&user_id=alice"

# 5. View file
cat file.txt

# 6. Cleanup
curl -X DELETE "http://localhost:8080/api/files/${TASK_ID}?requesting_user=alice&user_id=alice"
```

---

## Error Examples

### Missing Parameter
```bash
$ curl "http://localhost:8080/api/files?user_id=alice"
Missing requesting_user parameter
```

### Access Denied
```bash
$ curl "http://localhost:8080/api/files?requesting_user=alice&user_id=bob"
access denied: user alice cannot access files owned by bob
```

### Path Traversal
```bash
$ curl "http://localhost:8080/api/files/task-123/download/../../../etc/passwd?requesting_user=alice&user_id=alice"
access denied: path traversal not allowed
```

---

## Testing Checklist

- [ ] List own files (should succeed)
- [ ] List other user's files (should fail with 403)
- [ ] Download file from own task (should succeed)
- [ ] Download file from other user's task (should fail)
- [ ] Path traversal attack (should fail with 403)
- [ ] Admin access to any files (should succeed)
- [ ] Delete own task files (should succeed)
- [ ] Delete other user's task files (should fail)

---

## Production Deployment

### Required Changes
1. Replace query parameter auth with JWT tokens
2. Add rate limiting middleware
3. Restrict CORS to specific domains
4. Enable HTTPS/TLS
5. Add request logging and monitoring

### Example with JWT
```bash
# Instead of query parameters
curl -H "Authorization: Bearer <jwt-token>" \
  "http://localhost:8080/api/files?user_id=alice"
```

---

## See Also
- [Full File API Documentation](./053_FILE_RETRIEVAL_API.md)
- [Access Control Implementation](./051_ACCESS_CONTROL_IMPLEMENTATION.md)
- [HTTP API Reference](../HTTP_API_REFERENCE.md)
