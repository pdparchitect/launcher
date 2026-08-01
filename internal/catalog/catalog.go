package catalog

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"
)

const (
	ApplicationArtifactType = "application/vnd.pdparchitect.launcher.application.v1"
	ApplicationBundleType   = "application/vnd.pdparchitect.launcher.application.bundle.v1+zip"
	FeedArtifactType        = "application/vnd.pdparchitect.launcher.feed.v1"
	FeedDocumentType        = "application/vnd.pdparchitect.launcher.feed.v1+json"

	maxBundleFileSize = 16 << 20
	maxBundleContents = 32 << 20
	maxBundleFiles    = 64
)

type Manifest struct {
	ID           string               `json:"id"`
	Slug         string               `json:"slug"`
	Name         string               `json:"name"`
	Publisher    string               `json:"publisher"`
	Description  string               `json:"description"`
	Tags         []string             `json:"tags"`
	Media        Media                `json:"media"`
	Image        string               `json:"image"`
	Interfaces   map[string]Interface `json:"interfaces"`
	Memory       string               `json:"memory,omitempty"`
	SharedMemory string               `json:"sharedMemory"`
	Environment  map[string]string    `json:"environment"`
	Mounts       []Mount              `json:"mounts"`
}

type applicationDocument struct {
	SchemaVersion int                  `json:"schemaVersion"`
	ID            string               `json:"id"`
	Slug          string               `json:"slug"`
	Name          string               `json:"name"`
	Publisher     string               `json:"publisher"`
	Description   string               `json:"description"`
	Tags          []string             `json:"tags"`
	Media         Media                `json:"media"`
	Interfaces    map[string]Interface `json:"interfaces"`
	Memory        string               `json:"memory,omitempty"`
	SharedMemory  string               `json:"sharedMemory"`
	Environment   map[string]string    `json:"environment"`
	Mounts        []Mount              `json:"mounts"`
}

type Feed struct {
	SchemaVersion int      `json:"schemaVersion"`
	Publisher     string   `json:"publisher"`
	Applications  []string `json:"applications"`
}

type Sources struct {
	SchemaVersion int      `json:"schemaVersion"`
	Feeds         []string `json:"feeds"`
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
	Name    string `json:"name"`
	Target  string `json:"target"`
	Storage string `json:"storage,omitempty"`
}

const (
	MountStorageHost   = "host"
	MountStorageVolume = "volume"
)

type Interface struct {
	Kind string `json:"kind"`
	Port int    `json:"port"`
	Path string `json:"path"`
}

type applicationBundle struct {
	Manifest Manifest
	Assets   map[string][]byte
}

// Assets returns an empty filesystem for callers that do not supply a registry
// manager. Production callers pass Manager, which serves the active snapshot.
func Assets() fs.FS {
	return fstest.MapFS{}
}

func parseSources(data []byte) (Sources, error) {
	var sources Sources
	if err := decodeStrictJSON(data, &sources); err != nil {
		return Sources{}, fmt.Errorf("decode registry sources: %w", err)
	}
	if sources.SchemaVersion != 1 {
		return Sources{}, fmt.Errorf(
			"unsupported registry sources schema version %d",
			sources.SchemaVersion,
		)
	}
	if len(sources.Feeds) == 0 {
		return Sources{}, errors.New("registry sources must contain a feed")
	}
	seen := make(map[string]struct{}, len(sources.Feeds))
	for _, reference := range sources.Feeds {
		if strings.TrimSpace(reference) != reference || reference == "" {
			return Sources{}, errors.New("registry feed references cannot be empty")
		}
		if _, exists := seen[reference]; exists {
			return Sources{}, fmt.Errorf(
				"registry feed reference %q is duplicated",
				reference,
			)
		}
		seen[reference] = struct{}{}
	}
	return sources, nil
}

func parseFeed(data []byte) (Feed, error) {
	var feed Feed
	if err := decodeStrictJSON(data, &feed); err != nil {
		return Feed{}, fmt.Errorf("decode publisher feed: %w", err)
	}
	if feed.SchemaVersion != 1 {
		return Feed{}, fmt.Errorf(
			"unsupported publisher feed schema version %d",
			feed.SchemaVersion,
		)
	}
	if strings.TrimSpace(feed.Publisher) == "" {
		return Feed{}, errors.New("publisher feed publisher is required")
	}
	if len(feed.Applications) == 0 {
		return Feed{}, errors.New("publisher feed must contain an application")
	}
	seen := make(map[string]struct{}, len(feed.Applications))
	for _, reference := range feed.Applications {
		if strings.TrimSpace(reference) != reference || reference == "" {
			return Feed{}, errors.New(
				"publisher feed application references cannot be empty",
			)
		}
		if _, exists := seen[reference]; exists {
			return Feed{}, fmt.Errorf(
				"publisher feed application %q is duplicated",
				reference,
			)
		}
		seen[reference] = struct{}{}
	}
	return feed, nil
}

func loadApplicationBundle(
	data []byte,
	imageRepository string,
	imageDigest string,
) (applicationBundle, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return applicationBundle{}, fmt.Errorf("open application bundle: %w", err)
	}
	files := make(map[string][]byte, len(reader.File))
	var total uint64
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		if name == "" {
			continue
		}
		if !fs.ValidPath(name) || strings.Contains(file.Name, `\`) {
			return applicationBundle{}, fmt.Errorf(
				"application bundle path %q is invalid",
				file.Name,
			)
		}
		if file.Mode()&fs.ModeSymlink != 0 {
			return applicationBundle{}, fmt.Errorf(
				"application bundle path %q is a symbolic link",
				file.Name,
			)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if _, exists := files[name]; exists {
			return applicationBundle{}, fmt.Errorf(
				"application bundle path %q is duplicated",
				file.Name,
			)
		}
		if len(files) >= maxBundleFiles ||
			file.UncompressedSize64 > maxBundleFileSize {
			return applicationBundle{}, errors.New(
				"application bundle exceeds file limits",
			)
		}
		total += file.UncompressedSize64
		if total > maxBundleContents {
			return applicationBundle{}, errors.New(
				"application bundle exceeds uncompressed size limit",
			)
		}
		source, openErr := file.Open()
		if openErr != nil {
			return applicationBundle{}, fmt.Errorf(
				"open application bundle path %q: %w",
				name,
				openErr,
			)
		}
		content, readErr := io.ReadAll(source)
		closeErr := source.Close()
		if readErr != nil {
			return applicationBundle{}, fmt.Errorf(
				"read application bundle path %q: %w",
				name,
				readErr,
			)
		}
		if closeErr != nil {
			return applicationBundle{}, fmt.Errorf(
				"close application bundle path %q: %w",
				name,
				closeErr,
			)
		}
		files[name] = content
	}

	documentData, exists := files["application.json"]
	if !exists {
		return applicationBundle{}, errors.New(
			"application bundle has no application.json",
		)
	}
	var document applicationDocument
	if err := json.Unmarshal(documentData, &document); err != nil {
		return applicationBundle{}, fmt.Errorf(
			"decode application document: %w",
			err,
		)
	}
	if document.SchemaVersion != 2 {
		return applicationBundle{}, fmt.Errorf(
			"unsupported application schema version %d",
			document.SchemaVersion,
		)
	}
	if imageRepository == "" || !strings.HasPrefix(imageDigest, "sha256:") {
		return applicationBundle{}, errors.New(
			"application artifact must identify its image subject",
		)
	}
	manifest := Manifest{
		ID:           document.ID,
		Slug:         document.Slug,
		Name:         document.Name,
		Publisher:    document.Publisher,
		Description:  document.Description,
		Tags:         document.Tags,
		Media:        document.Media,
		Image:        imageRepository + "@" + imageDigest,
		Interfaces:   document.Interfaces,
		Memory:       document.Memory,
		SharedMemory: document.SharedMemory,
		Environment:  document.Environment,
		Mounts:       document.Mounts,
	}
	if err := manifest.Validate(); err != nil {
		return applicationBundle{}, fmt.Errorf(
			"validate application document: %w",
			err,
		)
	}

	localAssets := manifest.assetPaths()
	assets := make(map[string][]byte, len(localAssets))
	for _, name := range localAssets {
		content, assetExists := files[name]
		if !assetExists {
			return applicationBundle{}, fmt.Errorf(
				"application media asset %q is missing",
				name,
			)
		}
		assets[path.Join(manifest.Slug, name)] = content
	}
	manifest.prefixAssets()
	return applicationBundle{
		Manifest: manifest,
		Assets:   assets,
	}, nil
}

// references carries the order the publisher feeds declare, which is the order
// the catalogue is presented in. Ranging over bundles instead would hand back a
// different catalogue on every refresh.
func manifestsFromBundles(
	bundles map[string]applicationBundle,
	references []string,
) ([]Manifest, fstest.MapFS, error) {
	manifests := make([]Manifest, 0, len(bundles))
	assets := make(fstest.MapFS)
	ids := make(map[string]string, len(bundles))
	slugs := make(map[string]string, len(bundles))
	for _, reference := range references {
		bundle, resolved := bundles[reference]
		if !resolved {
			continue
		}
		manifest := bundle.Manifest
		if owner, exists := ids[manifest.ID]; exists {
			return nil, nil, fmt.Errorf(
				"application ID %q is published by both %q and %q",
				manifest.ID,
				owner,
				reference,
			)
		}
		if owner, exists := slugs[manifest.Slug]; exists {
			return nil, nil, fmt.Errorf(
				"application slug %q is published by both %q and %q",
				manifest.Slug,
				owner,
				reference,
			)
		}
		ids[manifest.ID] = reference
		slugs[manifest.Slug] = reference
		for name, content := range bundle.Assets {
			assets[name] = &fstest.MapFile{Data: append([]byte(nil), content...)}
		}
		manifests = append(manifests, manifest)
	}
	return manifests, assets, nil
}

func (manifest Manifest) Validate() error {
	if !validUUID(manifest.ID) {
		return errors.New("application ID must be a lowercase UUID")
	}
	if !safeSlug(manifest.Slug) {
		return errors.New(
			"application slug must use lowercase letters, numbers, and hyphens",
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
	image := strings.TrimSpace(manifest.Image)
	if image == "" {
		return errors.New("image is required")
	}
	if strings.HasSuffix(image, ":latest") {
		return errors.New("image must use an immutable release tag or digest")
	}
	if len(manifest.Interfaces) == 0 {
		return errors.New("at least one interface is required")
	}
	for id, definition := range manifest.Interfaces {
		if !safeSlug(id) {
			return fmt.Errorf(
				"interface ID %q must use lowercase letters, numbers, and hyphens",
				id,
			)
		}
		if !safeSlug(definition.Kind) {
			return fmt.Errorf(
				"interface %q kind must use lowercase letters, numbers, and hyphens",
				id,
			)
		}
		if definition.Port < 1 || definition.Port > 65535 {
			return fmt.Errorf(
				"interface %q port must be between 1 and 65535",
				id,
			)
		}
		if !validInterfacePath(definition.Path) {
			return fmt.Errorf(
				"interface %q path must be a clean absolute URL path",
				id,
			)
		}
	}
	if strings.TrimSpace(manifest.SharedMemory) == "" {
		return errors.New("shared memory is required")
	}
	seen := make(map[string]struct{}, len(manifest.Mounts))
	for _, mount := range manifest.Mounts {
		if !safeRelativePath(mount.Name) {
			return fmt.Errorf("mount name %q is not a safe relative path", mount.Name)
		}
		if !path.IsAbs(mount.Target) || path.Clean(mount.Target) == "/" {
			return fmt.Errorf("mount target %q must be an absolute path", mount.Target)
		}
		if mount.Storage != "" &&
			mount.Storage != MountStorageHost &&
			mount.Storage != MountStorageVolume {
			return fmt.Errorf(
				"mount storage %q must be %q or %q when specified",
				mount.Storage,
				MountStorageHost,
				MountStorageVolume,
			)
		}
		if _, exists := seen[mount.Name]; exists {
			return fmt.Errorf("mount name %q is duplicated", mount.Name)
		}
		seen[mount.Name] = struct{}{}
	}
	return nil
}

func (manifest Manifest) assetPaths() []string {
	names := []string{manifest.Media.Icon, manifest.Media.Cover}
	for _, screenshot := range manifest.Media.Screenshots {
		names = append(names, screenshot.Source)
	}
	return names
}

func (manifest *Manifest) prefixAssets() {
	manifest.Media.Icon = path.Join(manifest.Slug, manifest.Media.Icon)
	manifest.Media.Cover = path.Join(manifest.Slug, manifest.Media.Cover)
	for index := range manifest.Media.Screenshots {
		manifest.Media.Screenshots[index].Source = path.Join(
			manifest.Slug,
			manifest.Media.Screenshots[index].Source,
		)
	}
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

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
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

func validInterfacePath(value string) bool {
	return strings.HasPrefix(value, "/") &&
		path.Clean(value) == value &&
		!strings.ContainsAny(value, "?#")
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
