package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// UnpackResult contains paths to the unpacked files.
type UnpackResult struct {
	ProFile    string
	AssetsDirs []string
	SectionSlug string
}

// UnpackToDestinations unpacks a .probundle file to two destinations:
// - .pro file goes to librariesRoot/<section-slug>.pro
// - assets/* go to mediaAssetsRoot/<section-slug>/*
func UnpackToDestinations(bundlePath, librariesRoot, mediaAssetsRoot string) (*UnpackResult, error) {
	logger := slog.Default()

	// Extract section slug from filename (e.g., "Message.probundle" -> "Message")
	bundleFilename := filepath.Base(bundlePath)
	sectionSlug := strings.TrimSuffix(bundleFilename, ".probundle")
	if sectionSlug == bundleFilename {
		return nil, fmt.Errorf("invalid probundle filename: %s", bundleFilename)
	}

	result := &UnpackResult{
		SectionSlug: sectionSlug,
	}

	// Idempotent: delete existing section folder in Media/Assets
	sectionMediaDir := filepath.Join(mediaAssetsRoot, sectionSlug)
	if err := os.RemoveAll(sectionMediaDir); err != nil {
		return nil, fmt.Errorf("remove existing section directory: %w", err)
	}

	// Create directories
	if err := os.MkdirAll(mediaAssetsRoot, 0755); err != nil {
		return nil, fmt.Errorf("create media assets root: %w", err)
	}
	if err := os.MkdirAll(librariesRoot, 0755); err != nil {
		return nil, fmt.Errorf("create libraries root: %w", err)
	}
	if err := os.MkdirAll(sectionMediaDir, 0755); err != nil {
		return nil, fmt.Errorf("create section media directory: %w", err)
	}

	// Read the bundle file
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("read bundle: %w", err)
	}

	// Find the ZIP file signature (PK\x03\x04) since probundles have a byte offset
	zipSig := []byte{0x50, 0x4b, 0x03, 0x04}
	offset := bytes.Index(data, zipSig)
	if offset == -1 {
		return nil, fmt.Errorf("invalid probundle: ZIP signature not found")
	}

	// Open the ZIP reader from the offset
	zipReader, err := zip.NewReader(bytes.NewReader(data[offset:]), int64(len(data)-offset))
	if err != nil {
		return nil, fmt.Errorf("open bundle: %w", err)
	}

	foundPro := false
	for _, zf := range zipReader.File {
		if strings.HasSuffix(zf.Name, ".pro") {
			// Pro file
			if foundPro {
				return nil, fmt.Errorf("multiple .pro files found in bundle")
			}
			if err := unpackProFile(zf, sectionSlug, librariesRoot, result); err != nil {
				return nil, err
			}
			foundPro = true
		} else {
			// All other files go to media assets
			if err := unpackAssetFile(zf, sectionMediaDir, result); err != nil {
				return nil, err
			}
		}
	}

	if !foundPro {
		return nil, fmt.Errorf("no .pro file found in bundle")
	}

	logger.Info("unpacked bundle", "section", sectionSlug, "proFile", result.ProFile, "assetsDirs", result.AssetsDirs)
	return result, nil
}

func unpackProFile(zf *zip.File, sectionSlug, librariesRoot string, result *UnpackResult) error {
	src, err := zf.Open()
	if err != nil {
		return fmt.Errorf("open pro file in bundle: %w", err)
	}
	defer src.Close()

	// Write to Libraries/<section-slug>.pro
	destPath := filepath.Join(librariesRoot, sectionSlug+".pro")
	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create pro file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("copy pro file: %w", err)
	}

	result.ProFile = destPath
	return nil
}

func unpackAssetFile(zf *zip.File, sectionMediaDir string, result *UnpackResult) error {
	if strings.HasSuffix(zf.Name, "/") {
		return nil
	}

	// Use the file path as-is, removing "assets/" prefix if present
	relPath := strings.TrimPrefix(zf.Name, "assets/")

	destPath := filepath.Join(sectionMediaDir, relPath)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create asset directory: %w", err)
	}

	src, err := zf.Open()
	if err != nil {
		return fmt.Errorf("open asset file in bundle: %w", err)
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create asset file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("copy asset file: %w", err)
	}

	result.AssetsDirs = append(result.AssetsDirs, relPath)
	return nil
}
