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
			json: `{"bucket":"my-bucket","name":"output/presentation.json"}`,
			want: gcsNotification{Bucket: "my-bucket", Name: "output/presentation.json"},
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

// TestIsManifest tests that only manifest.json objects trigger a sync.
func TestIsManifest(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		want       bool
	}{
		{"bare manifest", "presentation.json", true},
		{"nested manifest", "output/presentation.json", true},
		{"deeply nested manifest", "a/b/c/presentation.json", true},
		{"asset file", "output/Boss.mov", false},
		{"pro file", "output/Presentation.pro", false},
		{"probundle", "output/Presentation.probundle", false},
		{"probundle sidecar json", "output/Presentation.probundle.json", false},
		{"legacy manifest json", "output/manifest.json", false},
		{"manifest-like suffix", "output/not-presentation.json", false},
		{"empty name", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isManifest(tt.objectName)
			if got != tt.want {
				t.Errorf("isManifest(%q) = %v, want %v", tt.objectName, got, tt.want)
			}
		})
	}
}
