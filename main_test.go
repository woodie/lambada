package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAttachment(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "test.pdf")

	content := "fake pdf content"
	err := saveAttachment(strings.NewReader(content), dest)
	if err != nil {
		t.Fatalf("saveAttachment returned error: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("could not read saved file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestSaveAttachmentBadPath(t *testing.T) {
	err := saveAttachment(strings.NewReader("data"), "/nonexistent/dir/file.pdf")
	if err == nil {
		t.Error("expected error for bad path, got nil")
	}
}

func TestCleanupOldFiles_KeepsNewFile(t *testing.T) {
	dir := t.TempDir()
	attachmentDir = dir

	path := filepath.Join(dir, "1234567890.pdf")
	os.WriteFile(path, []byte("data"), 0644)

	cleanupOldFiles()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected new file to be kept, but it was deleted")
	}
}

func TestCleanupOldFiles_DeletesOldFile(t *testing.T) {
	dir := t.TempDir()
	attachmentDir = dir

	path := filepath.Join(dir, "1234567890.pdf")
	os.WriteFile(path, []byte("data"), 0644)

	// Wind back the mtime by 25 hours
	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(path, old, old)

	cleanupOldFiles()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted, but it still exists")
	}
}

func TestCleanupOldFiles_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	attachmentDir = dir

	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)

	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(subdir, old, old)

	cleanupOldFiles() // should not attempt to remove the subdir

	if _, err := os.Stat(subdir); os.IsNotExist(err) {
		t.Error("expected subdirectory to be kept, but it was deleted")
	}
}
