package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	CredentialsPath string
	ProjectID       string
	Subscription    string
	Bucket          string
	WorkspaceDir    string
}

// Load reads configuration from environment variables or GCP Secret Manager.
// Prefers env vars for local development, falls back to Secret Manager for production.
func Load() (*Config, error) {
	return LoadWithContext(context.Background())
}

// LoadWithContext allows passing a context for Secret Manager calls.
func LoadWithContext(ctx context.Context) (*Config, error) {
	cfg := &Config{
		CredentialsPath: getConfigValue(ctx, "GOOGLE_APPLICATION_CREDENTIALS", "gcp-credentials-path"),
		ProjectID:       getConfigValue(ctx, "GCP_PROJECT_ID", "gcp-project-id"),
		Subscription:    getConfigValue(ctx, "PUBSUB_SUBSCRIPTION", "pubsub-subscription"),
		Bucket:          getConfigValue(ctx, "GCS_BUCKET", "gcs-bucket"),
		WorkspaceDir:    getConfigValue(ctx, "PROPRESENTER_WORKSPACE_DIR", "workspace-dir"),
	}

	required := map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": cfg.CredentialsPath,
		"GCP_PROJECT_ID":                 cfg.ProjectID,
		"PUBSUB_SUBSCRIPTION":            cfg.Subscription,
		"GCS_BUCKET":                     cfg.Bucket,
		"PROPRESENTER_WORKSPACE_DIR":     cfg.WorkspaceDir,
	}

	var missing []string
	for key, val := range required {
		if val == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %v", missing)
	}

	if _, err := os.Stat(cfg.CredentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS does not exist: %s", cfg.CredentialsPath)
	}

	if info, err := os.Stat(cfg.WorkspaceDir); os.IsNotExist(err) || !info.IsDir() {
		return nil, fmt.Errorf("PROPRESENTER_WORKSPACE_DIR does not exist or is not a directory: %s", cfg.WorkspaceDir)
	}

	return cfg, nil
}

// getConfigValue gets a config value from env var, falls back to Secret Manager.
// envVar: environment variable name to check first (e.g., "GCP_PROJECT_ID")
// secretName: Secret Manager secret name (e.g., "gcp-project-id")
func getConfigValue(ctx context.Context, envVar, secretName string) string {
	// Prefer environment variable (for local dev with .env)
	if val := os.Getenv(envVar); val != "" {
		return val
	}

	// Fall back to Secret Manager (for production/shared machines)
	if val, err := getSecretValue(ctx, secretName); err == nil {
		return val
	}

	return ""
}

// getSecretValue fetches a secret from GCP Secret Manager.
func getSecretValue(ctx context.Context, secretName string) (string, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		// Try to read from environment, but this might be called during config load
		return "", fmt.Errorf("GCP_PROJECT_ID not set, cannot fetch from Secret Manager")
	}

	client, err := newSecretManagerClient(ctx)
	if err != nil {
		return "", fmt.Errorf("secret manager client: %w", err)
	}
	defer client.Close()

	return client.AccessSecret(ctx, projectID, secretName)
}

// secretManagerClient is a wrapper for GCP Secret Manager operations.
type secretManagerClient interface {
	AccessSecret(ctx context.Context, projectID, secretName string) (string, error)
	Close() error
}

// newSecretManagerClient creates a new Secret Manager client.
// This is separated to allow testing with mock clients.
func newSecretManagerClient(ctx context.Context) (secretManagerClient, error) {
	return NewGCPSecretManager(ctx)
}

// Helper to ensure credentials path is absolute (required by some GCP libraries)
func (c *Config) ExpandCredentialsPath() error {
	if !filepath.IsAbs(c.CredentialsPath) {
		abs, err := filepath.Abs(c.CredentialsPath)
		if err != nil {
			return fmt.Errorf("expand credentials path: %w", err)
		}
		c.CredentialsPath = abs
	}
	return nil
}
