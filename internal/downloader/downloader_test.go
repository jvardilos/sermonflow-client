package downloader

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"sermonflow-client/internal/config"
)

// TestSaveFileSuccess tests the core file-saving logic.
func TestSaveFileSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	destination := filepath.Join(tmpDir, "test.probundle")
	testData := []byte("test bundle content")

	reader := io.NopCloser(bytes.NewReader(testData))
	err := saveFile(destination, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("file content mismatch: got %q, want %q", string(data), string(testData))
	}

	// Verify no partial file left behind
	partial := destination + ".part"
	if _, err := os.Stat(partial); err == nil {
		t.Error("partial file should not exist after successful save")
	}
}

func TestSaveFileEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	destination := filepath.Join(tmpDir, "empty.probundle")

	reader := io.NopCloser(bytes.NewReader([]byte{}))
	err := saveFile(destination, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestSaveFileLargeContent(t *testing.T) {
	tmpDir := t.TempDir()
	destination := filepath.Join(tmpDir, "large.probundle")

	// 10MB file
	largeData := make([]byte, 10*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	reader := io.NopCloser(bytes.NewReader(largeData))
	err := saveFile(destination, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !bytes.Equal(data, largeData) {
		t.Error("large file content mismatch")
	}
}

func TestSaveFileCreateError(t *testing.T) {
	// Try to save to a read-only directory
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	os.MkdirAll(readOnlyDir, 0555)

	destination := filepath.Join(readOnlyDir, "test.probundle")
	reader := io.NopCloser(bytes.NewReader([]byte("data")))

	err := saveFile(destination, reader)
	if err == nil {
		t.Fatal("expected error when saving to read-only directory")
	}

	// Verify partial file was cleaned up
	partial := destination + ".part"
	if _, err := os.Stat(partial); err == nil {
		t.Error("partial file should be cleaned up on error")
	}
}

func TestSaveFileReaderError(t *testing.T) {
	tmpDir := t.TempDir()
	destination := filepath.Join(tmpDir, "test.probundle")

	reader := io.NopCloser(&errReader{})
	err := saveFile(destination, reader)
	if err == nil {
		t.Fatal("expected error when reader fails")
	}

	// Verify partial file was cleaned up
	partial := destination + ".part"
	if _, err := os.Stat(partial); err == nil {
		t.Error("partial file should be cleaned up on read error")
	}
}

// errReader is a reader that always returns an error.
type errReader struct{}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestDestinationPath(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		wantFile   string
	}{
		{"simple filename", "bundle.probundle", "bundle.probundle"},
		{"nested path", "path/to/bundle.probundle", "bundle.probundle"},
		{"deep nesting", "a/b/c/d/bundle.probundle", "bundle.probundle"},
	}

	tmpDir := t.TempDir()
	cfg := &config.Config{WorkspaceDir: tmpDir}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Base(tt.objectName)
			if filename != tt.wantFile {
				t.Errorf("filepath.Base(%q) = %q, want %q", tt.objectName, filename, tt.wantFile)
			}

			destination := filepath.Join(cfg.WorkspaceDir, filename)
			// Verify destination is within workspace
			if !isInDir(destination, cfg.WorkspaceDir) {
				t.Errorf("destination %q should be in workspace %q", destination, cfg.WorkspaceDir)
			}
		})
	}
}

func isInDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel[0:1] != "."
}
