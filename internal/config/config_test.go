package config

import (
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

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credFile)
	t.Setenv("GCP_PROJECT_ID", "test-project")
	t.Setenv("PUBSUB_SUBSCRIPTION", "test-sub")
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("PROPRESENTER_WORKSPACE_DIR", tmpDir)

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
	if cfg.WorkspaceDir != tmpDir {
		t.Errorf("WorkspaceDir = %q, want %q", cfg.WorkspaceDir, tmpDir)
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
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "PUBSUB_SUBSCRIPTION": "sub", "GCS_BUCKET": "bucket", "PROPRESENTER_WORKSPACE_DIR": "/tmp"},
			wantMiss: "GCP_PROJECT_ID",
		},
		{
			name:     "missing PUBSUB_SUBSCRIPTION",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "GCP_PROJECT_ID": "proj", "GCS_BUCKET": "bucket", "PROPRESENTER_WORKSPACE_DIR": "/tmp"},
			wantMiss: "PUBSUB_SUBSCRIPTION",
		},
		{
			name:     "missing GCS_BUCKET",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "GCP_PROJECT_ID": "proj", "PUBSUB_SUBSCRIPTION": "sub", "PROPRESENTER_WORKSPACE_DIR": "/tmp"},
			wantMiss: "GCS_BUCKET",
		},
		{
			name:     "missing PROPRESENTER_WORKSPACE_DIR",
			setVars:  map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": "creds", "GCP_PROJECT_ID": "proj", "PUBSUB_SUBSCRIPTION": "sub", "GCS_BUCKET": "bucket"},
			wantMiss: "PROPRESENTER_WORKSPACE_DIR",
		},
		{
			name:     "missing GOOGLE_APPLICATION_CREDENTIALS",
			setVars:  map[string]string{"GCP_PROJECT_ID": "proj", "PUBSUB_SUBSCRIPTION": "sub", "GCS_BUCKET": "bucket", "PROPRESENTER_WORKSPACE_DIR": "/tmp"},
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
	t.Setenv("PROPRESENTER_WORKSPACE_DIR", "/tmp")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing credentials file")
	}
	if !contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Errorf("error should mention credentials file, got: %v", err)
	}
}

func TestLoadWorkspaceDirNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "creds.json")
	if err := os.WriteFile(credFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credFile)
	t.Setenv("GCP_PROJECT_ID", "test-project")
	t.Setenv("PUBSUB_SUBSCRIPTION", "test-sub")
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("PROPRESENTER_WORKSPACE_DIR", "/nonexistent/workspace")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing workspace directory")
	}
	if !contains(err.Error(), "PROPRESENTER_WORKSPACE_DIR") {
		t.Errorf("error should mention workspace dir, got: %v", err)
	}
}

func TestLoadWorkspaceDirIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "creds.json")
	if err := os.WriteFile(credFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	workspaceFile := filepath.Join(tmpDir, "workspace")
	if err := os.WriteFile(workspaceFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credFile)
	t.Setenv("GCP_PROJECT_ID", "test-project")
	t.Setenv("PUBSUB_SUBSCRIPTION", "test-sub")
	t.Setenv("GCS_BUCKET", "test-bucket")
	t.Setenv("PROPRESENTER_WORKSPACE_DIR", workspaceFile)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when workspace is a file")
	}
	if !contains(err.Error(), "PROPRESENTER_WORKSPACE_DIR") {
		t.Errorf("error should mention workspace dir, got: %v", err)
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
