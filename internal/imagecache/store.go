package imagecache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const schemaVersion = 1

type Entry struct {
	ID         string    `json:"id"`
	References []string  `json:"references"`
	LastUsedAt time.Time `json:"lastUsedAt"`
}

type state struct {
	SchemaVersion int     `json:"schemaVersion"`
	Entries       []Entry `json:"entries"`
}

type Store struct {
	path  string
	mutex sync.Mutex
}

func New(root string) *Store {
	return &Store{path: filepath.Join(root, "images", "cache.json")}
}

func (store *Store) Record(
	id string,
	reference string,
	usedAt time.Time,
) error {
	id = strings.TrimSpace(id)
	reference = strings.TrimSpace(reference)
	if id == "" || reference == "" {
		return errors.New("image ID and reference are required")
	}
	if usedAt.IsZero() {
		return errors.New("image use time is required")
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()

	current, err := store.read()
	if err != nil {
		return err
	}
	found := false
	for index := range current.Entries {
		entry := &current.Entries[index]
		if entry.ID != id {
			continue
		}
		if !contains(entry.References, reference) {
			entry.References = append(entry.References, reference)
			sort.Strings(entry.References)
		}
		entry.LastUsedAt = usedAt.UTC()
		found = true
		break
	}
	if !found {
		current.Entries = append(current.Entries, Entry{
			ID: id, References: []string{reference}, LastUsedAt: usedAt.UTC(),
		})
	}
	sort.Slice(current.Entries, func(left int, right int) bool {
		return current.Entries[left].ID < current.Entries[right].ID
	})
	return store.write(current)
}

func (store *Store) Entries() ([]Entry, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	current, err := store.read()
	if err != nil {
		return nil, err
	}
	return cloneEntries(current.Entries), nil
}

func (store *Store) Forget(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("image ID is required")
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	current, err := store.read()
	if err != nil {
		return err
	}
	entries := current.Entries[:0]
	for _, entry := range current.Entries {
		if entry.ID != id {
			entries = append(entries, entry)
		}
	}
	if len(entries) == len(current.Entries) {
		return nil
	}
	current.Entries = entries
	return store.write(current)
}

func (store *Store) read() (state, error) {
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return state{SchemaVersion: schemaVersion, Entries: []Entry{}}, nil
	}
	if err != nil {
		return state{}, fmt.Errorf("read image cache: %w", err)
	}
	var current state
	if err := json.Unmarshal(data, &current); err != nil {
		return state{}, fmt.Errorf("decode image cache: %w", err)
	}
	if current.SchemaVersion != schemaVersion {
		return state{}, fmt.Errorf(
			"unsupported image cache schema version %d",
			current.SchemaVersion,
		)
	}
	for _, entry := range current.Entries {
		if strings.TrimSpace(entry.ID) == "" || len(entry.References) == 0 ||
			entry.LastUsedAt.IsZero() {
			return state{}, errors.New("image cache contains an invalid entry")
		}
		for _, reference := range entry.References {
			if strings.TrimSpace(reference) == "" {
				return state{}, errors.New("image cache contains an empty reference")
			}
		}
	}
	return current, nil
}

func (store *Store) write(current state) error {
	current.SchemaVersion = schemaVersion
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode image cache: %w", err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create image cache directory: %w", err)
	}
	temporary := store.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write image cache: %w", err)
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("secure image cache: %w", err)
	}
	if err := os.Rename(temporary, store.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit image cache: %w", err)
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func cloneEntries(entries []Entry) []Entry {
	cloned := make([]Entry, len(entries))
	for index, entry := range entries {
		cloned[index] = entry
		cloned[index].References = append([]string(nil), entry.References...)
	}
	return cloned
}
