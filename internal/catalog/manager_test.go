package catalog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerStartsWithEmbeddedCatalogue(t *testing.T) {
	manager, err := NewManager(t.TempDir(), ManagerOptions{})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	manifests := manager.List()
	if manager.Version() != "0.1.1" || len(manifests) != 6 {
		t.Fatalf(
			"embedded catalogue = version %q, %d entries",
			manager.Version(),
			len(manifests),
		)
	}
	if _, err := manager.Open("openclaw/icon.svg"); err != nil {
		t.Fatalf("Open() embedded asset error = %v", err)
	}
	if manager.refreshInterval != 30*time.Minute {
		t.Fatalf(
			"default refresh interval = %v, want %v",
			manager.refreshInterval,
			30*time.Minute,
		)
	}
}

func TestManagerRefreshesAndRestoresCachedRelease(t *testing.T) {
	bundle := testBundle(t, "0.1.2")
	digest := sha256.Sum256(bundle)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		switch request.URL.Path {
		case "/releases":
			_ = json.NewEncoder(response).Encode([]map[string]any{{
				"tag_name":     "catalogue-v0.1.2",
				"draft":        false,
				"prerelease":   false,
				"published_at": "2026-07-30T12:00:00Z",
				"assets": []map[string]any{{
					"name":                 bundleAssetName,
					"browser_download_url": serverURL(request) + "/bundle",
					"digest":               "sha256:" + hex.EncodeToString(digest[:]),
					"size":                 len(bundle),
				}},
			}})
		case "/bundle":
			response.Header().Set("Content-Type", "application/zip")
			_, _ = response.Write(bundle)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	manager, err := NewManager(root, ManagerOptions{
		Client:      server.Client(),
		ReleasesURL: server.URL + "/releases",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	changed, err := manager.Refresh(t.Context(), true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !changed || manager.Version() != "0.1.2" {
		t.Fatalf(
			"Refresh() = %v, version %q",
			changed,
			manager.Version(),
		)
	}
	if _, err := manager.Open("buzzbox/screenshot.png"); err != nil {
		t.Fatalf("Open() refreshed asset error = %v", err)
	}

	restored, err := NewManager(root, ManagerOptions{
		Client:      server.Client(),
		ReleasesURL: server.URL + "/releases",
	})
	if err != nil {
		t.Fatalf("NewManager() restored error = %v", err)
	}
	if restored.Version() != "0.1.2" || len(restored.List()) != 6 {
		t.Fatalf(
			"restored catalogue = version %q, %d entries",
			restored.Version(),
			len(restored.List()),
		)
	}
	if requests.Load() != 2 {
		t.Fatalf("HTTP requests = %d, want release and bundle", requests.Load())
	}
}

func TestManagerRejectsReleaseWithWrongDigest(t *testing.T) {
	bundle := testBundle(t, "0.1.2")
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		switch request.URL.Path {
		case "/releases":
			_ = json.NewEncoder(response).Encode([]map[string]any{{
				"tag_name": "catalogue-v0.1.2",
				"assets": []map[string]any{{
					"name":                 bundleAssetName,
					"browser_download_url": serverURL(request) + "/bundle",
					"digest":               "sha256:" + strings.Repeat("0", 64),
					"size":                 len(bundle),
				}},
			}})
		case "/bundle":
			_, _ = response.Write(bundle)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	manager, err := NewManager(t.TempDir(), ManagerOptions{
		Client:      server.Client(),
		ReleasesURL: server.URL + "/releases",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	changed, err := manager.Refresh(t.Context(), true)
	if err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("Refresh() = %v, %v, want digest error", changed, err)
	}
	if changed || manager.Version() != "0.1.1" {
		t.Fatalf(
			"catalogue changed after rejected release: %v, %q",
			changed,
			manager.Version(),
		)
	}
}

func TestManagerSkipsFreshReleaseCheck(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		requests.Add(1)
	}))
	defer server.Close()

	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	manager, err := NewManager(t.TempDir(), ManagerOptions{
		Client:          server.Client(),
		ReleasesURL:     server.URL,
		RefreshInterval: 24 * time.Hour,
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.markChecked(now)

	changed, err := manager.Refresh(t.Context(), false)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if changed || requests.Load() != 0 {
		t.Fatalf("Refresh() = %v, HTTP requests = %d", changed, requests.Load())
	}
}

func TestSelectReleaseUsesHighestStableCatalogueVersion(t *testing.T) {
	release, _, err := selectRelease([]githubRelease{
		{
			TagName: "catalogue-v0.2.0",
			Assets:  []githubAsset{{Name: bundleAssetName}},
		},
		{
			TagName: "catalogue-v1.0.0-beta.1",
			Assets:  []githubAsset{{Name: bundleAssetName}},
		},
		{
			TagName: "catalogue-v0.10.0",
			Assets:  []githubAsset{{Name: bundleAssetName}},
		},
		{
			TagName: "launcher-v9.0.0",
			Assets:  []githubAsset{{Name: bundleAssetName}},
		},
	})
	if err != nil {
		t.Fatalf("selectRelease() error = %v", err)
	}
	if release.TagName != "catalogue-v0.10.0" {
		t.Fatalf("selected release = %q", release.TagName)
	}
}

func testBundle(t *testing.T, version string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	metadata, err := writer.Create("catalogue.json")
	if err != nil {
		t.Fatalf("create catalogue metadata: %v", err)
	}
	_, _ = io.WriteString(
		metadata,
		`{"schemaVersion":1,"version":"`+version+`"}`+"\n",
	)
	err = fs.WalkDir(files, "manifests", func(
		name string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			_, createErr := writer.Create(name + "/")
			return createErr
		}
		data, readErr := fs.ReadFile(files, name)
		if readErr != nil {
			return readErr
		}
		target, createErr := writer.Create(name)
		if createErr != nil {
			return createErr
		}
		_, createErr = target.Write(data)
		return createErr
	})
	if err != nil {
		t.Fatalf("write test bundle: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close test bundle: %v", err)
	}
	return buffer.Bytes()
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}

func TestManagerRefreshHonoursCancelledContext(t *testing.T) {
	manager, err := NewManager(t.TempDir(), ManagerOptions{
		ReleasesURL: "https://example.invalid/releases",
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := manager.Refresh(ctx, true); err == nil {
		t.Fatal("Refresh() error = nil, want cancelled context")
	}
}
