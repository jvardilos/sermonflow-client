package downloader

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestSaveFileSuccess tests the core file-saving logic.
func TestSaveFileSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	destination := filepath.Join(tmpDir, "test.mov")
	testData := []byte("test asset content")

	err := saveFile(destination, bytes.NewReader(testData))
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
	destination := filepath.Join(tmpDir, "empty.pro")

	err := saveFile(destination, bytes.NewReader([]byte{}))
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
	destination := filepath.Join(tmpDir, "large.mov")

	// 10MB file
	largeData := make([]byte, 10*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err := saveFile(destination, bytes.NewReader(largeData))
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

	destination := filepath.Join(readOnlyDir, "test.mov")

	err := saveFile(destination, bytes.NewReader([]byte("data")))
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
	destination := filepath.Join(tmpDir, "test.mov")

	err := saveFile(destination, &errReader{})
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
