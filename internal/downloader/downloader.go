package downloader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"sermonflow-client/internal/bundle"
	"sermonflow-client/internal/config"
	"sermonflow-client/internal/propresenter"

	"cloud.google.com/go/storage"
)

// Download fetches a file from GCS, unpacks it, and triggers ProPresenter.
func Download(ctx context.Context, client *storage.Client, cfg *config.Config, objectName string) error {
	reader, err := client.Bucket(cfg.Bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

	filename := filepath.Base(objectName)
	destination := filepath.Join(cfg.WorkspaceDir, filename)

	// Download the bundle to a temporary location
	if err := saveFile(destination, reader); err != nil {
		return err
	}

	// Derive Media/Assets root from Libraries root
	// If Libraries root is /path/to/ProPresenter/Libraries,
	// Media/Assets root is /path/to/ProPresenter/Media/Assets
	mediaAssetsRoot := filepath.Join(filepath.Dir(cfg.PPLibraryRoot), "Media", "Assets")
	profileRoot := filepath.Join(filepath.Dir(cfg.PPLibraryRoot), "Libraries", "Sermonflow")

	// Unpack the bundle to two locations
	result, err := bundle.UnpackToDestinations(destination, profileRoot, mediaAssetsRoot)
	if err != nil {
		return fmt.Errorf("unpack bundle: %w", err)
	}

	// Trigger ProPresenter to load the presentation
	ppClient := propresenter.NewClient(cfg.PPAPIBaseURL, cfg.PPAPIPassword)
	if err := ppClient.TriggerPresentation(result.ProFile); err != nil {
		return fmt.Errorf("trigger presentation: %w", err)
	}

	return nil
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
