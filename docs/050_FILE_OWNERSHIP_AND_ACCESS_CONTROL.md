# File Ownership and Access Control in CloudAI

**Date**: 2025-11-17  
**Status**: 📋 Implementation Guide

## How File Ownership Currently Works

### Operating System Level

Files are owned by **whoever runs the master/worker process**:

```bash
# Run as your user
$ whoami
codesmith28

$ ./master/masterNode
# All files created with owner: codesmith28, group: codesmith28

$ ls -la /var/cloudai/files/admin/
drwx------ 3 codesmith28 codesmith28 4096 Nov 17 14:05 .
```

**Key Point**: The OS doesn't know about CloudAI's "users" (admin, user1, user2). It only knows about OS users (codesmith28, root, etc.).

### Current Security Model

**File Permissions: 0700 (drwx------)**
- Owner: Can read, write, execute
- Group: No access
- Others: No access

**This protects against:**
- ✅ Other OS users on the same machine
- ✅ Unauthorized shell access
- ✅ Accidental file exposure

**This DOES NOT protect against:**
- ❌ Process compromise (if attacker gets control of master process, they access ALL files)
- ❌ CloudAI user isolation (all CloudAI users' files have same OS owner)
- ❌ Insider threats (anyone with access to master process)

## The Multi-Tenant Problem

### Scenario

Two CloudAI users: `alice` and `bob`

Both submit tasks, both get files stored:

```
/var/cloudai/files/
├── alice/
│   └── secret-project/
│       └── task-1/
│           └── confidential.pdf    # owner: codesmith28, mode: 0600
└── bob/
    └── public-project/
        └── task-2/
            └── data.csv            # owner: codesmith28, mode: 0600
```

**Problem**: Both files have **same OS owner** (codesmith28)!

If Alice somehow gets shell access to the master machine as `codesmith28`:
```bash
# Alice can read Bob's files!
cat /var/cloudai/files/bob/public-project/task-2/data.csv
```

## Solutions

### Solution 1: Application-Level Access Control (✅ Recommended)

**Concept**: Enforce access control in **application code**, not just file permissions.

#### Implementation

Create an `AccessControl` layer that wraps all file operations:

```go
// Instead of direct file access:
data, _ := os.ReadFile("/var/cloudai/files/bob/data.csv")  // ❌ No access check!

// Use access-controlled reads:
ac := NewAccessControl(fileStorage)
data, err := ac.ReadFile(requestingUser: "alice", targetUser: "bob", file: "data.csv")
// ✅ Returns error: "access denied: alice cannot access bob's files"
```

#### Benefits
- ✅ No root privileges needed
- ✅ Single master process
- ✅ Flexible access rules (roles, sharing, etc.)
- ✅ Works with existing infrastructure
- ✅ Can add advanced features (file sharing, temporary access, etc.)

#### Limitations
- ⚠️ Only protects at application level
- ⚠️ If process is compromised, attacker bypasses checks
- ⚠️ Requires careful implementation in all file access paths

#### Code Structure

```go
// master/internal/storage/access_control.go
type AccessControl struct {
    storage *FileStorageService
}

// Check access before any file operation
func (ac *AccessControl) CanAccessFile(requestingUserID, targetUserID string) error {
    if requestingUserID == targetUserID {
        return nil  // Own files
    }
    if ac.isAdmin(requestingUserID) {
        return nil  // Admin access
    }
    return fmt.Errorf("access denied")
}

// All file operations go through access control
func (ac *AccessControl) ReadFile(requestingUserID, targetUserID, path string) ([]byte, error) {
    // 1. Check access
    if err := ac.CanAccessFile(requestingUserID, targetUserID); err != nil {
        ac.AuditFileAccess(requestingUserID, "read", path, false)
        return nil, err
    }
    
    // 2. Read file
    data, err := os.ReadFile(fullPath)
    
    // 3. Audit
    ac.AuditFileAccess(requestingUserID, "read", path, err == nil)
    
    return data, err
}
```

### Solution 2: Per-User OS Accounts (⚠️ Complex but Strongest)

**Concept**: Map each CloudAI user to a dedicated OS user.

#### Setup

```bash
# Create OS user for each CloudAI user
sudo useradd -r -s /bin/false cloudai-alice
sudo useradd -r -s /bin/false cloudai-bob

# Run master as root, but use setuid to switch per operation
# (Advanced - requires C bindings or syscalls)
```

#### Directory Structure

```bash
/var/cloudai/files/
├── alice/
│   └── secret/
│       └── file.txt      # owner: cloudai-alice (uid: 1001)
└── bob/
    └── data/
        └── file.txt      # owner: cloudai-bob (uid: 1002)
```

#### Benefits
- ✅ True OS-level isolation
- ✅ Even if master is compromised, attacker can't access other users' files
- ✅ Kernel enforces access control
- ✅ Audit trail in OS logs

#### Limitations
- ❌ Requires root privileges
- ❌ Complex to implement (need setuid/setgid calls)
- ❌ Doesn't scale well (1000 users = 1000 OS users)
- ❌ Platform-specific (Linux syscalls)

### Solution 3: Encryption (🔐 Defense in Depth)

**Concept**: Encrypt each user's files with their own key.

#### Implementation

```go
// Encrypt files before writing
func (s *EncryptedStorage) WriteFile(userID string, data []byte) error {
    key := s.getUserEncryptionKey(userID)
    encrypted := aes256Encrypt(data, key)
    return os.WriteFile(path, encrypted, 0600)
}

// Decrypt when reading
func (s *EncryptedStorage) ReadFile(userID string) ([]byte, error) {
    encrypted, _ := os.ReadFile(path)
    key := s.getUserEncryptionKey(userID)
    return aes256Decrypt(encrypted, key)
}
```

#### Key Management

```go
type UserEncryptionKey struct {
    UserID       string    `bson:"user_id"`
    EncryptedKey []byte    `bson:"encrypted_key"`  // Encrypted with master key
    CreatedAt    time.Time `bson:"created_at"`
}

// Master key stored in secure location (HSM, Vault, env var)
masterKey := os.Getenv("CLOUDAI_MASTER_KEY")
```

#### Benefits
- ✅ Files unreadable without key
- ✅ Protects against disk theft
- ✅ Protects against backup compromise
- ✅ Compliance with data protection laws

#### Limitations
- ⚠️ Performance overhead (encrypt/decrypt)
- ⚠️ Key management complexity
- ⚠️ If master process is compromised, keys are accessible

## Recommended Implementation Strategy

### Phase 1: Application-Level Access Control (Now)

**Files created**: 
- `master/internal/storage/access_control.go` ✅ (created)
- `master/internal/storage/file_storage_secure.go` ✅ (created)

**Changes needed**:
1. Wrap all file operations with `AccessControl`
2. Add user authentication to HTTP API
3. Implement role-based access (admin, user, worker)
4. Add audit logging

### Phase 2: Encryption at Rest (Next Sprint)

1. Implement AES-256-GCM encryption
2. Per-user encryption keys
3. Master key rotation
4. Key derivation from user passwords

### Phase 3: Advanced Features (Future)

1. File sharing between users
2. Temporary access links
3. File expiration
4. Compression

## How to Use (Once Implemented)

### Current (Direct File Access)
```go
// In HTTP handler
files := fileStorage.GetUserFiles(userID)  // ❌ No access control
```

### Secure (With Access Control)
```go
// In HTTP handler
func GetFilesHandler(c *gin.Context) {
    requestingUser := c.Get("user").(*User)  // From auth middleware
    targetUser := c.Query("user_id")
    
    ac := NewAccessControl(fileStorage)
    files, err := ac.GetUserFiles(requestingUser.ID, targetUser)
    if err != nil {
        c.JSON(403, gin.H{"error": "Access denied"})
        return
    }
    
    c.JSON(200, files)
}
```

## Security Best Practices

### 1. Defense in Depth
Use **multiple layers**:
- File permissions (0700/0600) ✅ Implemented
- Application access control ✅ Code provided
- Encryption at rest 🔜 Next phase
- TLS in transit 🔜 Next phase
- Audit logging 🔜 Next phase

### 2. Principle of Least Privilege
- Don't run as root (unless necessary)
- Drop privileges after startup
- Use specific capabilities instead of full root

### 3. Regular Audits
- Log all file access
- Monitor for suspicious patterns
- Alert on unauthorized access attempts

### 4. Secure Key Management
- Never hardcode keys
- Use environment variables or secrets management
- Rotate keys regularly
- Use HSM for production

## Current Status

### ✅ Implemented
- Automatic directory creation
- Secure file permissions (0700/0600)
- Directory structure ready

### 🔨 Ready to Implement
- Application-level access control (code provided)
- User authentication
- Audit logging

### 🔜 Planned
- Encryption at rest
- TLS for gRPC
- Worker authentication

## Example Attack Scenarios

### Scenario 1: Compromised Worker
**Attack**: Worker is compromised, attacker tries to access other tasks' files

**Defense**:
- ✅ Worker files in isolated directory (`/var/cloudai/outputs/<worker-id>/`)
- ✅ Container isolation prevents access to host files
- ✅ Files transferred to master immediately, then deleted

### Scenario 2: Compromised Master Process
**Attack**: Attacker gets control of master process, tries to read all users' files

**Defense**:
- ⚠️ Current: Application access control (can be bypassed if process compromised)
- 🔜 Future: Encryption at rest (files unreadable without keys)
- 🔜 Future: Per-user OS accounts (kernel enforces access)

### Scenario 3: Shell Access to Master Machine
**Attack**: Attacker gets shell access as `codesmith28` user

**Defense**:
- ✅ File permissions 0700/0600 prevent access (only owner)
- ✅ Would need to be same OS user (codesmith28) to read files
- ⚠️ If attacker IS codesmith28, can read files
- 🔜 Future: Encryption makes files unreadable

### Scenario 4: Disk Theft
**Attack**: Physical server is stolen, attacker mounts disk

**Defense**:
- ❌ Current: Files readable from disk
- 🔜 Future: Encryption at rest makes files unreadable
- 🔜 Future: Full disk encryption (LUKS/BitLocker)

## Summary

**Who owns the files?**
- Currently: The OS user that runs master/worker (e.g., `codesmith28`)
- Files have permissions: 0700 (dirs), 0600 (files)
- Only that OS user can access the files

**How to secure multi-tenant setup?**
1. ✅ **Now**: Application-level access control
2. 🔜 **Next**: Encryption at rest
3. 🔜 **Future**: Per-user OS accounts (advanced)

**Which solution is best?**
- **Small deployment**: Application-level access control (sufficient)
- **Enterprise**: Application + Encryption + TLS
- **High security**: All of the above + Per-user OS accounts

**Next steps:**
1. Review the access control code provided
2. Integrate with HTTP API (when implementing file retrieval)
3. Add user authentication
4. Implement audit logging
