package listener

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"sermonflow-client/internal/config"
	"sermonflow-client/internal/downloader"
)

type mockPubsubMessage struct {
	data       []byte
	ackCalled  bool
	nackCalled bool
}

func (m *mockPubsubMessage) Ack() {
	m.ackCalled = true
}

func (m *mockPubsubMessage) Nack() {
	m.nackCalled = true
}

func (m *mockPubsubMessage) Data() []byte {
	return m.data
}

type mockObjectReader struct {
	data []byte
	pos  int
	err  error
}

func (m *mockObjectReader) Read(p []byte) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n := copy(p, m.data[m.pos:])
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

type mockBucketHandle struct {
	objects map[string]downloader.ObjectReader
}

func (m *mockBucketHandle) Object(name string) downloader.ObjectReader {
	if obj, exists := m.objects[name]; exists {
		return obj
	}
	return &mockObjectHandle{readErr: errors.New("not found")}
}

type mockStorageClient struct {
	buckets map[string]downloader.BucketHandle
}

func (m *mockStorageClient) Bucket(name string) downloader.BucketHandle {
	if bucket, exists := m.buckets[name]; exists {
		return bucket
	}
	return &mockBucketHandle{objects: make(map[string]downloader.ObjectReader)}
}

type mockSubscriber struct {
	messages []PubsubMessage
	handler  func(context.Context, PubsubMessage)
}

func (m *mockSubscriber) Receive(ctx context.Context, handler func(context.Context, PubsubMessage)) error {
	for _, msg := range m.messages {
		handler(ctx, msg)
	}
	return nil
}

func TestHandleMessageValidProBundle(t *testing.T) {
	tmpDir := t.TempDir()
	notif := gcsNotification{
		Bucket: "test-bucket",
		Name:   "test.probundle",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	msg := &mockPubsubMessage{data: data}
	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	mockBucket := &mockBucketHandle{objects: make(map[string]downloader.ObjectReader)}
	mockBucket.objects["test.probundle"] = &mockObjectHandle{data: []byte("test content")}
	mockStorage.buckets["test-bucket"] = mockBucket

	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	HandleMessageWithDeps(context.Background(), msg, mockStorage, cfg)

	if !msg.ackCalled {
		t.Error("message should be acked after successful download")
	}
	if msg.nackCalled {
		t.Error("message should not be nacked after successful download")
	}
}

func TestHandleMessageInvalidJSON(t *testing.T) {
	msg := &mockPubsubMessage{data: []byte("invalid json")}
	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	cfg := &config.Config{}

	HandleMessageWithDeps(context.Background(), msg, mockStorage, cfg)

	if !msg.ackCalled {
		t.Error("message should be acked even with invalid JSON")
	}
	if msg.nackCalled {
		t.Error("message should not be nacked for invalid JSON")
	}
}

func TestHandleMessageNonProBundle(t *testing.T) {
	notif := gcsNotification{
		Bucket: "test-bucket",
		Name:   "test.txt",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	msg := &mockPubsubMessage{data: data}
	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	cfg := &config.Config{}

	HandleMessageWithDeps(context.Background(), msg, mockStorage, cfg)

	if !msg.ackCalled {
		t.Error("message should be acked for non-.probundle files")
	}
	if msg.nackCalled {
		t.Error("message should not be nacked for non-.probundle files")
	}
}

func TestHandleMessageDownloadError(t *testing.T) {
	tmpDir := t.TempDir()
	notif := gcsNotification{
		Bucket: "test-bucket",
		Name:   "fail.probundle",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	msg := &mockPubsubMessage{data: data}
	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	mockBucket := &mockBucketHandle{objects: make(map[string]downloader.ObjectReader)}
	mockBucket.objects["fail.probundle"] = &mockObjectHandle{readErr: errors.New("connection failed")}
	mockStorage.buckets["test-bucket"] = mockBucket

	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	HandleMessageWithDeps(context.Background(), msg, mockStorage, cfg)

	if msg.ackCalled {
		t.Error("message should not be acked when download fails")
	}
	if !msg.nackCalled {
		t.Error("message should be nacked when download fails")
	}
}

func TestRunWithDeps(t *testing.T) {
	tmpDir := t.TempDir()
	notif := gcsNotification{
		Bucket: "test-bucket",
		Name:   "test.probundle",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	msg := &mockPubsubMessage{data: data}
	mockSubscriber := &mockSubscriber{
		messages: []PubsubMessage{msg},
	}

	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	mockBucket := &mockBucketHandle{objects: make(map[string]downloader.ObjectReader)}
	mockBucket.objects["test.probundle"] = &mockObjectHandle{data: []byte("bundle content")}
	mockStorage.buckets["test-bucket"] = mockBucket

	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	err = RunWithDeps(context.Background(), cfg, mockSubscriber, mockStorage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !msg.ackCalled {
		t.Error("message should be acked")
	}
}

func TestRunWithDepsContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create a subscriber that returns context.Canceled error
	mockSubscriber := &mockSubscriberWithError{
		err: context.Canceled,
	}
	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	cfg := &config.Config{}

	err := RunWithDeps(ctx, cfg, mockSubscriber, mockStorage)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

type mockSubscriberWithError struct {
	err error
}

func (m *mockSubscriberWithError) Receive(ctx context.Context, handler func(context.Context, PubsubMessage)) error {
	return m.err
}

func TestGCSNotificationUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    gcsNotification
		wantErr bool
	}{
		{
			name:    "valid notification",
			json:    `{"bucket":"my-bucket","name":"file.probundle"}`,
			want:    gcsNotification{Bucket: "my-bucket", Name: "file.probundle"},
			wantErr: false,
		},
		{
			name:    "empty fields",
			json:    `{"bucket":"","name":""}`,
			want:    gcsNotification{Bucket: "", Name: ""},
			wantErr: false,
		},
		{
			name:    "missing name field",
			json:    `{"bucket":"test"}`,
			want:    gcsNotification{Bucket: "test", Name: ""},
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notif gcsNotification
			err := json.Unmarshal([]byte(tt.json), &notif)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && notif != tt.want {
				t.Errorf("Unmarshal() = %v, want %v", notif, tt.want)
			}
		})
	}
}

func TestHandleMessageMultipleBundles(t *testing.T) {
	tmpDir := t.TempDir()
	mockStorage := &mockStorageClient{buckets: make(map[string]downloader.BucketHandle)}
	mockBucket := &mockBucketHandle{objects: make(map[string]downloader.ObjectReader)}
	mockBucket.objects["first.probundle"] = &mockObjectHandle{data: []byte("first")}
	mockBucket.objects["second.probundle"] = &mockObjectHandle{data: []byte("second")}
	mockStorage.buckets["test-bucket"] = mockBucket

	cfg := &config.Config{
		Bucket:       "test-bucket",
		WorkspaceDir: tmpDir,
	}

	// First message
	notif1 := gcsNotification{Bucket: "test-bucket", Name: "first.probundle"}
	data1, _ := json.Marshal(notif1)
	msg1 := &mockPubsubMessage{data: data1}

	HandleMessageWithDeps(context.Background(), msg1, mockStorage, cfg)
	if !msg1.ackCalled {
		t.Error("first message should be acked")
	}

	// Second message
	notif2 := gcsNotification{Bucket: "test-bucket", Name: "second.probundle"}
	data2, _ := json.Marshal(notif2)
	msg2 := &mockPubsubMessage{data: data2}

	HandleMessageWithDeps(context.Background(), msg2, mockStorage, cfg)
	if !msg2.ackCalled {
		t.Error("second message should be acked")
	}
}
