package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"sermonflow-client/internal/config"
	"sermonflow-client/internal/presentation"
	"sermonflow-client/internal/propresenter"

	"cloud.google.com/go/storage"
)

// Sync downloads a presentation described by a presentation.json object in
// GCS: media assets go to Media/Assets, the .pro file goes to the Sermonflow
// library, and ProPresenter is triggered once everything is in place.
func Sync(ctx context.Context, client *storage.Client, cfg *config.Config, manifestObject string) error {
	bucket := client.Bucket(cfg.Bucket)

	// All other objects live under the same GCS prefix as the manifest,
	// e.g. "output/presentation.json" -> "output/".
	prefix := path.Dir(manifestObject)
	if prefix == "." {
		prefix = ""
	}

	p, err := fetchPresentation(ctx, bucket, manifestObject)
	if err != nil {
		return err
	}

	// Derive local roots from the ProPresenter library root.
	mediaAssetsRoot := filepath.Join(filepath.Dir(cfg.PPLibraryRoot), "Media", "Assets")
	libraryRoot := filepath.Join(filepath.Dir(cfg.PPLibraryRoot), "Libraries", "Sermonflow")

	fmt.Printf("manifest gs://%s/%s: presentation %q, %d assets\n",
		cfg.Bucket, manifestObject, p.Name, len(p.Assets))

	if err := os.MkdirAll(mediaAssetsRoot, 0755); err != nil {
		return fmt.Errorf("create media assets root: %w", err)
	}
	if err := os.MkdirAll(libraryRoot, 0755); err != nil {
		return fmt.Errorf("create library root: %w", err)
	}

	// Assets first, so the presentation never references missing media.
	for _, asset := range p.Assets {
		object := path.Join(prefix, asset)
		destination := filepath.Join(mediaAssetsRoot, filepath.FromSlash(asset))
		if err := pull(ctx, bucket, cfg.Bucket, object, destination); err != nil {
			return fmt.Errorf("asset %s: %w", asset, err)
		}
	}

	proDestination := filepath.Join(libraryRoot, p.ProFile())
	if err := pull(ctx, bucket, cfg.Bucket, path.Join(prefix, p.ProFile()), proDestination); err != nil {
		return fmt.Errorf("pro file: %w", err)
	}

	fmt.Printf("synced presentation %q: %d assets -> %s, pro file -> %s\n",
		p.Name, len(p.Assets), mediaAssetsRoot, proDestination)

	ppClient := propresenter.NewClient(cfg.PPAPIBaseURL, cfg.PPAPIPassword)
	if err := ppClient.TriggerPresentation(proDestination); err != nil {
		return fmt.Errorf("trigger presentation: %w", err)
	}

	return nil
}

// fetchPresentation downloads and parses the presentation.json object.
func fetchPresentation(ctx context.Context, bucket *storage.BucketHandle, object string) (*presentation.Presentation, error) {
	reader, err := bucket.Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("open manifest %s: %w", object, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", object, err)
	}

	return presentation.Parse(data)
}

// pull fetches a GCS object to a local path, writing atomically. It skips
// the download when the local file already matches the object's size.
func pull(ctx context.Context, bucket *storage.BucketHandle, bucketName, object, destination string) error {
	reader, err := bucket.Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("open %s: %w", object, err)
	}
	defer reader.Close()

	if info, err := os.Stat(destination); err == nil && info.Size() == reader.Attrs.Size {
		fmt.Printf("  up to date  %s (%s)\n", destination, formatSize(info.Size()))
		return nil
	}

	fmt.Printf("  pulling     gs://%s/%s -> %s (%s)\n",
		bucketName, object, destination, formatSize(reader.Attrs.Size))

	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("create directory for %s: %w", destination, err)
	}

	if err := saveFile(destination, reader); err != nil {
		return fmt.Errorf("save %s: %w", object, err)
	}
	return nil
}

// formatSize renders a byte count as a human-readable string.
func formatSize(bytes int64) string {
	const mb = 1024 * 1024
	if bytes < mb {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/mb)
}

// saveFile writes an io.Reader to disk atomically using a temporary file.
func saveFile(destination string, reader io.Reader) error {
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

	return nil
}
