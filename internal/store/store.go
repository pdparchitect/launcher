package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
)

var (
	ErrNotFound      = errors.New("agent not found")
	ErrDuplicateName = errors.New("an agent with that name already exists")
)

type Store struct{ root string }

type Paths struct {
	Root   string
	Mounts map[string]string
}

type Issue struct {
	ID    string
	Error string
}

func New(root string) *Store          { return &Store{root: root} }
func (dataStore *Store) Root() string { return dataStore.root }

func (dataStore *Store) Create(
	instance domain.Instance,
	manifest catalog.Manifest,
) (Paths, error) {
	if err := instance.Validate(); err != nil {
		return Paths{}, fmt.Errorf("validate agent: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Paths{}, fmt.Errorf("validate catalogue entry: %w", err)
	}
	instances, err := dataStore.List()
	if err != nil {
		return Paths{}, err
	}
	for _, existing := range instances {
		if strings.EqualFold(existing.Name, instance.Name) {
			return Paths{}, ErrDuplicateName
		}
	}
	paths := dataStore.Paths(instance.ID, manifest)
	if _, err := os.Stat(paths.Root); err == nil {
		return Paths{}, fmt.Errorf("agent ID %q already exists", instance.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Paths{}, fmt.Errorf("inspect agent directory: %w", err)
	}
	if err := os.MkdirAll(paths.Root, 0o700); err != nil {
		return Paths{}, fmt.Errorf("create agent directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(paths.Root)
		}
	}()
	for _, mountPath := range paths.Mounts {
		if err := os.MkdirAll(mountPath, 0o700); err != nil {
			return Paths{}, fmt.Errorf("create mount directory: %w", err)
		}
	}
	if err := dataStore.write(instance); err != nil {
		return Paths{}, err
	}
	cleanup = false
	return paths, nil
}

func (dataStore *Store) Save(instance domain.Instance) error {
	if err := instance.Validate(); err != nil {
		return fmt.Errorf("validate agent: %w", err)
	}
	if _, err := os.Stat(dataStore.instanceRoot(instance.ID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	instances, err := dataStore.List()
	if err != nil {
		return err
	}
	for _, existing := range instances {
		if existing.ID != instance.ID &&
			strings.EqualFold(existing.Name, instance.Name) {
			return ErrDuplicateName
		}
	}
	return dataStore.write(instance)
}

func (dataStore *Store) List() ([]domain.Instance, error) {
	instances, _, err := dataStore.ListWithIssues()
	return instances, err
}

func (dataStore *Store) ListWithIssues() (
	[]domain.Instance,
	[]Issue,
	error,
) {
	entries, err := os.ReadDir(dataStore.agentsRoot())
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Instance{}, []Issue{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("list agents: %w", err)
	}
	instances := make([]domain.Instance, 0, len(entries))
	issues := make([]Issue, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !domain.ValidID(entry.Name()) {
			continue
		}
		instance, readErr := dataStore.read(entry.Name())
		if readErr != nil {
			issues = append(issues, Issue{
				ID:    entry.Name(),
				Error: readErr.Error(),
			})
			continue
		}
		instances = append(instances, instance)
	}
	sort.Slice(instances, func(left, right int) bool {
		return strings.ToLower(instances[left].Name) <
			strings.ToLower(instances[right].Name)
	})
	return instances, issues, nil
}

func (dataStore *Store) Get(reference string) (domain.Instance, error) {
	if domain.ValidID(reference) {
		instance, err := dataStore.read(reference)
		if errors.Is(err, os.ErrNotExist) {
			return domain.Instance{}, ErrNotFound
		}
		return instance, err
	}
	instances, err := dataStore.List()
	if err != nil {
		return domain.Instance{}, err
	}
	for _, instance := range instances {
		if strings.EqualFold(instance.Name, reference) {
			return instance, nil
		}
	}
	return domain.Instance{}, ErrNotFound
}

func (dataStore *Store) Delete(id string) error {
	if !domain.ValidID(id) {
		return errors.New("invalid agent ID")
	}
	root := dataStore.instanceRoot(id)
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("delete agent directory: %w", err)
	}
	return nil
}

func (dataStore *Store) Paths(id string, manifest catalog.Manifest) Paths {
	root := dataStore.instanceRoot(id)
	mounts := make(map[string]string, len(manifest.Mounts))
	for _, mount := range manifest.Mounts {
		mounts[mount.Name] = filepath.Join(root, filepath.FromSlash(mount.Name))
	}
	return Paths{Root: root, Mounts: mounts}
}

func (dataStore *Store) AgentRoot(id string) (string, error) {
	if !domain.ValidID(id) {
		return "", errors.New("invalid agent ID")
	}
	root := dataStore.instanceRoot(id)
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("inspect agent directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("agent path is not a directory")
	}
	return root, nil
}

func (dataStore *Store) write(instance domain.Instance) error {
	data, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent state: %w", err)
	}
	data = append(data, '\n')
	target := filepath.Join(dataStore.instanceRoot(instance.ID), "instance.json")
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write agent state: %w", err)
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("secure agent state: %w", err)
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit agent state: %w", err)
	}
	return nil
}

func (dataStore *Store) read(id string) (domain.Instance, error) {
	data, err := os.ReadFile(filepath.Join(dataStore.instanceRoot(id), "instance.json"))
	if err != nil {
		return domain.Instance{}, err
	}
	var instance domain.Instance
	if err := json.Unmarshal(data, &instance); err != nil {
		return domain.Instance{}, fmt.Errorf("decode agent %q: %w", id, err)
	}
	if err := instance.Validate(); err != nil {
		return domain.Instance{}, fmt.Errorf("validate agent %q: %w", id, err)
	}
	return instance, nil
}

func (dataStore *Store) agentsRoot() string {
	return filepath.Join(dataStore.root, "agents")
}
func (dataStore *Store) instanceRoot(id string) string {
	return filepath.Join(dataStore.agentsRoot(), id)
}
