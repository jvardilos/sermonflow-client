package bundle

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Helper to create a test bundle file
func createTestBundle(t *testing.T, bundlePath string) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Add a .pro file
	fw, err := zw.Create("Message.pro")
	if err != nil {
		t.Fatalf("create pro file in zip: %v", err)
	}
	fw.Write([]byte("dummy pro content"))

	// Add assets
	fw, err = zw.Create("assets/bg.png")
	if err != nil {
		t.Fatalf("create asset file in zip: %v", err)
	}
	fw.Write([]byte("dummy image data"))

	fw, err = zw.Create("assets/slide-04.png")
	if err != nil {
		t.Fatalf("create asset file in zip: %v", err)
	}
	fw.Write([]byte("dummy image data"))

	zw.Close()

	if err := os.WriteFile(bundlePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write bundle file: %v", err)
	}
}

func TestUnpackToDestinations(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test bundle
	bundlePath := filepath.Join(tmpDir, "Message.probundle")
	createTestBundle(t, bundlePath)

	librariesRoot := filepath.Join(tmpDir, "Libraries")
	mediaAssetsRoot := filepath.Join(tmpDir, "Media", "Assets")

	result, err := UnpackToDestinations(bundlePath, librariesRoot, mediaAssetsRoot)
	if err != nil {
		t.Fatalf("unpack failed: %v", err)
	}

	// Verify section slug
	if result.SectionSlug != "Message" {
		t.Errorf("section slug = %q, want %q", result.SectionSlug, "Message")
	}

	// Verify pro file exists
	expectedProFile := filepath.Join(librariesRoot, "Message.pro")
	if result.ProFile != expectedProFile {
		t.Errorf("pro file path = %q, want %q", result.ProFile, expectedProFile)
	}
	if _, err := os.Stat(expectedProFile); os.IsNotExist(err) {
		t.Errorf("pro file does not exist: %s", expectedProFile)
	}

	// Verify asset files exist
	expectedAssets := []string{"bg.png", "slide-04.png"}
	for _, asset := range expectedAssets {
		assetPath := filepath.Join(mediaAssetsRoot, "Message", asset)
		if _, err := os.Stat(assetPath); os.IsNotExist(err) {
			t.Errorf("asset file does not exist: %s", assetPath)
		}
	}
}

func TestUnpackToDestinationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test bundle
	bundlePath := filepath.Join(tmpDir, "Message.probundle")
	createTestBundle(t, bundlePath)

	librariesRoot := filepath.Join(tmpDir, "Libraries")
	mediaAssetsRoot := filepath.Join(tmpDir, "Media", "Assets")

	// Unpack twice
	if _, err := UnpackToDestinations(bundlePath, librariesRoot, mediaAssetsRoot); err != nil {
		t.Fatalf("first unpack failed: %v", err)
	}

	if _, err := UnpackToDestinations(bundlePath, librariesRoot, mediaAssetsRoot); err != nil {
		t.Fatalf("second unpack failed: %v", err)
	}

	// Verify files still exist
	expectedProFile := filepath.Join(librariesRoot, "Message.pro")
	if _, err := os.Stat(expectedProFile); os.IsNotExist(err) {
		t.Errorf("pro file does not exist after second unpack: %s", expectedProFile)
	}

	assetPath := filepath.Join(mediaAssetsRoot, "Message", "bg.png")
	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		t.Errorf("asset file does not exist after second unpack: %s", assetPath)
	}
}

func TestUnpackInvalidBundleName(t *testing.T) {
	tmpDir := t.TempDir()

	bundlePath := filepath.Join(tmpDir, "invalid.zip")
	if err := os.WriteFile(bundlePath, []byte("invalid"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	librariesRoot := filepath.Join(tmpDir, "Libraries")
	mediaAssetsRoot := filepath.Join(tmpDir, "Media", "Assets")

	_, err := UnpackToDestinations(bundlePath, librariesRoot, mediaAssetsRoot)
	if err == nil {
		t.Fatal("expected error for invalid bundle name")
	}
}

func TestUnpackMissingProFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create bundle without .pro file
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	fw, _ := zw.Create("assets/bg.png")
	fw.Write([]byte("dummy"))
	zw.Close()

	bundlePath := filepath.Join(tmpDir, "Message.probundle")
	os.WriteFile(bundlePath, buf.Bytes(), 0644)

	librariesRoot := filepath.Join(tmpDir, "Libraries")
	mediaAssetsRoot := filepath.Join(tmpDir, "Media", "Assets")

	_, err := UnpackToDestinations(bundlePath, librariesRoot, mediaAssetsRoot)
	if err == nil || err.Error() != "no .pro file found in bundle" {
		t.Errorf("expected 'no .pro file' error, got %v", err)
	}
}
