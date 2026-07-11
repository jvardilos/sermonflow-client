package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "creds.json")
	if err := os.WriteFile(credFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	librariesDir := filepath.Join(tmpDir, "Libraries")
	os.MkdirAll(librariesDir, 0755)

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credFile)
	t.Setenv("GCP_PROJECT_ID", "test-project")
	t.Setenv("PUBSUB_SUBSCRIPTION", "test-sub")
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("PROPRESENTER_LIBRARY_ROOT", librariesDir)
	t.Setenv("PROPRESENTER_API_URL", "http://localhost:50000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ProjectID != "test-project" {
		t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "test-project")
	}
	if cfg.Subscription != "test-sub" {
		t.Errorf("Subscription = %q, want %q", cfg.Subscription, "test-sub")
	}
	if cfg.Bucket != "test-bucket" {
		t.Errorf("Bucket = %q, want %q", cfg.Bucket, "test-bucket")
	}
	if cfg.CredentialsPath != credFile {
		t.Errorf("CredentialsPath = %q, want %q", cfg.CredentialsPath, credFile)
	}
}

func TestLoadMissingEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		setVars  map[string]string
		wantMiss string
	}{
		{
			name:     "missing GCP_PROJECT_ID",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "PUBSUB_SUBSCRIPTION": "sub", "GCS_BUCKET": "bucket", "PROPRESENTER_LIBRARY_ROOT": "/lib", "PROPRESENTER_API_URL": "http://localhost"},
			wantMiss: "GCP_PROJECT_ID",
		},
		{
			name:     "missing PUBSUB_SUBSCRIPTION",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "GCP_PROJECT_ID": "proj", "GCS_BUCKET": "bucket", "PROPRESENTER_LIBRARY_ROOT": "/lib", "PROPRESENTER_API_URL": "http://localhost"},
			wantMiss: "PUBSUB_SUBSCRIPTION",
		},
		{
			name:     "missing GCS_BUCKET",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "GCP_PROJECT_ID": "proj", "PUBSUB_SUBSCRIPTION": "sub", "PROPRESENTER_LIBRARY_ROOT": "/lib", "PROPRESENTER_API_URL": "http://localhost"},
			wantMiss: "GCS_BUCKET",
		},
		{
			name:     "missing PROPRESENTER_LIBRARY_ROOT",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "GCP_PROJECT_ID": "proj", "PUBSUB_SUBSCRIPTION": "sub", "GCS_BUCKET": "bucket", "PROPRESENTER_API_URL": "http://localhost"},
			wantMiss: "PROPRESENTER_LIBRARY_ROOT",
		},
		{
			name:     "missing GOOGLE_APPLICATION_CREDENTIALS",
			setVars:  map[string]string{"GCP_PROJECT_ID": "proj", "PUBSUB_SUBSCRIPTION": "sub", "GCS_BUCKET": "bucket", "PROPRESENTER_LIBRARY_ROOT": "/lib", "PROPRESENTER_API_URL": "http://localhost"},
			wantMiss: "GOOGLE_APPLICATION_CREDENTIALS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range tt.setVars {
				t.Setenv(k, v)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("expected error for missing env var")
			}
			if !contains(err.Error(), tt.wantMiss) {
				t.Errorf("error message should mention %q, got: %v", tt.wantMiss, err)
			}
		})
	}
}

func TestLoadCredsFileNotFound(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/file")
	t.Setenv("GCP_PROJECT_ID", "test-project")
	t.Setenv("PUBSUB_SUBSCRIPTION", "test-sub")
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("PROPRESENTER_LIBRARY_ROOT", "/lib")
	t.Setenv("PROPRESENTER_API_URL", "http://localhost")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing credentials file")
	}
	if !contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Errorf("error should mention credentials file, got: %v", err)
	}
}

func TestGetConfigValuePreferesEnvVar(t *testing.T) {
	t.Setenv("TEST_VAR", "from-env")
	// Even though we can't mock Secret Manager in this test, we verify env var takes precedence
	val := getConfigValue(context.Background(), "TEST_VAR", "non-existent-secret")
	if val != "from-env" {
		t.Errorf("getConfigValue should prefer env var, got %q", val)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
