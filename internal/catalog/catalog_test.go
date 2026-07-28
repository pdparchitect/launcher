package catalog

import (
	"io"
	"path"
	"strings"
	"testing"
)

const (
	pantalkGhostID = "370a2228-322d-4089-846b-62fb8c15d154"
	buzznodeID     = "4398d440-4e4f-4137-b25e-303bfeb2a276"
)

func TestLoadGhost(t *testing.T) {
	manifest, err := Load("pantalk-ghost")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if manifest.ID != pantalkGhostID ||
		manifest.Slug != "pantalk-ghost" ||
		manifest.Name != "Pantalk Ghost" {
		t.Fatalf("manifest identity = %#v", manifest)
	}
	if manifest.Image != "ghcr.io/pantalk/ghost:0.0.10" {
		t.Fatalf("Image = %q", manifest.Image)
	}
	if manifest.Viewer != "kasmvnc" {
		t.Fatalf("Viewer = %q", manifest.Viewer)
	}
	if manifest.ContainerPort != 6901 || manifest.Memory != "4g" {
		t.Fatalf("manifest resources = %#v", manifest)
	}
	if len(manifest.Mounts) != 6 {
		t.Fatalf("len(Mounts) = %d", len(manifest.Mounts))
	}
	if !strings.Contains(manifest.Description, "desktop") {
		t.Fatalf("Description = %q", manifest.Description)
	}
	if len(manifest.Tags) < 2 {
		t.Fatalf("Tags = %#v", manifest.Tags)
	}
	if manifest.Media.Icon == "" ||
		manifest.Media.Cover == "" ||
		manifest.Media.Icon != "pantalk-ghost/icon.svg" ||
		manifest.Media.Cover != "pantalk-ghost/screenshot.png" ||
		len(manifest.Media.Screenshots) != 1 {
		t.Fatalf("Media = %#v", manifest.Media)
	}
}

func TestLoadBuzznode(t *testing.T) {
	manifest, err := Load("buzznode")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if manifest.Name != "Buzznode" ||
		manifest.ID != buzznodeID ||
		manifest.Slug != "buzznode" ||
		manifest.Image != "ghcr.io/pdparchitect/buzznode:latest" {
		t.Fatalf("manifest identity = %#v", manifest)
	}
	if manifest.Viewer != "kasmvnc" {
		t.Fatalf("Viewer = %q", manifest.Viewer)
	}
	if manifest.ContainerPort != 6901 ||
		manifest.ResolutionEnvironment != "BUZZNODE_RESOLUTION" {
		t.Fatalf("manifest desktop = %#v", manifest)
	}
	if len(manifest.Mounts) != 6 ||
		manifest.Media.Icon != "buzznode/icon.png" ||
		manifest.Media.Cover != "buzznode/screenshot.png" ||
		len(manifest.Media.Screenshots) != 1 {
		t.Fatalf("manifest persistence/media = %#v", manifest)
	}
}

func TestAllCatalogueMediaAssetsAreEmbedded(t *testing.T) {
	manifests, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	assets := Assets()
	for _, manifest := range manifests {
		paths := []string{manifest.Media.Icon, manifest.Media.Cover}
		for _, screenshot := range manifest.Media.Screenshots {
			paths = append(paths, screenshot.Source)
		}
		for _, name := range paths {
			file, openErr := assets.Open(name)
			if openErr != nil {
				t.Fatalf("open asset %q: %v", name, openErr)
			}
			header := make([]byte, 8)
			count, readErr := io.ReadFull(file, header)
			if readErr != nil && readErr != io.ErrUnexpectedEOF {
				_ = file.Close()
				t.Fatalf("read asset %q: %v", name, readErr)
			}
			_ = file.Close()
			switch path.Ext(name) {
			case ".png":
				if string(header[:count]) != "\x89PNG\r\n\x1a\n" {
					t.Fatalf("asset %q is not a PNG", name)
				}
			case ".svg":
				if !strings.Contains(string(header[:count]), "<svg") {
					t.Fatalf("asset %q is not an SVG", name)
				}
			default:
				t.Fatalf("asset %q has an unsupported test format", name)
			}
		}
	}
}

func TestListContainsGhost(t *testing.T) {
	manifests, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(manifests) != 2 ||
		manifests[0].ID != buzznodeID ||
		manifests[1].ID != pantalkGhostID {
		t.Fatalf("List() = %#v", manifests)
	}
}

func TestValidateRejectsUnsafeMountName(t *testing.T) {
	manifest := Manifest{
		ID:                    pantalkGhostID,
		Slug:                  "unsafe",
		Name:                  "Unsafe",
		Publisher:             "Test",
		Image:                 "example.invalid/test",
		ContainerPort:         6901,
		SharedMemory:          "1g",
		ResolutionEnvironment: "GHOST_RESOLUTION",
		Resolution:            "1920x1080",
		Environment:           map[string]string{},
		Mounts:                []Mount{{Name: "../outside", Target: "/workspace"}},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsafe mount error")
	}
}

func TestValidateRejectsUnsafeMediaPath(t *testing.T) {
	manifest := Manifest{
		ID:                    pantalkGhostID,
		Slug:                  "unsafe",
		Name:                  "Unsafe",
		Publisher:             "Test",
		Description:           "An unsafe test entry.",
		Tags:                  []string{"TEST"},
		Image:                 "example.invalid/test",
		ContainerPort:         6901,
		SharedMemory:          "1g",
		ResolutionEnvironment: "GHOST_RESOLUTION",
		Resolution:            "1920x1080",
		Environment:           map[string]string{},
		Media: Media{
			Icon:  "../outside.png",
			Cover: "unsafe/cover.png",
			Screenshots: []Screenshot{{
				Source: "unsafe/screenshot.png",
				Alt:    "Unsafe screenshot",
			}},
		},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsafe media error")
	}
}
