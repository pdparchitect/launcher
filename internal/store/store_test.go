package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
)

func TestCreateListAndDeleteInstance(t *testing.T) {
	dataStore := New(t.TempDir())
	manifest := testManifest()
	instance := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")

	paths, err := dataStore.Create(instance, manifest)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, mount := range manifest.Mounts {
		if info, statErr := os.Stat(paths.Mounts[mount.Name]); statErr != nil ||
			!info.IsDir() {
			t.Fatalf("mount %q was not created: %v", mount.Name, statErr)
		}
	}
	info, err := os.Stat(filepath.Join(paths.Root, "instance.json"))
	if err != nil {
		t.Fatalf("Stat(instance.json) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("instance.json mode = %o", info.Mode().Perm())
	}
	found, err := dataStore.Get("ada")
	if err != nil || found.ID != instance.ID {
		t.Fatalf("Get(name) = %#v, %v", found, err)
	}
	if err := dataStore.Delete(instance.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(paths.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("instance root still exists: %v", err)
	}
}

func TestCreateDoesNotCreateHostPathForRuntimeVolume(t *testing.T) {
	dataStore := New(t.TempDir())
	manifest := testManifest()
	manifest.Mounts = append(manifest.Mounts, catalog.Mount{
		Name: "private/services", Target: "/var/lib/services",
		Storage: catalog.MountStorageVolume,
	})
	instance := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")

	paths, err := dataStore.Create(instance, manifest)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, exists := paths.Mounts["private/services"]; exists {
		t.Fatalf("Paths().Mounts contains runtime volume: %#v", paths.Mounts)
	}
	if _, err := os.Stat(filepath.Join(paths.Root, "private/services")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime volume host path exists: %v", err)
	}
}

func TestEnsurePathsCreatesNewHostMountWithoutRuntimeVolumeDirectory(t *testing.T) {
	dataStore := New(t.TempDir())
	manifest := testManifest()
	instance := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")
	if _, err := dataStore.Create(instance, manifest); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	manifest.Mounts = append(manifest.Mounts,
		catalog.Mount{Name: "new-host", Target: "/new-host"},
		catalog.Mount{
			Name: "new-volume", Target: "/new-volume",
			Storage: catalog.MountStorageVolume,
		},
	)

	paths, err := dataStore.EnsurePaths(instance.ID, manifest)

	if err != nil {
		t.Fatalf("EnsurePaths() error = %v", err)
	}
	if _, err := os.Stat(paths.Mounts["new-host"]); err != nil {
		t.Fatalf("new host mount is unavailable: %v", err)
	}
	if _, exists := paths.Mounts["new-volume"]; exists {
		t.Fatalf("runtime volume has host path: %#v", paths.Mounts)
	}
}

func TestExistingHostPathsForVolumesDetectsLegacyStorage(t *testing.T) {
	dataStore := New(t.TempDir())
	manifest := testManifest()
	instance := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")
	if _, err := dataStore.Create(instance, manifest); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	manifest.Mounts[0].Storage = catalog.MountStorageVolume

	conflicts, err := dataStore.ExistingHostPathsForVolumes(instance.ID, manifest)

	if err != nil || !slices.Equal(conflicts, []string{"workspace"}) {
		t.Fatalf("ExistingHostPathsForVolumes() = %#v, %v", conflicts, err)
	}
}

func TestCreateRejectsDuplicateName(t *testing.T) {
	dataStore := New(t.TempDir())
	if _, err := dataStore.Create(
		testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada"),
		testManifest(),
	); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	_, err := dataStore.Create(
		testInstance("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "ada"),
		testManifest(),
	)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("second Create() error = %v, want ErrDuplicateName", err)
	}
}

func TestSaveRejectsRenamingToDuplicateName(t *testing.T) {
	dataStore := New(t.TempDir())
	first := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")
	second := testInstance("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "Grace")
	if _, err := dataStore.Create(first, testManifest()); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if _, err := dataStore.Create(second, testManifest()); err != nil {
		t.Fatalf("second Create() error = %v", err)
	}

	second.Name = "ada"
	err := dataStore.Save(second)

	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("Save() error = %v, want ErrDuplicateName", err)
	}
}

func TestListSkipsInvalidAgentState(t *testing.T) {
	dataStore := New(t.TempDir())
	valid := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")
	if _, err := dataStore.Create(valid, testManifest()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	invalidID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	invalidRoot := dataStore.instanceRoot(invalidID)
	if err := os.MkdirAll(invalidRoot, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(invalidRoot, "instance.json"),
		[]byte(`{
  "id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "catalogId": "ghost",
  "name": "Broken",
  "image": "pantalk/ghost:test",
  "containerName": "launcher-ghost-bbbbbbbbbbbb",
  "desiredState": "stopped",
  "createdAt": "2026-07-27T12:00:00Z"
}
`),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	instances, issues, err := dataStore.ListWithIssues()

	if err != nil {
		t.Fatalf("ListWithIssues() error = %v", err)
	}
	if len(instances) != 1 || instances[0].ID != valid.ID {
		t.Fatalf("instances = %#v, want only valid agent", instances)
	}
	if len(issues) != 1 ||
		issues[0].ID != invalidID ||
		!strings.Contains(issues[0].Error, "resolved interface") {
		t.Fatalf("issues = %#v, want invalid agent validation issue", issues)
	}
	if _, err := dataStore.Get(invalidID); err == nil {
		t.Fatal("Get(invalid ID) error = nil, want validation error")
	}
}

func TestCreateRejectsUnsafeInstanceID(t *testing.T) {
	instance := testInstance("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada")
	instance.ID = "../outside"
	if _, err := New(t.TempDir()).Create(instance, testManifest()); err == nil {
		t.Fatal("Create() error = nil, want invalid ID error")
	}
}

func testInstance(id string, name string) domain.Instance {
	return domain.Instance{
		ID:            id,
		CatalogID:     "ghost",
		Name:          name,
		Image:         "pantalk/ghost:test",
		ContainerName: "launcher-ghost-" + id[:12],
		Interfaces: map[string]domain.Interface{
			"desktop": {Kind: "kasmweb", Port: 16902, Path: "/"},
		},
		DesiredState: domain.DesiredStopped,
		CreatedAt:    time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}

func testManifest() catalog.Manifest {
	return catalog.Manifest{
		ID:          "370a2228-322d-4089-846b-62fb8c15d154",
		Slug:        "pantalk-ghost",
		Name:        "Pantalk Ghost",
		Publisher:   "Pantalk",
		Description: "A local desktop agent.",
		Tags:        []string{"DESKTOP"},
		Media: catalog.Media{
			Icon:  "ghost/icon.png",
			Cover: "ghost/cover.png",
			Screenshots: []catalog.Screenshot{{
				Source: "ghost/screenshot.png",
				Alt:    "Ghost desktop",
			}},
		},
		Image: "pantalk/ghost:test",
		Interfaces: map[string]catalog.Interface{
			"desktop": {Kind: "kasmweb", Port: 6901, Path: "/"},
		},
		SharedMemory: "1g",
		Environment:  map[string]string{"PANTALK_AUTOSTART": "true"},
		Mounts: []catalog.Mount{
			{Name: "workspace", Target: "/workspace"},
			{Name: "private/config", Target: "/home/ghost/.config"},
		},
	}
}
