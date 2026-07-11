// Package presentation parses the presentation.json that the pipeline writes
// to GCS alongside the decoded .pro file and media assets. It is a readable
// JSON rendering of the .pro file; the client uses it as a manifest to learn
// the presentation name and which media files the cues reference, so it can
// download individual files instead of unpacking a .probundle.
package presentation

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// Presentation is what the client needs from presentation.json: the name
// (which also names the .pro file) and the media assets the cues reference.
type Presentation struct {
	Name   string
	UUID   string
	Assets []string
}

// ProFile returns the name of the .pro object that accompanies the JSON.
func (p *Presentation) ProFile() string {
	return p.Name + ".pro"
}

// Parse decodes presentation.json and collects every referenced asset path.
func Parse(data []byte) (*Presentation, error) {
	var raw struct {
		Name string `json:"name"`
		UUID struct {
			String string `json:"string"`
		} `json:"uuid"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode presentation.json: %w", err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("presentation.json missing name")
	}

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode presentation.json: %w", err)
	}

	// Media references appear in cues as url objects like:
	//   "url": {"absoluteString": "file://...", "local": {"root": "ROOT_SHOW", "path": "11.14.tif"}}
	// Walk the whole document so we don't depend on the exact cue structure.
	seen := map[string]bool{}
	collectAssetPaths(doc, seen)

	p := &Presentation{
		Name: raw.Name,
		UUID: raw.UUID.String,
	}
	for assetPath := range seen {
		if err := validatePath(assetPath); err != nil {
			return nil, fmt.Errorf("asset %q: %w", assetPath, err)
		}
		p.Assets = append(p.Assets, assetPath)
	}
	sort.Strings(p.Assets)

	return p, nil
}

// collectAssetPaths walks the decoded JSON looking for url.local.path values.
func collectAssetPaths(node any, seen map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		if url, ok := v["url"].(map[string]any); ok {
			if local, ok := url["local"].(map[string]any); ok {
				if p, ok := local["path"].(string); ok && p != "" {
					seen[p] = true
				}
			}
		}
		for _, child := range v {
			collectAssetPaths(child, seen)
		}
	case []any:
		for _, child := range v {
			collectAssetPaths(child, seen)
		}
	}
}

// validatePath rejects paths that could escape the destination directory.
func validatePath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if path.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %s", p)
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path escapes destination: %s", p)
	}
	return nil
}
