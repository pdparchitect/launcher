package updatecheck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerSelectsLatestStableLauncherRelease(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if request.Header.Get("Accept") != "application/vnd.github+json" ||
			request.Header.Get("User-Agent") != "pdparchitect-launcher" {
			t.Fatalf("release request headers = %#v", request.Header)
		}
		_ = json.NewEncoder(response).Encode([]map[string]any{
			{
				"tag_name": "catalogue-v9.0.0",
				"html_url": releaseURL("catalogue-v9.0.0"),
			},
			{
				"tag_name":   "v0.11.0",
				"html_url":   releaseURL("v0.11.0"),
				"prerelease": true,
			},
			{
				"tag_name": "v0.3.0",
				"html_url": releaseURL("v0.3.0"),
				"draft":    true,
			},
			{
				"tag_name": "v0.10.0",
				"html_url": releaseURL("v0.10.0"),
			},
			{
				"tag_name": "v0.9.0",
				"html_url": releaseURL("v0.9.0"),
			},
		})
	}))
	defer server.Close()

	manager := NewManager(t.TempDir(), "0.2.0", Options{
		Client:      server.Client(),
		ReleasesURL: server.URL,
	})
	available, err := manager.Refresh(t.Context(), true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	status := manager.Status()
	if !available ||
		!status.UpdateAvailable ||
		status.CurrentVersion != "0.2.0" ||
		status.LatestVersion != "0.10.0" ||
		status.ReleaseURL != releaseURL("v0.10.0") ||
		status.CheckedAt == nil ||
		status.Checking {
		t.Fatalf("status = %#v", status)
	}
	if requests.Load() != 1 {
		t.Fatalf("release requests = %d, want 1", requests.Load())
	}
}

func TestManagerCachesChecksAndUsesETag(t *testing.T) {
	var requests atomic.Int32
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		count := requests.Add(1)
		if count == 2 {
			if request.Header.Get("If-None-Match") != `"launcher-v0.3.0"` {
				t.Fatalf(
					"If-None-Match = %q",
					request.Header.Get("If-None-Match"),
				)
			}
			response.WriteHeader(http.StatusNotModified)
			return
		}
		response.Header().Set("ETag", `"launcher-v0.3.0"`)
		_ = json.NewEncoder(response).Encode([]map[string]any{{
			"tag_name": "v0.3.0",
			"html_url": releaseURL("v0.3.0"),
		}})
	}))
	defer server.Close()

	root := t.TempDir()
	manager := NewManager(root, "0.2.0", Options{
		Client:          server.Client(),
		ReleasesURL:     server.URL,
		RefreshInterval: 24 * time.Hour,
		Now:             func() time.Time { return now },
	})
	if _, err := manager.Refresh(t.Context(), false); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	restored := NewManager(root, "0.2.0", Options{
		Client:          server.Client(),
		ReleasesURL:     server.URL,
		RefreshInterval: 24 * time.Hour,
		Now:             func() time.Time { return now },
	})
	if status := restored.Status(); !status.UpdateAvailable ||
		status.LatestVersion != "0.3.0" ||
		status.CheckedAt == nil ||
		!status.CheckedAt.Equal(now) {
		t.Fatalf("restored status = %#v", status)
	}
	if _, err := restored.Refresh(t.Context(), false); err != nil {
		t.Fatalf("fresh Refresh() error = %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("startup release requests = %d, want 2", requests.Load())
	}

	now = now.Add(25 * time.Hour)
	if _, err := restored.Refresh(t.Context(), false); err != nil {
		t.Fatalf("ETag Refresh() error = %v", err)
	}
	if requests.Load() != 3 {
		t.Fatalf("ETag release requests = %d, want 3", requests.Load())
	}
	if restored.Status().CheckedAt == nil ||
		!restored.Status().CheckedAt.Equal(now) {
		t.Fatalf("checked at = %s, want %s", restored.Status().CheckedAt, now)
	}
}

func TestManagerRevalidatesRestoredCacheOnStartup(t *testing.T) {
	var requests atomic.Int32
	latest := "v0.4.0"
	now := time.Date(2026, 7, 30, 15, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		requests.Add(1)
		if latest == "v0.4.1" &&
			request.Header.Get("If-None-Match") != `"launcher-v0.4.0"` {
			t.Fatalf(
				"If-None-Match = %q",
				request.Header.Get("If-None-Match"),
			)
		}
		response.Header().Set("ETag", `"launcher-`+latest+`"`)
		_ = json.NewEncoder(response).Encode([]map[string]any{{
			"tag_name": latest,
			"html_url": releaseURL(latest),
		}})
	}))
	defer server.Close()

	root := t.TempDir()
	manager := NewManager(root, "0.4.0", Options{
		Client:          server.Client(),
		ReleasesURL:     server.URL,
		RefreshInterval: 24 * time.Hour,
		Now:             func() time.Time { return now },
	})
	if available, err := manager.Refresh(t.Context(), false); err != nil ||
		available {
		t.Fatalf("initial Refresh() = %v, %v", available, err)
	}

	latest = "v0.4.1"
	now = now.Add(time.Hour)
	restored := NewManager(root, "0.4.0", Options{
		Client:          server.Client(),
		ReleasesURL:     server.URL,
		RefreshInterval: 24 * time.Hour,
		Now:             func() time.Time { return now },
	})
	available, err := restored.Refresh(t.Context(), false)
	if err != nil {
		t.Fatalf("startup Refresh() error = %v", err)
	}
	if !available || !restored.Status().UpdateAvailable ||
		restored.Status().LatestVersion != "0.4.1" {
		t.Fatalf("startup status = %#v", restored.Status())
	}
	if requests.Load() != 2 {
		t.Fatalf("release requests = %d, want 2", requests.Load())
	}
}

func TestManagerSkipsDevelopmentBuilds(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		requests.Add(1)
	}))
	defer server.Close()

	manager := NewManager(t.TempDir(), "dev", Options{
		Client:      server.Client(),
		ReleasesURL: server.URL,
	})
	available, err := manager.Refresh(t.Context(), true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if available || manager.Status().UpdateAvailable || requests.Load() != 0 {
		t.Fatalf(
			"Refresh() = %v, status %#v, requests %d",
			available,
			manager.Status(),
			requests.Load(),
		)
	}
}

func releaseURL(tag string) string {
	return "https://github.com/pdparchitect/launcher/releases/tag/" + tag
}
