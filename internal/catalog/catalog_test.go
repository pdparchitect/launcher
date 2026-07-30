package catalog

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

const testApplicationID = "370a2228-322d-4089-846b-62fb8c15d154"

func TestLoadApplicationBundleDerivesDigestImageAndScopesAssets(t *testing.T) {
	bundle, err := loadApplicationBundle(
		testApplicationBundle(t, "ghost", testApplicationID),
		"ghcr.io/example/ghost",
		"sha256:"+strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("loadApplicationBundle() error = %v", err)
	}
	if bundle.Manifest.Image !=
		"ghcr.io/example/ghost@sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("application identity = %#v", bundle)
	}
	if bundle.Manifest.Media.Icon != "ghost/icon.svg" ||
		bundle.Manifest.Media.Cover != "ghost/screenshot.png" ||
		bundle.Manifest.Media.Screenshots[0].Source !=
			"ghost/screenshot.png" {
		t.Fatalf("scoped media = %#v", bundle.Manifest.Media)
	}
	if string(bundle.Assets["ghost/icon.svg"]) != "<svg/>" {
		t.Fatalf("assets = %#v", bundle.Assets)
	}
}

func TestLoadApplicationBundleAcceptsUnknownFields(t *testing.T) {
	document := strings.Replace(
		testApplicationJSON("ghost", testApplicationID),
		`"schemaVersion": 1,`,
		`"schemaVersion": 1,
		"version": "1.2.3",
		"resolutionEnvironment": "DESKTOP_RESOLUTION",
		"resolution": "1920x1080",
		"futureMetadata": {"channel": "stable"},`,
		1,
	)
	_, err := loadApplicationBundle(
		testBundle(t, map[string]string{
			"application.json": document,
			"icon.svg":         "<svg/>",
			"screenshot.png":   "\x89PNG\r\n\x1a\n",
		}),
		"ghcr.io/example/ghost",
		"sha256:"+strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("loadApplicationBundle() unknown fields error = %v", err)
	}
}

func TestLoadApplicationBundleRequiresImageSubject(t *testing.T) {
	_, err := loadApplicationBundle(
		testApplicationBundle(t, "ghost", testApplicationID),
		"ghcr.io/example/ghost",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("loadApplicationBundle() error = %v, want subject error", err)
	}
}

func TestLoadApplicationBundleRejectsMissingAsset(t *testing.T) {
	data := testBundle(t, map[string]string{
		"application.json": testApplicationJSON("ghost", testApplicationID),
		"icon.svg":         "<svg/>",
	})
	_, err := loadApplicationBundle(
		data,
		"ghcr.io/example/ghost",
		"sha256:"+strings.Repeat("a", 64),
	)
	if err == nil || !strings.Contains(err.Error(), "screenshot.png") {
		t.Fatalf("loadApplicationBundle() error = %v, want asset error", err)
	}
}

func TestManifestsFromBundlesRejectsIdentityCollision(t *testing.T) {
	first, err := loadApplicationBundle(
		testApplicationBundle(t, "first", testApplicationID),
		"ghcr.io/example/first",
		"sha256:"+strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("load first bundle: %v", err)
	}
	second, err := loadApplicationBundle(
		testApplicationBundle(t, "second", testApplicationID),
		"ghcr.io/example/second",
		"sha256:"+strings.Repeat("b", 64),
	)
	if err != nil {
		t.Fatalf("load second bundle: %v", err)
	}
	_, _, err = manifestsFromBundles(map[string]applicationBundle{
		"first":  first,
		"second": second,
	})
	if err == nil || !strings.Contains(err.Error(), "published by both") {
		t.Fatalf("manifestsFromBundles() error = %v, want collision", err)
	}
}

func TestParseFeedRejectsDuplicateApplication(t *testing.T) {
	_, err := parseFeed([]byte(`{
		"schemaVersion": 1,
		"publisher": "Example",
		"applications": ["example.test/app:stable", "example.test/app:stable"]
	}`))
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("parseFeed() error = %v, want duplicate error", err)
	}
}

func TestValidateRejectsFloatingLatestImage(t *testing.T) {
	manifest := validManifest()
	manifest.Image = "ghcr.io/example/ghost:latest"
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want immutable digest error")
	}
}

func TestValidateRejectsUnsafeMountName(t *testing.T) {
	manifest := validManifest()
	manifest.Mounts = []Mount{{Name: "../outside", Target: "/workspace"}}
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsafe mount error")
	}
}

func validManifest() Manifest {
	return Manifest{
		ID:            testApplicationID,
		Slug:          "ghost",
		Name:          "Ghost",
		Publisher:     "Example",
		Description:   "A test application.",
		Tags:          []string{"TEST"},
		Media:         Media{Icon: "icon.svg", Cover: "screenshot.png", Screenshots: []Screenshot{{Source: "screenshot.png", Alt: "Ghost screen"}}},
		Image:         "ghcr.io/example/ghost@sha256:" + strings.Repeat("a", 64),
		Viewer:        "kasmvnc",
		ContainerPort: 6901,
		SharedMemory:  "1g",
		Environment:   map[string]string{},
		Mounts:        []Mount{{Name: "workspace", Target: "/workspace"}},
	}
}

func testApplicationBundle(t *testing.T, slug string, id string) []byte {
	t.Helper()
	return testBundle(t, map[string]string{
		"application.json": testApplicationJSON(slug, id),
		"icon.svg":         "<svg/>",
		"screenshot.png":   "\x89PNG\r\n\x1a\n",
	})
}

func testApplicationJSON(slug string, id string) string {
	return `{
		"schemaVersion": 1,
		"id": "` + id + `",
		"slug": "` + slug + `",
		"name": "` + slug + `",
		"publisher": "Example",
		"description": "A test application.",
		"tags": ["TEST"],
		"media": {
			"icon": "icon.svg",
			"cover": "screenshot.png",
			"screenshots": [{"source": "screenshot.png", "alt": "Screen"}]
		},
		"viewer": "kasmvnc",
		"containerPort": 6901,
		"sharedMemory": "1g",
		"environment": {},
		"mounts": [{"name": "workspace", "target": "/workspace"}]
	}`
}

func testBundle(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		target, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create bundle path %q: %v", name, err)
		}
		if _, err := io.WriteString(target, content); err != nil {
			t.Fatalf("write bundle path %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close bundle: %v", err)
	}
	return buffer.Bytes()
}
