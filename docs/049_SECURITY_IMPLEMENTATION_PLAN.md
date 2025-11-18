# CloudAI Security Implementation Plan

**Status**: 🔴 Critical - Security Gaps Identified  
**Priority**: HIGH  
**Date**: 2025-11-17

## Current Security Issues

### 1. Physical Access to Machines ⚠️
**Issue**: If an attacker gains shell access to worker/master, they can read all files.

**Current State**:
```bash
# On worker machine - anyone with shell access can:
ls /var/cloudai/outputs/task-*/
cat /var/cloudai/outputs/task-1234/sensitive-data.txt

# On master machine - anyone with shell access can:
ls /var/cloudai/files/admin/
cat /var/cloudai/files/admin/project-x/1234567890/task-1234/confidential.pdf
```

**Risk Level**: 🔴 HIGH
- Competitor gains access to proprietary ML models
- Data breach of sensitive user data
- Intellectual property theft

### 2. No Worker Authentication ⚠️
**Issue**: Any machine can register as a worker and receive tasks.

**Current State**:
```go
// ANY machine can register by calling:
masterCLI> register malicious-worker 192.168.1.100:50052
```

**Risk Level**: 🔴 HIGH
- Malicious worker receives sensitive tasks
- Data exfiltration via fake worker
- Denial of service by registering but not executing

### 3. Unencrypted File Transfer ⚠️
**Issue**: Files transferred over gRPC without TLS encryption.

**Current State**:
```go
conn, err := grpc.Dial(s.masterAddr, grpc.WithInsecure()) // ⚠️ No TLS!
```

**Risk Level**: 🟡 MEDIUM
- Network sniffing can intercept files
- Man-in-the-middle attacks possible
- Especially risky on untrusted networks

### 4. No User Isolation on Disk ⚠️
**Issue**: All users' files stored under same base directory with world-readable permissions.

**Current State**:
```bash
/var/cloudai/files/
├── user1/         # 0755 - world readable!
│   └── secret-project/
│       └── ...
├── user2/         # 0755 - world readable!
│   └── confidential/
│       └── ...
```

**Risk Level**: 🟡 MEDIUM
- Users on same machine can read each other's files
- Shared hosting environments are vulnerable

### 5. No Authorization Checks ⚠️
**Issue**: No verification that user requesting files owns those files.

**Current State**:
```javascript
// Future API - currently no auth:
GET /api/files?user_id=admin  // Anyone can query any user!
```

**Risk Level**: 🔴 HIGH (when API implemented)

## Security Solution Architecture

### Phase 1: Access Control & Encryption (Priority 1)

#### 1.1 Worker Authentication with API Keys

**Implementation**:

```go
// master/internal/config/config.go
type Config struct {
    // ... existing fields ...
    WorkerAPIKeys map[string]string // worker_id -> api_key
    RequireWorkerAuth bool
}

// Worker registration with API key
func (s *MasterServer) RegisterWorker(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
    // Verify API key
    expectedKey := s.config.WorkerAPIKeys[req.WorkerId]
    if expectedKey == "" || req.ApiKey != expectedKey {
        return &pb.RegisterResponse{
            Success: false,
            Message: "Invalid worker API key",
        }, nil
    }
    
    // ... rest of registration ...
}
```

**Configuration**:
```yaml
# config/worker_keys.yaml
worker_keys:
  worker-1: "sk-worker1-abc123xyz789"
  worker-2: "sk-worker2-def456uvw012"
  Tessa: "sk-tessa-ghi789rst345"

require_auth: true
```

**Benefits**:
- ✅ Only authorized workers can register
- ✅ Revocable keys (remove from config to ban worker)
- ✅ Audit trail (log which key was used)

#### 1.2 TLS Encryption for gRPC

**Implementation**:

```go
// master/main.go - Enable TLS
creds, err := credentials.NewServerTLSFromFile(
    "certs/master.crt",
    "certs/master.key",
)
if err != nil {
    log.Fatalf("Failed to load TLS credentials: %v", err)
}

grpcServer := grpc.NewServer(grpc.Creds(creds))

// worker/internal/server/worker_server.go - Use TLS for master connection
creds, err := credentials.NewClientTLSFromFile("certs/master.crt", "master.cloudai.local")
conn, err := grpc.Dial(masterAddr, grpc.WithTransportCredentials(creds))
```

**Certificate Generation**:
```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -key ca.key -out ca.crt -days 3650

# Generate master certificate
openssl genrsa -out master.key 4096
openssl req -new -key master.key -out master.csr
openssl x509 -req -in master.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out master.crt -days 365
```

**Benefits**:
- ✅ End-to-end encryption for file transfers
- ✅ Prevents network sniffing
- ✅ Mutual TLS possible for stronger auth

#### 1.3 File Encryption at Rest

**Implementation**:

```go
// master/internal/storage/encryption.go
package storage

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "io"
)

type EncryptedStorage struct {
    baseStorage *FileStorageService
    key         []byte // AES-256 key (32 bytes)
}

func (s *EncryptedStorage) StoreFile(data []byte, path string) error {
    // Encrypt file before storing
    encrypted, err := s.encrypt(data)
    if err != nil {
        return err
    }
    
    return s.baseStorage.WriteFile(path, encrypted)
}

func (s *EncryptedStorage) encrypt(plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *EncryptedStorage) decrypt(ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonceSize := gcm.NonceSize()
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

**Key Management**:
```go
// Store encryption keys per user in database
type EncryptionKey struct {
    UserID    string `bson:"user_id"`
    Key       []byte `bson:"key"` // Encrypted with master key
    CreatedAt time.Time `bson:"created_at"`
}
```

**Benefits**:
- ✅ Files unreadable if disk is stolen
- ✅ Per-user encryption keys
- ✅ Compliance with data protection regulations

### Phase 2: User Isolation & Access Control (Priority 2)

#### 2.1 Strict File Permissions

**Implementation**:

```go
// master/internal/storage/file_storage.go
func (s *FileStorageService) GetTaskStoragePath(userID, taskName string, submittedAt int64, taskID string) string {
    path := filepath.Join(s.baseDir, userID, taskName, fmt.Sprintf("%d", submittedAt), taskID)
    
    // Create directory with user-only permissions
    os.MkdirAll(path, 0700) // Only owner can read/write/execute
    
    return path
}

func (s *FileStorageService) WriteFile(path string, data []byte) error {
    // Write file with user-only read permissions
    return os.WriteFile(path, data, 0600) // Only owner can read/write
}
```

**Directory Structure with Permissions**:
```bash
/var/cloudai/files/           # 0755 (drwxr-xr-x)
├── user1/                    # 0700 (drwx------) ✅ Only user1 can access
│   └── secret-project/       # 0700
│       └── 1234567890/       # 0700
│           └── task-1234/    # 0700
│               └── data.txt  # 0600 (-rw-------) ✅ Only owner can read
└── user2/                    # 0700 (drwx------) ✅ Only user2 can access
    └── project/              # 0700
```

**Benefits**:
- ✅ User files isolated at OS level
- ✅ Even root needs sudo to access other users' files
- ✅ Prevents accidental data leaks

#### 2.2 Database Access Control

**Implementation**:

```go
// master/internal/db/file_metadata.go
func (db *FileMetadataDB) GetFileMetadataByUser(ctx context.Context, userID, requestingUserID string) ([]*FileMetadata, error) {
    // Authorization check
    if userID != requestingUserID {
        // Check if requesting user is admin
        if !db.IsAdmin(requestingUserID) {
            return nil, fmt.Errorf("access denied: cannot access other users' files")
        }
    }
    
    cursor, err := db.collection.Find(ctx, bson.M{"user_id": userID})
    // ... rest of implementation ...
}
```

**Role-Based Access Control (RBAC)**:
```go
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleUser   Role = "user"
    RoleWorker Role = "worker"
)

type User struct {
    UserID   string   `bson:"user_id"`
    Roles    []Role   `bson:"roles"`
    APIKey   string   `bson:"api_key"`
}
```

**Benefits**:
- ✅ Fine-grained access control
- ✅ Admin can access all files (for support)
- ✅ Users isolated from each other

#### 2.3 API Authentication & Authorization

**Implementation**:

```go
// master/internal/http/middleware.go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get API key from header
        apiKey := c.GetHeader("X-API-Key")
        if apiKey == "" {
            c.JSON(401, gin.H{"error": "Missing API key"})
            c.Abort()
            return
        }
        
        // Validate API key and get user
        user, err := validateAPIKey(apiKey)
        if err != nil {
            c.JSON(401, gin.H{"error": "Invalid API key"})
            c.Abort()
            return
        }
        
        // Store user in context
        c.Set("user", user)
        c.Next()
    }
}

// master/internal/http/handlers.go
func ListUserFiles(c *gin.Context) {
    requestedUserID := c.Query("user_id")
    currentUser := c.MustGet("user").(*User)
    
    // Authorization check
    if requestedUserID != currentUser.UserID && !currentUser.HasRole(RoleAdmin) {
        c.JSON(403, gin.H{"error": "Access denied"})
        return
    }
    
    // ... retrieve files ...
}
```

**Benefits**:
- ✅ Every API request authenticated
- ✅ Users can only access their own files
- ✅ Admins have elevated privileges

### Phase 3: Worker Isolation (Priority 3)

#### 3.1 Worker-Specific Output Directories

**Current Issue**: Workers use `/var/cloudai/outputs/` - shared if workers run on same machine.

**Solution**: Use worker-specific directories:

```go
// worker/main.go
outputBaseDir := fmt.Sprintf("/var/cloudai/outputs/%s", workerID)
if err := os.MkdirAll(outputBaseDir, 0700); err != nil { // Only worker process can access
    log.Fatalf("Failed to create output directory: %v", err)
}
```

**Structure**:
```bash
/var/cloudai/outputs/
├── worker-1/        # 0700 (only worker-1 process)
│   └── task-1234/
└── Tessa/           # 0700 (only Tessa process)
    └── task-5678/
```

#### 3.2 Container Isolation

**Docker Security Best Practices**:

```go
// worker/internal/executor/executor.go
func (e *TaskExecutor) createContainer(...) (string, error) {
    hostConfig := &container.HostConfig{
        // Security options
        SecurityOpt: []string{
            "no-new-privileges", // Prevent privilege escalation
        },
        
        // Read-only root filesystem
        ReadonlyRootfs: true,
        
        // Only /output is writable
        Mounts: []mount.Mount{
            {
                Type:   mount.TypeBind,
                Source: outputDir,
                Target: "/output",
            },
            {
                Type:   mount.TypeTmpfs,
                Target: "/tmp", // Temporary files
            },
        },
        
        // Drop all capabilities
        CapDrop: []string{"ALL"},
        
        // No network access (optional - depends on use case)
        NetworkMode: "none",
    }
    
    // ... rest of container creation ...
}
```

**Benefits**:
- ✅ Container cannot access other tasks' files
- ✅ Limited attack surface
- ✅ Even if container is compromised, damage is contained

### Phase 4: Audit Logging (Priority 4)

#### 4.1 Security Event Logging

**Implementation**:

```go
// master/internal/audit/audit.go
type AuditLog struct {
    Timestamp  time.Time `bson:"timestamp"`
    EventType  string    `bson:"event_type"` // "file_access", "worker_register", "task_submit"
    UserID     string    `bson:"user_id"`
    WorkerID   string    `bson:"worker_id,omitempty"`
    TaskID     string    `bson:"task_id,omitempty"`
    Action     string    `bson:"action"` // "read", "write", "delete"
    Resource   string    `bson:"resource"` // File path, task ID, etc.
    Success    bool      `bson:"success"`
    IPAddress  string    `bson:"ip_address"`
    UserAgent  string    `bson:"user_agent,omitempty"`
}

func LogFileAccess(userID, taskID, filePath string, success bool) {
    log := AuditLog{
        Timestamp:  time.Now(),
        EventType:  "file_access",
        UserID:     userID,
        TaskID:     taskID,
        Action:     "read",
        Resource:   filePath,
        Success:    success,
    }
    auditDB.Insert(context.Background(), log)
}
```

**Benefits**:
- ✅ Track all file access attempts
- ✅ Detect suspicious activity
- ✅ Compliance with security regulations

## Implementation Priority

### Immediate (This Sprint)
1. ✅ **Automatic Directory Creation** - Already implemented
2. 🔴 **Strict File Permissions** (0700 for user dirs, 0600 for files)
3. 🔴 **Worker API Key Authentication**

### Short Term (Next Sprint)
4. 🟡 **TLS Encryption for gRPC**
5. 🟡 **API Authentication & Authorization**
6. 🟡 **Database Access Control**

### Medium Term (Future Sprints)
7. 🟢 **File Encryption at Rest**
8. 🟢 **Worker-Specific Directories**
9. 🟢 **Enhanced Container Security**

### Long Term (Future Releases)
10. 🔵 **Audit Logging**
11. 🔵 **Intrusion Detection**
12. 🔵 **Security Scanning**

## Quick Wins (Implement Now)

### 1. Strict File Permissions

```go
// master/internal/storage/file_storage.go
func (s *FileStorageService) GetTaskStoragePath(userID, taskName string, submittedAt int64, taskID string) string {
    userDir := filepath.Join(s.baseDir, userID)
    
    // Create user directory with strict permissions
    os.MkdirAll(userDir, 0700) // drwx------ (owner only)
    
    path := filepath.Join(userDir, taskName, fmt.Sprintf("%d", submittedAt), taskID)
    os.MkdirAll(path, 0700)
    
    return path
}
```

### 2. Worker API Keys

```go
// proto/master_worker.proto
message RegisterRequest {
    string worker_id = 1;
    string worker_address = 2;
    string api_key = 3; // NEW: Worker API key
    // ... other fields ...
}
```

### 3. Input Validation

```go
// Prevent path traversal attacks
func sanitizeFilePath(path string) (string, error) {
    // Reject paths with ".."
    if strings.Contains(path, "..") {
        return "", fmt.Errorf("invalid path: contains ..")
    }
    
    // Reject absolute paths
    if filepath.IsAbs(path) {
        return "", fmt.Errorf("invalid path: must be relative")
    }
    
    return filepath.Clean(path), nil
}
```

## Testing Security

### Penetration Testing Scenarios

1. **Unauthorized File Access**:
   ```bash
   # Try to access another user's files
   curl -H "X-API-Key: user1-key" \
        http://master:8080/api/files?user_id=user2
   # Expected: 403 Forbidden
   ```

2. **Path Traversal**:
   ```bash
   # Try to escape output directory
   echo "malicious" > /output/../../../etc/passwd
   # Expected: Permission denied
   ```

3. **Fake Worker Registration**:
   ```bash
   # Try to register without valid API key
   grpcurl -d '{"worker_id":"evil","api_key":"wrong"}' \
           master:50051 MasterWorker/RegisterWorker
   # Expected: Registration rejected
   ```

## Compliance Considerations

- **GDPR**: User data encrypted at rest and in transit
- **HIPAA**: Audit logs for all file access
- **SOC 2**: Access controls and monitoring
- **PCI DSS**: Encryption and secure key management

## Summary

**Current State**: 🔴 Multiple security vulnerabilities
**Target State**: 🟢 Enterprise-grade security with:
- ✅ Worker authentication
- ✅ TLS encryption
- ✅ File encryption at rest
- ✅ User isolation
- ✅ Access control
- ✅ Audit logging

**Immediate Actions Required**:
1. Implement strict file permissions (0700/0600)
2. Add worker API key authentication
3. Plan TLS implementation for next sprint
