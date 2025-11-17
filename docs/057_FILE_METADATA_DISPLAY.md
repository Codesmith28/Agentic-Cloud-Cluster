# File Metadata Display Enhancement

**Date**: November 17, 2025  
**Status**: ✅ Complete  
**Related Docs**: [043_FILE_TRANSFER_SYSTEM.md](043_FILE_TRANSFER_SYSTEM.md), [049_FILE_API.md](049_FILE_API.md)

## Overview

Enhanced file display system to show useful metadata (file sizes, total size) instead of revealing internal storage paths that could be exploited by users.

## Security Issue Addressed

### Previous Behavior
The `task-files` command displayed the internal storage path:
```
║  Storage:    /home/codesmith28/.cloudai/files/admin/cloudai-cpu-heavy-1763370124/2025-11-17_14-32-04/task-1763370124
```

**Problem**: This reveals:
- Internal directory structure
- Exact file system paths
- Storage organization scheme
- Potential exploitation points for path traversal attacks

### New Behavior
Shows useful file metadata instead:
```
║  Total Size: 245.67 KB
║  Files:      3
╠═══════════════════════════════════════════════════════
║  [1] output.txt (12.34 KB)
║  [2] results.json (156.89 KB)
║  [3] logs/debug.log (76.44 KB)
```

**Benefits**:
- No internal path exposure
- Human-readable file sizes
- Individual file size information
- Total size at task level

## Implementation Details

### 1. Updated Data Structures

#### FileInfo Structure
```go
// FileInfo represents individual file information
type FileInfo struct {
    Path string // Relative path from task directory
    Size int64  // File size in bytes
}
```

#### Enhanced FileMetadata
```go
type FileMetadata struct {
    UserID      string
    TaskID      string
    TaskName    string
    Timestamp   time.Time
    FilePaths   []string   // Deprecated, kept for backward compatibility
    Files       []FileInfo // New: Detailed file information with sizes
    StoragePath string     // Internal use only, not exposed to users
    TotalSize   int64      // New: Total size of all files in bytes
}
```

### 2. File Size Collection

#### ListUserFiles() Enhancement
```go
// Collect file information during directory walk
var files []FileInfo
var totalSize int64

filepath.Walk(taskDir, func(filePath string, fileInfo os.FileInfo, err error) error {
    if err != nil || fileInfo.IsDir() {
        return nil
    }
    relPath, _ := filepath.Rel(taskDir, filePath)
    
    files = append(files, FileInfo{
        Path: relPath,
        Size: fileInfo.Size(),
    })
    totalSize += fileInfo.Size()
    return nil
})

// Store in metadata
metadata := FileMetadata{
    // ... other fields ...
    Files:     files,
    TotalSize: totalSize,
}
```

### 3. Human-Readable Size Formatting

#### formatFileSize() Utility
```go
func formatFileSize(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
```

**Examples**:
- `1023 B` → "1023 B"
- `1024 B` → "1.00 KB"
- `1536 B` → "1.50 KB"
- `1048576 B` → "1.00 MB"
- `1234567890 B` → "1.15 GB"

### 4. CLI Display Updates

#### task-files Command
```go
fmt.Printf("║  Total Size: %s\n", formatFileSize(metadata.TotalSize))
fmt.Printf("║  Files:      %d\n", len(metadata.Files))

for i, file := range metadata.Files {
    fmt.Printf("║  [%d] %s (%s)\n", i+1, file.Path, formatFileSize(file.Size))
}
```

#### download Command
```go
fmt.Printf("║  Total Size:   %s\n", formatFileSize(totalSize))
```

### 5. HTTP API Updates

#### Updated JSON Structures
```go
type FileInfoJSON struct {
    Path string `json:"path"`
    Size int64  `json:"size"`
}

type TaskFileInfo struct {
    TaskID    string         `json:"task_id"`
    TaskName  string         `json:"task_name"`
    Timestamp string         `json:"timestamp"`
    Files     []FileInfoJSON `json:"files"`
    TotalSize int64          `json:"total_size"`
    // StoragePath removed - no longer exposed
}
```

#### API Response Example
```json
{
    "user_id": "admin",
    "tasks": [
        {
            "task_id": "task-1763370124",
            "task_name": "cloudai-cpu-heavy-1763370124",
            "timestamp": "2025-11-17 14:32:04",
            "files": [
                {
                    "path": "sort_benchmark.csv",
                    "size": 251567
                },
                {
                    "path": "logs/execution.log",
                    "size": 78234
                }
            ],
            "total_size": 329801
        }
    ],
    "count": 1
}
```

## Changes Summary

### Files Modified

#### 1. master/internal/storage/file_storage.go
- Added `FileInfo` struct with Path and Size
- Enhanced `FileMetadata` with `Files []FileInfo` and `TotalSize int64`
- Updated `ListUserFiles()` to collect file sizes
- Maintained `FilePaths []string` for backward compatibility

#### 2. master/internal/cli/cli.go
- Added `formatFileSize()` utility function
- Updated `showTaskFiles()` to display sizes instead of storage path
- Updated `downloadTaskFiles()` to use formatted sizes
- Changed display from `metadata.FilePaths` to `metadata.Files`

#### 3. master/internal/http/file_handler.go
- Added `FileInfoJSON` struct for API responses
- Updated `TaskFileInfo` to use `[]FileInfoJSON` and `TotalSize`
- Updated `FileDetailResponse` similarly
- Removed `StoragePath` from JSON responses
- Added conversion logic in handlers

## Security Improvements

### Path Information Hiding
- ✅ Internal storage paths no longer exposed
- ✅ Directory structure kept private
- ✅ Only relative file paths shown (within task context)
- ✅ Reduces attack surface for path traversal attempts

### Access Control Preserved
- ✅ All existing access control checks remain in place
- ✅ User isolation still enforced
- ✅ Admin privileges still work
- ✅ Audit logging continues to function

## Testing

### CLI Testing
```bash
# Start master
./master/masterNode

# View task files
master> task-files task-1763370124 admin
```

**Expected Output**:
```
╔═══════════════════════════════════════════════════════
║  Task Files: task-1763370124
╠═══════════════════════════════════════════════════════
║  Task Name:  cloudai-cpu-heavy-1763370124
║  Owner:      admin
║  Timestamp:  2025-11-17 14:32:04
║  Total Size: 245.67 KB
║  Files:      1
╠═══════════════════════════════════════════════════════
║  [1] sort_benchmark.csv (245.67 KB)
╚═══════════════════════════════════════════════════════

💡 To download all files:
   download task-1763370124 admin
```

### HTTP API Testing
```bash
# List user files with sizes
curl "http://localhost:8080/api/files?user_id=admin&requesting_user=admin"
```

**Expected Response**:
```json
{
    "user_id": "admin",
    "tasks": [
        {
            "task_id": "task-1763370124",
            "task_name": "cloudai-cpu-heavy-1763370124",
            "timestamp": "2025-11-17 14:32:04",
            "files": [
                {
                    "path": "sort_benchmark.csv",
                    "size": 251567
                }
            ],
            "total_size": 251567
        }
    ],
    "count": 1
}
```

### Size Formatting Testing
```go
formatFileSize(512)          // "512 B"
formatFileSize(1024)         // "1.00 KB"
formatFileSize(1536)         // "1.50 KB"
formatFileSize(1048576)      // "1.00 MB"
formatFileSize(1234567890)   // "1.15 GB"
```

## Backward Compatibility

### Deprecated Fields
- `FilePaths []string` in `FileMetadata` - still populated for compatibility
- Can be removed in future major version

### Migration Path
1. **Phase 1 (Current)**: Both `FilePaths` and `Files` populated
2. **Phase 2**: Deprecation warning in logs
3. **Phase 3**: Remove `FilePaths` in v2.0

## User Benefits

### Better Information
- See file sizes without downloading
- Understand storage usage per task
- Make informed decisions about downloads

### Security
- No exposure to internal paths
- Reduces information leakage
- Maintains secure access control

### Usability
- Human-readable sizes (KB, MB, GB)
- Individual file sizes shown
- Total size at task level

## Future Enhancements

### Potential Additions
- File modification timestamps
- File content types (MIME types)
- Checksums (MD5/SHA256)
- Compression status
- Download counts

### API Enhancements
- Sorting by size/name/date
- Filtering by file type
- Bulk operations
- Pagination for large file lists

## Production Considerations

### Performance
- File size collection adds minimal overhead (stat calls during walk)
- No additional disk I/O beyond existing file listing
- Size information cached in metadata

### Storage
- `TotalSize` field adds 8 bytes per task
- `FileInfo` adds ~24 bytes per file (vs ~16 for just path)
- Negligible for typical file counts

### Monitoring
- Track total storage per user
- Alert on quota approaching limits
- Analyze storage patterns

## Conclusion

This enhancement improves security by hiding internal storage paths while providing users with more useful information (file sizes). The changes maintain backward compatibility and require no database migrations. All existing functionality continues to work with added benefits of size information in both CLI and HTTP API.

**Status**: ✅ Implemented, Tested, Deployed
