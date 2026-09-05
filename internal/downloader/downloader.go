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

	"cloud.google.com/go/storage"
)

// Syncer downloads presentations from GCS and remembers which manifest
// version it last synced, so redelivered or duplicate notifications for an
// unchanged presentation.json are no-ops.
type Syncer struct {
	client *storage.Client
	cfg    *config.Config
	// lastSynced maps a manifest object name to the GCS generation that was
	// last synced successfully. GCS bumps the generation on every overwrite,
	// so an unchanged generation means there is nothing new to sync.
	lastSynced map[string]int64
}

// New creates a Syncer.
func New(client *storage.Client, cfg *config.Config) *Syncer {
	return &Syncer{
		client:     client,
		cfg:        cfg,
		lastSynced: make(map[string]int64),
	}
}

// Sync downloads the presentation described by a presentation.json object in
// GCS: media assets go to Media/Assets and the .pro file goes to the
// Sermonflow library, where ProPresenter picks it up automatically. It does
// nothing when the manifest hasn't changed since the last successful sync.
// A returned error means the sync failed and is worth retrying.
func (s *Syncer) Sync(ctx context.Context, manifestObject string) error {
	bucket := s.client.Bucket(s.cfg.Bucket)

	// All other objects live under the same GCS prefix as the manifest,
	// e.g. "output/presentation.json" -> "output/".
	prefix := path.Dir(manifestObject)
	if prefix == "." {
		prefix = ""
	}

	p, generation, err := fetchPresentation(ctx, bucket, manifestObject)
	if err != nil {
		return err
	}

	if s.lastSynced[manifestObject] == generation {
		fmt.Printf("manifest gs://%s/%s unchanged (generation %d), nothing to sync\n",
			s.cfg.Bucket, manifestObject, generation)
		return nil
	}

	// Derive local roots from the ProPresenter library root.
	mediaAssetsRoot := filepath.Join(filepath.Dir(s.cfg.PPLibraryRoot), "Media", "Assets")
	libraryRoot := filepath.Join(filepath.Dir(s.cfg.PPLibraryRoot), "Libraries", "Sermonflow")

	fmt.Printf("manifest gs://%s/%s (generation %d): presentation %q, %d assets\n",
		s.cfg.Bucket, manifestObject, generation, p.Name, len(p.Assets))

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
		if err := pull(ctx, bucket, s.cfg.Bucket, object, destination); err != nil {
			return fmt.Errorf("asset %s: %w", asset, err)
		}
	}

	proDestination := filepath.Join(libraryRoot, p.ProFile())
	if err := pull(ctx, bucket, s.cfg.Bucket, path.Join(prefix, p.ProFile()), proDestination); err != nil {
		return fmt.Errorf("pro file: %w", err)
	}

	s.lastSynced[manifestObject] = generation
	fmt.Printf("synced presentation %q: %d assets -> %s, pro file -> %s\n",
		p.Name, len(p.Assets), mediaAssetsRoot, proDestination)

	return nil
}

// fetchPresentation downloads and parses the presentation.json object,
// returning the parsed presentation and the object's GCS generation.
func fetchPresentation(ctx context.Context, bucket *storage.BucketHandle, object string) (*presentation.Presentation, int64, error) {
	reader, err := bucket.Object(object).NewReader(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("open manifest %s: %w", object, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, fmt.Errorf("read manifest %s: %w", object, err)
	}

	p, err := presentation.Parse(data)
	if err != nil {
		return nil, 0, err
	}
	return p, reader.Attrs.Generation, nil
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

	// Any path that does not reach a successful rename must leave nothing
	// behind. The .part staging exists so ProPresenter never sees a
	// half-written asset under its real name; that guarantee is only half kept
	// if the truncated file survives under a name one suffix away from the one
	// the show references. It also accumulates, and this client prunes and
	// reports on the contents of the media directory.
	renamed := false
	defer func() {
		tmpFile.Close() // no-op once closed below; the error is not actionable
		if !renamed {
			os.Remove(partial)
		}
	}()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(partial, destination); err != nil {
		return fmt.Errorf("rename to destination: %w", err)
	}

	renamed = true
	return nil
}
