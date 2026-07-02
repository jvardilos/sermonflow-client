package config

import (
	"fmt"
	"os"
)

type Config struct {
	CredentialsPath string
	ProjectID       string
	Subscription    string
	Bucket          string
	WorkspaceDir    string
}

func Load() (*Config, error) {
	cfg := &Config{
		CredentialsPath: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		ProjectID:       os.Getenv("GCP_PROJECT_ID"),
		Subscription:    os.Getenv("PUBSUB_SUBSCRIPTION"),
		Bucket:          os.Getenv("GCS_BUCKET"),
		WorkspaceDir:    os.Getenv("PROPRESENTER_WORKSPACE_DIR"),
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
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	if _, err := os.Stat(cfg.CredentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS does not exist: %s", cfg.CredentialsPath)
	}

	if info, err := os.Stat(cfg.WorkspaceDir); os.IsNotExist(err) || !info.IsDir() {
		return nil, fmt.Errorf("PROPRESENTER_WORKSPACE_DIR does not exist or is not a directory: %s", cfg.WorkspaceDir)
	}

	return cfg, nil
}
