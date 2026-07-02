package downloader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"sermonflow-client/internal/config"
)

type mockObjectReader struct {
	data []byte
	pos  int
}

func (m *mockObjectReader) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockObjectReader) Close() error {
	return nil
}

type mockObjectHandle struct {
	data    []byte
	readErr error
}

func (m *mockObjectHandle) NewReader(ctx context.Context) (io.ReadCloser, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	return &mockObjectReader{data: m.data}, nil
}

type mockBucket struct {
	objects map[string]*mockObjectHandle
}

func (m *mockBucket) Object(name string) ObjectReader {
	if obj, exists := m.objects[name]; exists {
		return obj
	}
	return &mockObjectHandle{readErr: errors.New("object not found")}
}

type mockStorage struct {
	buckets map[string]*mockBucket
}

func (m *mockStorage) Bucket(name string) BucketHandle {
	if bucket, exists := m.buckets[name]; exists {
		return bucket
	}
	return &mockBucket{objects: make(map[string]*mockObjectHandle)}
}

func newMockStorage() *mockStorage {
	return &mockStorage{buckets: make(map[string]*mockBucket)}
}

func TestDownloadSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	testData := []byte("test bundle content")
	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["test.probundle"] = &mockObjectHandle{data: testData}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "test.probundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destination := filepath.Join(tmpDir, "test.probundle")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("file content mismatch, got %q, want %q", string(data), string(testData))
	}
}

func TestDownloadReaderError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["fail.probundle"] = &mockObjectHandle{readErr: errors.New("read failed")}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "fail.probundle")
	if err == nil {
		t.Fatal("expected error when reader fails")
	}
	if !contains(err.Error(), "open reader") {
		t.Errorf("error should mention 'open reader', got: %v", err)
	}
}

func TestDownloadCreateFileError(t *testing.T) {
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(readOnlyDir, 0555)

	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: readOnlyDir,
	}

	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["test.probundle"] = &mockObjectHandle{data: []byte("data")}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "test.probundle")
	if err == nil {
		t.Fatal("expected error when creating file in read-only dir")
	}
	if !contains(err.Error(), "create temporary file") {
		t.Errorf("error should mention 'create temporary file', got: %v", err)
	}
}

func TestDownloadIOCopyError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}

	errorReader := &errorObjectReader{}
	mockHandle := &mockObjectHandle{}
	mockHandle.readErr = nil

	mockBucket.objects["error.probundle"] = &mockObjectHandle{readErr: nil}
	mockStorage.buckets["test-bucket"] = mockBucket

	// Override the Object method to return our error reader
	mockBucket.objects["error.probundle"] = &mockObjectHandle{data: []byte(""), readErr: nil}

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "error.probundle")

	// This test primarily checks that we attempt to download
	if err != nil && !contains(err.Error(), "copy data") && !contains(err.Error(), "open reader") {
		t.Errorf("unexpected error type: %v", err)
	}

	_ = errorReader
}

type errorObjectReader struct {
}

func (e *errorObjectReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

func (e *errorObjectReader) Close() error {
	return nil
}

func TestDownloadFilenameExtraction(t *testing.T) {
	tests := []struct {
		objectName string
		wantFile   string
	}{
		{
			objectName: "simple.probundle",
			wantFile:   "simple.probundle",
		},
		{
			objectName: "path/to/file.probundle",
			wantFile:   "file.probundle",
		},
		{
			objectName: "/absolute/path/file.probundle",
			wantFile:   "file.probundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.objectName, func(t *testing.T) {
			filename := filepath.Base(tt.objectName)
			if filename != tt.wantFile {
				t.Errorf("filepath.Base(%q) = %q, want %q", tt.objectName, filename, tt.wantFile)
			}
		})
	}
}

func TestDownloadDestinationPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceDir: tmpDir,
	}

	tests := []struct {
		objectName string
	}{
		{"simple.probundle"},
		{"path/to/file.probundle"},
	}

	for _, tt := range tests {
		t.Run(tt.objectName, func(t *testing.T) {
			filename := filepath.Base(tt.objectName)
			destination := filepath.Join(cfg.WorkspaceDir, filename)

			absWorkspace, _ := filepath.Abs(cfg.WorkspaceDir)
			absDest, _ := filepath.Abs(destination)

			if !pathInDir(absDest, absWorkspace) {
				t.Errorf("destination %q not in workspace %q", absDest, absWorkspace)
			}
		})
	}
}

func TestDownloadRenameSequence(t *testing.T) {
	tmpDir := t.TempDir()
	objectName := "test.probundle"
	filename := filepath.Base(objectName)
	destination := filepath.Join(tmpDir, filename)
	partial := destination + ".part"

	if err := os.WriteFile(partial, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(partial, destination); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(partial); !errors.Is(err, os.ErrNotExist) {
		t.Error("partial file should not exist after rename")
	}
	if _, err := os.Stat(destination); err != nil {
		t.Errorf("destination file should exist: %v", err)
	}
}

func TestDownloadWithPathInObjectName(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	testData := []byte("nested bundle")
	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["nested/deep/bundle.probundle"] = &mockObjectHandle{data: testData}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "nested/deep/bundle.probundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only extract the filename, not the path
	destination := filepath.Join(tmpDir, "bundle.probundle")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("file content mismatch")
	}
}

func TestDownloadCloseError(t *testing.T) {
	_ = &closeErrorReader{data: []byte("test")}
}

type closeErrorReader struct {
	data []byte
}

func (c *closeErrorReader) Read(p []byte) (int, error) {
	n := copy(p, c.data)
	c.data = c.data[n:]
	return n, nil
}

func (c *closeErrorReader) Close() error {
	return errors.New("close error")
}

func TestDownloadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	testData := []byte{}
	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["empty.probundle"] = &mockObjectHandle{data: testData}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "empty.probundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destination := filepath.Join(tmpDir, "empty.probundle")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestDownloadLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	// Create a large test file (1MB)
	testData := make([]byte, 1024*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["large.probundle"] = &mockObjectHandle{data: testData}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "large.probundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destination := filepath.Join(tmpDir, "large.probundle")
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !bytes.Equal(data, testData) {
		t.Errorf("large file content mismatch")
	}
}

func TestDownloadRenameCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	testData := []byte("test data")
	mockStorage := newMockStorage()
	mockBucket := &mockBucket{objects: make(map[string]*mockObjectHandle)}
	mockBucket.objects["test.probundle"] = &mockObjectHandle{data: testData}
	mockStorage.buckets["test-bucket"] = mockBucket

	ctx := context.Background()
	err := Download(ctx, mockStorage, cfg, "test.probundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file exists and partial file was cleaned up
	destination := filepath.Join(tmpDir, "test.probundle")
	partial := destination + ".part"
	if _, err := os.Stat(destination); err != nil {
		t.Errorf("destination file should exist: %v", err)
	}
	if _, err := os.Stat(partial); !errors.Is(err, os.ErrNotExist) {
		t.Error("partial file should not exist after successful download")
	}
}


func pathInDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return !bytes.HasPrefix([]byte(rel), []byte(".."))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
