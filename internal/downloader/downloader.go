package downloader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sermonflow-client/internal/config"

	"cloud.google.com/go/storage"
)

// Download fetches a file from GCS and saves it to the workspace.
func Download(ctx context.Context, client *storage.Client, cfg *config.Config, objectName string) error {
	reader, err := client.Bucket(cfg.Bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	filename := filepath.Base(objectName)
	destination := filepath.Join(cfg.WorkspaceDir, filename)

	return saveFile(destination, reader)
}

// saveFile writes an io.ReadCloser to disk atomically using a temporary file.
// This is the testable core logic, separate from cloud client concerns.
func saveFile(destination string, reader io.ReadCloser) error {
	defer reader.Close()

	logger := slog.Default()
	partial := destination + ".part"

	tmpFile, err := os.Create(partial)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		os.Remove(partial)
		return fmt.Errorf("copy data: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(partial)
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(partial, destination); err != nil {
		os.Remove(partial)
		return fmt.Errorf("rename to destination: %w", err)
	}

	logger.Info("downloaded bundle", "destination", destination)
	return nil
}
