package catalog

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed manifests
var files embed.FS

type Manifest struct {
	ID                    string            `json:"id"`
	Slug                  string            `json:"slug"`
	Name                  string            `json:"name"`
	Publisher             string            `json:"publisher"`
	Description           string            `json:"description"`
	Tags                  []string          `json:"tags"`
	Media                 Media             `json:"media"`
	Image                 string            `json:"image"`
	Viewer                string            `json:"viewer"`
	ContainerPort         int               `json:"containerPort"`
	Memory                string            `json:"memory,omitempty"`
	SharedMemory          string            `json:"sharedMemory"`
	ResolutionEnvironment string            `json:"resolutionEnvironment,omitempty"`
	Resolution            string            `json:"resolution"`
	Environment           map[string]string `json:"environment"`
	Mounts                []Mount           `json:"mounts"`
}

type Media struct {
	Icon        string       `json:"icon"`
	Cover       string       `json:"cover"`
	Screenshots []Screenshot `json:"screenshots"`
}

type Screenshot struct {
	Source string `json:"source"`
	Alt    string `json:"alt"`
}

type Mount struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

func Load(id string) (Manifest, error) {
	if !safeSlug(id) {
		return Manifest{}, fmt.Errorf("invalid catalogue filename %q", id)
	}
	data, err := files.ReadFile(path.Join("manifests", id, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read catalogue entry %q: %w", id, err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode catalogue entry %q: %w", id, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("validate catalogue entry %q: %w", id, err)
	}
	if err := manifest.validateAssets(); err != nil {
		return Manifest{}, fmt.Errorf("validate catalogue entry %q: %w", id, err)
	}
	return manifest, nil
}

func Assets() fs.FS {
	assets, err := fs.Sub(files, "manifests")
	if err != nil {
		panic(fmt.Sprintf("open embedded catalogue assets: %v", err))
	}
	return assets
}

func List() ([]Manifest, error) {
	entries, err := files.ReadDir("manifests")
	if err != nil {
		return nil, fmt.Errorf("list catalogue: %w", err)
	}
	manifests := make([]Manifest, 0, len(entries))
	ids := make(map[string]string, len(entries))
	slugs := make(map[string]string, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, loadErr := Load(entry.Name())
		if loadErr != nil {
			return nil, loadErr
		}
		if owner, exists := ids[manifest.ID]; exists {
			return nil, fmt.Errorf(
				"catalogue ID %q is used by both %q and %q",
				manifest.ID,
				owner,
				manifest.Slug,
			)
		}
		if owner, exists := slugs[manifest.Slug]; exists {
			return nil, fmt.Errorf(
				"catalogue slug %q is used by both %q and %q",
				manifest.Slug,
				owner,
				manifest.ID,
			)
		}
		ids[manifest.ID] = manifest.Slug
		slugs[manifest.Slug] = manifest.ID
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(left, right int) bool {
		return manifests[left].Name < manifests[right].Name
	})
	return manifests, nil
}

func (manifest Manifest) Validate() error {
	if !validUUID(manifest.ID) {
		return errors.New("catalogue ID must be a lowercase UUID")
	}
	if !safeSlug(manifest.Slug) {
		return errors.New(
			"catalogue slug must use lowercase letters, numbers, and hyphens",
		)
	}
	if strings.TrimSpace(manifest.Name) == "" ||
		strings.TrimSpace(manifest.Publisher) == "" {
		return errors.New("name and publisher are required")
	}
	if strings.TrimSpace(manifest.Description) == "" {
		return errors.New("description is required")
	}
	if len(manifest.Tags) == 0 {
		return errors.New("at least one tag is required")
	}
	for _, tag := range manifest.Tags {
		if strings.TrimSpace(tag) == "" {
			return errors.New("tags cannot be empty")
		}
	}
	if err := manifest.Media.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(manifest.Image) == "" {
		return errors.New("image is required")
	}
	if manifest.Viewer != "web" && manifest.Viewer != "kasmvnc" {
		return errors.New("viewer must be web or kasmvnc")
	}
	if manifest.ContainerPort < 1 || manifest.ContainerPort > 65535 {
		return errors.New("container port must be between 1 and 65535")
	}
	if strings.TrimSpace(manifest.SharedMemory) == "" ||
		strings.TrimSpace(manifest.Resolution) == "" {
		return errors.New("shared memory and resolution are required")
	}
	if strings.TrimSpace(manifest.ResolutionEnvironment) == "" {
		return errors.New("resolution environment variable is required")
	}
	seen := make(map[string]struct{}, len(manifest.Mounts))
	for _, mount := range manifest.Mounts {
		if !safeRelativePath(mount.Name) {
			return fmt.Errorf("mount name %q is not a safe relative path", mount.Name)
		}
		if !path.IsAbs(mount.Target) || path.Clean(mount.Target) == "/" {
			return fmt.Errorf("mount target %q must be an absolute path", mount.Target)
		}
		if _, exists := seen[mount.Name]; exists {
			return fmt.Errorf("mount name %q is duplicated", mount.Name)
		}
		seen[mount.Name] = struct{}{}
	}
	return nil
}

func (media Media) validate() error {
	if !safeAssetPath(media.Icon) {
		return errors.New("media icon must be a safe relative asset path")
	}
	if !safeAssetPath(media.Cover) {
		return errors.New("media cover must be a safe relative asset path")
	}
	if len(media.Screenshots) == 0 {
		return errors.New("at least one media screenshot is required")
	}
	for _, screenshot := range media.Screenshots {
		if !safeAssetPath(screenshot.Source) {
			return errors.New("screenshot source must be a safe relative asset path")
		}
		if strings.TrimSpace(screenshot.Alt) == "" {
			return errors.New("screenshot alt text is required")
		}
	}
	return nil
}

func (manifest Manifest) validateAssets() error {
	names := []string{manifest.Media.Icon, manifest.Media.Cover}
	for _, screenshot := range manifest.Media.Screenshots {
		names = append(names, screenshot.Source)
	}
	for _, name := range names {
		info, err := fs.Stat(files, path.Join("manifests", name))
		if err != nil {
			return fmt.Errorf("media asset %q: %w", name, err)
		}
		if info.IsDir() {
			return fmt.Errorf("media asset %q is a directory", name)
		}
	}
	return nil
}

func safeSlug(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9' && index > 0) ||
			(character == '-' && index > 0 && index < len(value)-1) {
			continue
		}
		return false
	}
	return true
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdef", character) {
				return false
			}
		}
	}
	return true
}

func safeRelativePath(value string) bool {
	if value == "" || path.IsAbs(value) || strings.Contains(value, `\`) {
		return false
	}
	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." &&
		!strings.HasPrefix(cleaned, "../")
}

func safeAssetPath(value string) bool {
	if !safeRelativePath(value) {
		return false
	}
	switch strings.ToLower(path.Ext(value)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".svg":
		return true
	default:
		return false
	}
}
