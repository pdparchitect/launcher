package imagecache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordsReferencesAndForgetsImages(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	first := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	if err := store.Record("sha256:aaa", "example/app@sha256:111", first); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := store.Record("sha256:aaa", "example/app:stable", second); err != nil {
		t.Fatalf("Record() second reference error = %v", err)
	}
	entries, err := store.Entries()
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "sha256:aaa" ||
		len(entries[0].References) != 2 || !entries[0].LastUsedAt.Equal(second) {
		t.Fatalf("Entries() = %#v", entries)
	}
	if err := store.Forget("sha256:aaa"); err != nil {
		t.Fatalf("Forget() error = %v", err)
	}
	entries, err = store.Entries()
	if err != nil || len(entries) != 0 {
		t.Fatalf("Entries() after Forget = %#v, %v", entries, err)
	}

	info, err := os.Stat(filepath.Join(root, "images", "cache.json"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode = %o, want 600", info.Mode().Perm())
	}
}

func TestStoreRejectsCorruptStateWithoutReplacingIt(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "images")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "cache.json")
	if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := New(root)

	if err := store.Record(
		"sha256:aaa",
		"example/app@sha256:111",
		time.Now(),
	); err == nil {
		t.Fatal("Record() error = nil, want corrupt-state error")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "not json\n" {
		t.Fatalf("cache after failed Record = %q, %v", data, err)
	}
}
