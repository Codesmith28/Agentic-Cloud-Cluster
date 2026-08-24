package controlplane

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func fmtFileSize(bytes int64) string {
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

func (e *Executor) cmdListFiles(requestingUser, targetUser string) CommandOutcome {
	if e.fs == nil {
		return CommandOutcome{
			Transcript: "❌ File storage not available",
			Err:        fmt.Errorf("file storage not available"),
		}
	}

	fileList, err := e.fs.ListUserFilesWithAccess(requestingUser, targetUser)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to list files: %v", err),
			Err:        err,
		}
	}

	if len(fileList) == 0 {
		return CommandOutcome{Transcript: fmt.Sprintf("\n✓ No files found for user '%s'", targetUser)}
	}

	var b strings.Builder
	b.WriteString("\n╔═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  Files for User: %s\n", targetUser))
	b.WriteString(fmt.Sprintf("║  Total Tasks: %d\n", len(fileList)))
	b.WriteString("╠═══════════════════════════════════════════════════════\n")

	for i, metadata := range fileList {
		b.WriteString(fmt.Sprintf("║  [%d] Task: %s\n", i+1, metadata.TaskName))
		b.WriteString(fmt.Sprintf("║      Task ID:   %s\n", metadata.TaskID))
		b.WriteString(fmt.Sprintf("║      Timestamp: %s\n", metadata.Timestamp.Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("║      Files:     %d\n", len(metadata.FilePaths)))
		for _, file := range metadata.FilePaths {
			b.WriteString(fmt.Sprintf("║        - %s\n", file))
		}
		if i < len(fileList)-1 {
			b.WriteString("║      ───────────────────────────────────────────────\n")
		}
	}
	b.WriteString("╚═══════════════════════════════════════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdTaskFiles(taskID, requestingUser, targetUser string) CommandOutcome {
	if e.fs == nil {
		return CommandOutcome{
			Transcript: "❌ File storage not available",
			Err:        fmt.Errorf("file storage not available"),
		}
	}

	metadata, err := e.fs.GetTaskFilesWithAccess(requestingUser, targetUser, taskID)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to get task files: %v", err),
			Err:        err,
		}
	}

	var b strings.Builder
	b.WriteString("\n╔═══════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("║  Task Files: %s\n", taskID))
	b.WriteString(fmt.Sprintf("║  Task Name:  %s\n", metadata.TaskName))
	b.WriteString(fmt.Sprintf("║  Owner:      %s\n", targetUser))
	b.WriteString(fmt.Sprintf("║  Files:      %d\n", len(metadata.Files)))
	b.WriteString("╠═══════════════════════════════════════════════════════\n")

	for i, file := range metadata.Files {
		b.WriteString(fmt.Sprintf("║  [%d] %s (%s)\n", i+1, file.Path, fmtFileSize(file.Size)))
	}
	b.WriteString("╚═══════════════════════════════════════════════════════\n")
	return CommandOutcome{Transcript: b.String()}
}

func (e *Executor) cmdDownload(taskID, requestingUser, targetUser, outputDir string) CommandOutcome {
	if e.fs == nil {
		return CommandOutcome{
			Transcript: "❌ File storage not available",
			Err:        fmt.Errorf("file storage not available"),
		}
	}

	metadata, err := e.fs.GetTaskFilesWithAccess(requestingUser, targetUser, taskID)
	if err != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to get task files: %v", err),
			Err:        err,
		}
	}

	if len(metadata.FilePaths) == 0 {
		return CommandOutcome{Transcript: "❌ No files found for this task"}
	}

	taskOutputDir := filepath.Join(outputDir, taskID)
	if mkErr := os.MkdirAll(taskOutputDir, 0755); mkErr != nil {
		return CommandOutcome{
			Transcript: fmt.Sprintf("❌ Failed to create output directory: %v", mkErr),
			Err:        mkErr,
		}
	}

	successCount := 0
	for _, filePath := range metadata.FilePaths {
		fileData, readErr := e.fs.ReadFileWithAccess(requestingUser, targetUser, taskID, filePath)
		if readErr != nil {
			continue
		}
		outputPath := filepath.Join(taskOutputDir, filePath)
		_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
		if err := os.WriteFile(outputPath, fileData, 0644); err == nil {
			successCount++
		}
	}

	return CommandOutcome{
		Transcript: fmt.Sprintf("✓ Downloaded %d/%d files to %s\n", successCount, len(metadata.FilePaths), taskOutputDir),
	}
}
