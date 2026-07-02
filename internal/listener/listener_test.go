package listener

import (
	"encoding/json"
	"testing"
)

// TestGCSNotificationParsing tests that we correctly unmarshal GCS notifications.
func TestGCSNotificationParsing(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    gcsNotification
		wantErr bool
	}{
		{
			name: "valid notification",
			json: `{"bucket":"my-bucket","name":"file.probundle"}`,
			want: gcsNotification{Bucket: "my-bucket", Name: "file.probundle"},
		},
		{
			name: "missing name field",
			json: `{"bucket":"test"}`,
			want: gcsNotification{Bucket: "test", Name: ""},
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

// TestProbundleFilter tests that we correctly filter for .probundle files.
func TestProbundleFilter(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		want       bool
	}{
		{"simple probundle", "test.probundle", true},
		{"nested probundle", "path/to/test.probundle", true},
		{"txt file", "test.txt", false},
		{"missing extension", "probundle", false},
		{"wrong extension", "test.bundle", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProbundle(tt.objectName)
			if got != tt.want {
				t.Errorf("isProbundle(%q) = %v, want %v", tt.objectName, got, tt.want)
			}
		})
	}
}

// isProbundle checks if a filename is a .probundle file.
// This is the testable business logic extracted from handleMessage.
func isProbundle(objectName string) bool {
	const ext = ".probundle"
	return len(objectName) >= len(ext) && objectName[len(objectName)-len(ext):] == ext
}
