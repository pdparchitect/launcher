package catalog

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testFeedReference = "ghcr.io/example/feed:stable"
	testAppReference  = "ghcr.io/example/ghost:launcher-stable"
)

type fakeResolver struct {
	mutex     sync.Mutex
	artifacts map[string]Artifact
	errors    map[string]error
	requests  int
}

func (resolver *fakeResolver) Fetch(
	_ context.Context,
	reference string,
) (Artifact, error) {
	resolver.mutex.Lock()
	defer resolver.mutex.Unlock()
	resolver.requests++
	if err := resolver.errors[reference]; err != nil {
		return Artifact{}, err
	}
	artifact, exists := resolver.artifacts[reference]
	if !exists {
		return Artifact{}, fmt.Errorf("no artifact for %s", reference)
	}
	return artifact, nil
}

func TestManagerStartsEmptyThenRefreshesAndRestoresCache(t *testing.T) {
	resolver := testResolver(t, "ghost", testApplicationID)
	root := t.TempDir()
	manager, err := NewManager(root, ManagerOptions{
		Resolver:   resolver,
		SourceData: testSources(),
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("initial applications = %#v, want empty", manager.List())
	}
	changed, err := manager.Refresh(t.Context(), true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !changed || len(manager.List()) != 1 {
		t.Fatalf("Refresh() = %v, applications = %#v", changed, manager.List())
	}
	manifest := manager.List()[0]
	if manifest.Image !=
		"ghcr.io/example/ghost@sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("image = %q", manifest.Image)
	}
	if _, err := manager.Open("ghost/icon.svg"); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	restored, err := NewManager(root, ManagerOptions{
		Resolver:   &fakeResolver{},
		SourceData: testSources(),
	})
	if err != nil {
		t.Fatalf("NewManager() restored error = %v", err)
	}
	if len(restored.List()) != 1 ||
		restored.List()[0].Image != manifest.Image {
		t.Fatalf("restored applications = %#v", restored.List())
	}
}

// The catalogue is presented in the order publishers declare it, so neither a
// refresh nor a restore from cache may reorder it — including alphabetically,
// which these deliberately reversed names would otherwise hide.
func TestManagerKeepsDeclaredApplicationOrder(t *testing.T) {
	zebraReference := "ghcr.io/example/zebra:launcher-stable"
	alphaReference := "ghcr.io/example/alpha:launcher-stable"
	resolver := &fakeResolver{
		artifacts: map[string]Artifact{
			testFeedReference: feedArtifact(zebraReference, alphaReference),
			zebraReference: applicationArtifact(
				t,
				zebraReference,
				"zebra",
				testApplicationID,
				"a",
			),
			alphaReference: applicationArtifact(
				t,
				alphaReference,
				"alpha",
				"2784cf32-591a-4ba7-9c26-64ce6deeba55",
				"b",
			),
		},
		errors: make(map[string]error),
	}
	root := t.TempDir()
	manager, err := NewManager(root, ManagerOptions{
		Resolver:   resolver,
		SourceData: testSources(),
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Refresh(t.Context(), true); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	applications := manager.List()
	if len(applications) != 2 ||
		applications[0].Slug != "zebra" ||
		applications[1].Slug != "alpha" {
		t.Fatalf("Refresh() applications = %#v", applications)
	}

	restored, err := NewManager(root, ManagerOptions{
		Resolver:   &fakeResolver{},
		SourceData: testSources(),
	})
	if err != nil {
		t.Fatalf("NewManager() restored error = %v", err)
	}

	cached := restored.List()
	if len(cached) != 2 ||
		cached[0].Slug != "zebra" ||
		cached[1].Slug != "alpha" {
		t.Fatalf("restored applications = %#v", cached)
	}
}

func TestManagerUpdatesOtherApplicationsWhileOneUsesCache(t *testing.T) {
	resolver := testResolver(t, "ghost", testApplicationID)
	manager, err := NewManager(t.TempDir(), ManagerOptions{
		Resolver:   resolver,
		SourceData: testSources(),
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Refresh(t.Context(), true); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}

	secondReference := "ghcr.io/example/second:launcher-stable"
	resolver.artifacts[testFeedReference] = feedArtifact(
		testAppReference,
		secondReference,
	)
	resolver.artifacts[secondReference] = applicationArtifact(
		t,
		secondReference,
		"second",
		"2784cf32-591a-4ba7-9c26-64ce6deeba55",
		"b",
	)
	resolver.errors[testAppReference] = fmt.Errorf("temporary registry failure")

	changed, err := manager.Refresh(t.Context(), true)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !changed || len(manager.List()) != 2 {
		t.Fatalf("Refresh() = %v, applications = %#v", changed, manager.List())
	}
	if len(manager.Warnings()) != 1 ||
		!strings.Contains(manager.Warnings()[0], "cached copy") {
		t.Fatalf("Warnings() = %#v", manager.Warnings())
	}
}

func TestManagerRejectsCollisionWithoutReplacingSnapshot(t *testing.T) {
	resolver := testResolver(t, "ghost", testApplicationID)
	manager, err := NewManager(t.TempDir(), ManagerOptions{
		Resolver:   resolver,
		SourceData: testSources(),
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Refresh(t.Context(), true); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}

	secondReference := "ghcr.io/example/collision:launcher-stable"
	resolver.artifacts[testFeedReference] = feedArtifact(
		testAppReference,
		secondReference,
	)
	resolver.artifacts[secondReference] = applicationArtifact(
		t,
		secondReference,
		"collision",
		testApplicationID,
		"c",
	)
	changed, err := manager.Refresh(t.Context(), true)
	if err == nil || !strings.Contains(err.Error(), "published by both") {
		t.Fatalf("Refresh() = %v, %v, want collision", changed, err)
	}
	if changed || len(manager.List()) != 1 ||
		manager.List()[0].Slug != "ghost" {
		t.Fatalf("snapshot changed after collision: %#v", manager.List())
	}
}

func TestManagerSkipsRefreshInsideInterval(t *testing.T) {
	resolver := testResolver(t, "ghost", testApplicationID)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	manager, err := NewManager(t.TempDir(), ManagerOptions{
		Resolver:        resolver,
		SourceData:      testSources(),
		RefreshInterval: time.Hour,
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Refresh(t.Context(), false); err != nil {
		t.Fatalf("first Refresh() error = %v", err)
	}
	requests := resolver.requests
	changed, err := manager.Refresh(t.Context(), false)
	if err != nil {
		t.Fatalf("second Refresh() error = %v", err)
	}
	if changed || resolver.requests != requests {
		t.Fatalf(
			"second Refresh() = %v, requests = %d, want %d",
			changed,
			resolver.requests,
			requests,
		)
	}
}

func TestManagerRefreshHonoursCancelledContext(t *testing.T) {
	manager, err := NewManager(t.TempDir(), ManagerOptions{
		Resolver:   &fakeResolver{},
		SourceData: testSources(),
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

func testResolver(t *testing.T, slug string, id string) *fakeResolver {
	t.Helper()
	return &fakeResolver{
		artifacts: map[string]Artifact{
			testFeedReference: feedArtifact(testAppReference),
			testAppReference: applicationArtifact(
				t,
				testAppReference,
				slug,
				id,
				"a",
			),
		},
		errors: make(map[string]error),
	}
}

func feedArtifact(references ...string) Artifact {
	content := []byte(`{
		"schemaVersion": 1,
		"publisher": "Example",
		"applications": ["` + strings.Join(references, `","`) + `"]
	}`)
	return Artifact{
		Reference:      testFeedReference,
		ManifestDigest: digestFor(content),
		ArtifactType:   FeedArtifactType,
		LayerType:      FeedDocumentType,
		Content:        content,
	}
}

func applicationArtifact(
	t *testing.T,
	reference string,
	slug string,
	id string,
	digestCharacter string,
) Artifact {
	t.Helper()
	content := testApplicationBundle(t, slug, id)
	return Artifact{
		Reference:      reference,
		ManifestDigest: digestFor(content),
		ArtifactType:   ApplicationArtifactType,
		LayerType:      ApplicationBundleType,
		Repository:     strings.TrimSuffix(reference, ":launcher-stable"),
		SubjectDigest:  "sha256:" + strings.Repeat(digestCharacter, 64),
		Content:        content,
	}
}

func digestFor(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}

func testSources() []byte {
	return []byte(`{
		"schemaVersion": 1,
		"feeds": ["` + testFeedReference + `"]
	}`)
}
