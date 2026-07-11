package presentation

import (
	"testing"
)

func TestParseValid(t *testing.T) {
	// Mirrors the shape of the real presentation.json: media references are
	// nested in cues under actions[].media.element.url.
	data := []byte(`{
		"name": "Presentation",
		"uuid": {"string": "8EA6AD3F-A21D-4D1F-B40F-F849C7955003"},
		"cues": [
			{
				"actions": [
					{"label": {"text": "slide 1"}},
					{
						"media": {
							"element": {
								"url": {
									"absoluteString": "file:///Library/.../Media/Assets/11.14.tif",
									"local": {"root": "ROOT_SHOW", "path": "11.14.tif"}
								}
							}
						}
					}
				]
			},
			{
				"actions": [
					{
						"media": {
							"element": {
								"url": {
									"absoluteString": "file:///Library/.../Media/Assets/Boss.mov",
									"local": {"root": "ROOT_SHOW", "path": "Boss.mov"}
								}
							}
						}
					}
				]
			},
			{
				"actions": [
					{
						"media": {
							"element": {
								"url": {
									"absoluteString": "file:///Library/.../Media/Assets/Boss.mov",
									"local": {"root": "ROOT_SHOW", "path": "Boss.mov"}
								}
							}
						}
					}
				]
			}
		]
	}`)

	p, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name != "Presentation" {
		t.Errorf("Name = %q, want %q", p.Name, "Presentation")
	}
	if p.ProFile() != "Presentation.pro" {
		t.Errorf("ProFile() = %q, want %q", p.ProFile(), "Presentation.pro")
	}
	if p.UUID != "8EA6AD3F-A21D-4D1F-B40F-F849C7955003" {
		t.Errorf("UUID = %q", p.UUID)
	}

	// Duplicate references collapse to one entry, sorted.
	want := []string{"11.14.tif", "Boss.mov"}
	if len(p.Assets) != len(want) {
		t.Fatalf("Assets = %v, want %v", p.Assets, want)
	}
	for i := range want {
		if p.Assets[i] != want[i] {
			t.Errorf("Assets[%d] = %q, want %q", i, p.Assets[i], want[i])
		}
	}
}

func TestParseNoAssets(t *testing.T) {
	p, err := Parse([]byte(`{"name": "TextOnly", "cues": []}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Assets) != 0 {
		t.Errorf("Assets = %v, want none", p.Assets)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"invalid json", `{not json`},
		{"missing name", `{"cues": []}`},
		{
			"traversal asset path",
			`{"name": "X", "cues": [{"actions": [{"media": {"element": {"url": {"local": {"path": "../../escape.mov"}}}}}]}]}`,
		},
		{
			"absolute asset path",
			`{"name": "X", "cues": [{"actions": [{"media": {"element": {"url": {"local": {"path": "/etc/passwd"}}}}}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.json)); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
