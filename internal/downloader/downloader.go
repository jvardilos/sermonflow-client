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

type ObjectReader interface {
	NewReader(ctx context.Context) (io.ReadCloser, error)
}

type BucketHandle interface {
	Object(name string) ObjectReader
}

type StorageClient interface {
	Bucket(name string) BucketHandle
}

func Download(ctx context.Context, client StorageClient, cfg *config.Config, objectName string) error {
	logger := slog.Default()

	filename := filepath.Base(objectName)
	destination := filepath.Join(cfg.WorkspaceDir, filename)
	partial := destination + ".part"

	reader, err := client.Bucket(cfg.Bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("open reader: %w", err)
	}
	defer reader.Close()

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

	logger.Info("downloaded bundle", "object", objectName, "destination", destination)
	return nil
}

// Adapter to make *storage.Client compatible with StorageClient interface
type storageClientAdapter struct {
	*storage.Client
}

func (a *storageClientAdapter) Bucket(name string) BucketHandle {
	return &bucketHandleAdapter{a.Client.Bucket(name)}
}

type bucketHandleAdapter struct {
	*storage.BucketHandle
}

func (a *bucketHandleAdapter) Object(name string) ObjectReader {
	return &objectHandleAdapter{a.BucketHandle.Object(name)}
}

type objectHandleAdapter struct {
	*storage.ObjectHandle
}

func (a *objectHandleAdapter) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return a.ObjectHandle.NewReader(ctx)
}
