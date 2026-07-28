package store

import (
	"errors"
	"os"
	"path/filepath"
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
		Port:          16902,
		DesiredState:  domain.DesiredStopped,
		CreatedAt:     time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
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
		Image:                 "pantalk/ghost:test",
		Viewer:                "kasmvnc",
		ContainerPort:         6901,
		SharedMemory:          "1g",
		ResolutionEnvironment: "GHOST_RESOLUTION",
		Resolution:            "1920x1080",
		Environment:           map[string]string{"PANTALK_AUTOSTART": "true"},
		Mounts: []catalog.Mount{
			{Name: "workspace", Target: "/workspace"},
			{Name: "private/config", Target: "/home/ghost/.config"},
		},
	}
}
