package config

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// GCPSecretManager wraps the GCP Secret Manager client.
type GCPSecretManager struct {
	client *secretmanager.Client
}

// NewGCPSecretManager creates a new GCP Secret Manager client.
func NewGCPSecretManager(ctx context.Context) (*GCPSecretManager, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &GCPSecretManager{client: client}, nil
}

// AccessSecret retrieves a secret from GCP Secret Manager.
// Returns the secret value or an error if the secret doesn't exist or access is denied.
func (sm *GCPSecretManager) AccessSecret(ctx context.Context, projectID, secretName string) (string, error) {
	// Build the secret resource name
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secretName)

	// Access the secret version
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := sm.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", secretName, err)
	}

	return string(result.Payload.Data), nil
}

// Close closes the Secret Manager client.
func (sm *GCPSecretManager) Close() error {
	if sm.client != nil {
		return sm.client.Close()
	}
	return nil
}
