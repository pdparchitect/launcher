package catalog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRefreshInterval = 30 * time.Minute
	bundleAssetName        = "launcher-catalogue.zip"
	defaultReleasesURL     = "https://api.github.com/repos/pdparchitect/launcher/releases?per_page=100"
	maxReleaseResponse     = 2 << 20
	maxBundleSize          = 32 << 20
	maxBundleFileSize      = 16 << 20
	maxBundleContents      = 64 << 20
	maxBundleFiles         = 1000
)

var semanticVersion = regexp.MustCompile(
	`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`,
)

type Metadata struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
}

type ManagerOptions struct {
	Client          *http.Client
	ReleasesURL     string
	RefreshInterval time.Duration
	Now             func() time.Time
}

type snapshot struct {
	version   string
	manifests []Manifest
	assets    fs.FS
}

type cacheState struct {
	CheckedAt time.Time `json:"checkedAt"`
	Version   string    `json:"version,omitempty"`
	Tag       string    `json:"tag,omitempty"`
	Digest    string    `json:"digest,omitempty"`
	Bundle    string    `json:"bundle,omitempty"`
	ETag      string    `json:"etag,omitempty"`
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Digest      string `json:"digest"`
	Size        int64  `json:"size"`
}

type Manager struct {
	mutex           sync.RWMutex
	refreshMutex    sync.Mutex
	root            string
	client          *http.Client
	releasesURL     string
	refreshInterval time.Duration
	now             func() time.Time
	current         snapshot
	state           cacheState
}

func NewManager(root string, options ManagerOptions) (*Manager, error) {
	embedded, err := loadSnapshot(files)
	if err != nil {
		return nil, fmt.Errorf("load embedded catalogue: %w", err)
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 30 * time.Second}
	}
	if options.ReleasesURL == "" {
		options.ReleasesURL = defaultReleasesURL
	}
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = DefaultRefreshInterval
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	manager := &Manager{
		root:            filepath.Join(root, "catalogue"),
		client:          options.Client,
		releasesURL:     options.ReleasesURL,
		refreshInterval: options.RefreshInterval,
		now:             options.Now,
		current:         embedded,
	}
	manager.restore()
	return manager, nil
}

func (manager *Manager) List() []Manifest {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return append([]Manifest(nil), manager.current.manifests...)
}

func (manager *Manager) Version() string {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return manager.current.version
}

// Open implements fs.FS for catalogue artwork. Each opened file retains the
// snapshot it came from while a concurrent refresh swaps future requests.
func (manager *Manager) Open(name string) (fs.File, error) {
	manager.mutex.RLock()
	assets := manager.current.assets
	manager.mutex.RUnlock()
	return assets.Open(name)
}

func (manager *Manager) Refresh(
	ctx context.Context,
	force bool,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	manager.refreshMutex.Lock()
	defer manager.refreshMutex.Unlock()

	manager.mutex.RLock()
	state := manager.state
	manager.mutex.RUnlock()
	now := manager.now().UTC()
	if !force && !state.CheckedAt.IsZero() &&
		now.Sub(state.CheckedAt) < manager.refreshInterval {
		return false, nil
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		manager.releasesURL,
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("create catalogue release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	request.Header.Set("User-Agent", "pdparchitect-launcher")
	if state.ETag != "" {
		request.Header.Set("If-None-Match", state.ETag)
	}
	response, err := manager.client.Do(request)
	if err != nil {
		return false, fmt.Errorf("check catalogue releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		state.CheckedAt = now
		if err := manager.commitState(state); err != nil {
			return false, err
		}
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return false, fmt.Errorf(
			"check catalogue releases: GitHub returned %s",
			response.Status,
		)
	}
	data, err := readBounded(response.Body, maxReleaseResponse)
	if err != nil {
		return false, fmt.Errorf("read catalogue releases: %w", err)
	}
	var releases []githubRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return false, fmt.Errorf("decode catalogue releases: %w", err)
	}
	release, asset, err := selectRelease(releases)
	if err != nil {
		return false, err
	}
	digest, err := parseDigest(asset.Digest)
	if err != nil {
		return false, fmt.Errorf("catalogue release %q: %w", release.TagName, err)
	}
	etag := response.Header.Get("ETag")
	if state.Digest == asset.Digest && state.Tag == release.TagName {
		state.CheckedAt = now
		state.ETag = etag
		if err := manager.commitState(state); err != nil {
			return false, err
		}
		return false, nil
	}
	if asset.Size < 1 || asset.Size > maxBundleSize {
		return false, fmt.Errorf(
			"catalogue release %q has invalid asset size %d",
			release.TagName,
			asset.Size,
		)
	}
	bundle, err := manager.download(ctx, asset)
	if err != nil {
		return false, err
	}
	actual := sha256.Sum256(bundle)
	if subtle.ConstantTimeCompare(actual[:], digest) != 1 {
		return false, fmt.Errorf(
			"catalogue release %q asset digest does not match GitHub metadata",
			release.TagName,
		)
	}
	next, err := loadArchive(bundle)
	if err != nil {
		return false, fmt.Errorf(
			"validate catalogue release %q: %w",
			release.TagName,
			err,
		)
	}
	if release.TagName != "catalogue-v"+next.version {
		return false, fmt.Errorf(
			"catalogue release tag %q does not match bundle version %q",
			release.TagName,
			next.version,
		)
	}

	hexDigest := hex.EncodeToString(digest)
	bundleName := filepath.Join("bundles", hexDigest+".zip")
	if err := manager.writeBundle(bundleName, bundle); err != nil {
		return false, err
	}
	nextState := cacheState{
		CheckedAt: now,
		Version:   next.version,
		Tag:       release.TagName,
		Digest:    asset.Digest,
		Bundle:    bundleName,
		ETag:      etag,
	}
	if err := writeJSONAtomic(manager.statePath(), nextState); err != nil {
		return false, fmt.Errorf("save catalogue state: %w", err)
	}
	manager.mutex.Lock()
	manager.current = next
	manager.state = nextState
	manager.mutex.Unlock()
	return true, nil
}

func (manager *Manager) download(
	ctx context.Context,
	asset githubAsset,
) ([]byte, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		asset.DownloadURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create catalogue download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "pdparchitect-launcher")
	response, err := manager.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download catalogue release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil, fmt.Errorf(
			"download catalogue release: GitHub returned %s",
			response.Status,
		)
	}
	data, err := readBounded(response.Body, maxBundleSize)
	if err != nil {
		return nil, fmt.Errorf("download catalogue release: %w", err)
	}
	if int64(len(data)) != asset.Size {
		return nil, fmt.Errorf(
			"download catalogue release: got %d bytes, want %d",
			len(data),
			asset.Size,
		)
	}
	return data, nil
}

func (manager *Manager) restore() {
	data, err := os.ReadFile(manager.statePath())
	if err != nil {
		return
	}
	var state cacheState
	if json.Unmarshal(data, &state) != nil {
		return
	}
	if state.Bundle == "" {
		manager.state = state
		return
	}
	digest, err := parseDigest(state.Digest)
	if err != nil || !safeBundleName(state.Bundle) {
		return
	}
	bundlePath := filepath.Join(manager.root, filepath.FromSlash(state.Bundle))
	bundle, err := readFileBounded(bundlePath, maxBundleSize)
	if err != nil {
		return
	}
	actual := sha256.Sum256(bundle)
	if subtle.ConstantTimeCompare(actual[:], digest) != 1 {
		return
	}
	cached, err := loadArchive(bundle)
	if err != nil || cached.version != state.Version ||
		state.Tag != "catalogue-v"+cached.version {
		return
	}
	manager.current = cached
	manager.state = state
}

func (manager *Manager) commitState(state cacheState) error {
	if err := writeJSONAtomic(manager.statePath(), state); err != nil {
		return fmt.Errorf("save catalogue state: %w", err)
	}
	manager.mutex.Lock()
	manager.state = state
	manager.mutex.Unlock()
	return nil
}

func (manager *Manager) markChecked(checkedAt time.Time) {
	manager.mutex.Lock()
	manager.state.CheckedAt = checkedAt
	manager.mutex.Unlock()
}

func (manager *Manager) writeBundle(name string, data []byte) error {
	target := filepath.Join(manager.root, filepath.FromSlash(name))
	if err := writeFileAtomic(target, data, 0o600); err != nil {
		return fmt.Errorf("cache catalogue bundle: %w", err)
	}
	return nil
}

func (manager *Manager) statePath() string {
	return filepath.Join(manager.root, "state.json")
}

func loadSnapshot(source fs.FS) (snapshot, error) {
	metadata, err := loadMetadata(source)
	if err != nil {
		return snapshot{}, err
	}
	manifests, err := list(source)
	if err != nil {
		return snapshot{}, err
	}
	assets, err := fs.Sub(source, "manifests")
	if err != nil {
		return snapshot{}, fmt.Errorf("open catalogue assets: %w", err)
	}
	return snapshot{
		version:   metadata.Version,
		manifests: manifests,
		assets:    assets,
	}, nil
}

func loadMetadata(source fs.FS) (Metadata, error) {
	data, err := fs.ReadFile(source, "catalogue.json")
	if err != nil {
		return Metadata{}, fmt.Errorf("read catalogue metadata: %w", err)
	}
	var metadata Metadata
	if err := decodeStrictJSON(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("decode catalogue metadata: %w", err)
	}
	if metadata.SchemaVersion != 1 {
		return Metadata{}, fmt.Errorf(
			"unsupported catalogue schema version %d",
			metadata.SchemaVersion,
		)
	}
	if !semanticVersion.MatchString(metadata.Version) {
		return Metadata{}, fmt.Errorf(
			"catalogue version %q is not semantic versioning",
			metadata.Version,
		)
	}
	return metadata, nil
}

func loadArchive(data []byte) (snapshot, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return snapshot{}, fmt.Errorf("open catalogue archive: %w", err)
	}
	seen := make(map[string]struct{}, len(reader.File))
	var total uint64
	var count int
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		if name == "" || !fs.ValidPath(name) || strings.Contains(file.Name, `\`) {
			return snapshot{}, fmt.Errorf(
				"catalogue archive path %q is invalid",
				file.Name,
			)
		}
		if _, exists := seen[name]; exists {
			return snapshot{}, fmt.Errorf(
				"catalogue archive path %q is duplicated",
				file.Name,
			)
		}
		seen[name] = struct{}{}
		if file.Mode()&fs.ModeSymlink != 0 {
			return snapshot{}, fmt.Errorf(
				"catalogue archive path %q is a symbolic link",
				file.Name,
			)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		count++
		if count > maxBundleFiles ||
			file.UncompressedSize64 > maxBundleFileSize {
			return snapshot{}, errors.New("catalogue archive exceeds file limits")
		}
		total += file.UncompressedSize64
		if total > maxBundleContents {
			return snapshot{}, errors.New(
				"catalogue archive exceeds uncompressed size limit",
			)
		}
	}
	return loadSnapshot(reader)
}

func selectRelease(
	releases []githubRelease,
) (githubRelease, githubAsset, error) {
	var selected githubRelease
	var selectedAsset githubAsset
	var selectedVersion [3]uint64
	found := false
	for _, release := range releases {
		if release.Draft || release.Prerelease ||
			!strings.HasPrefix(release.TagName, "catalogue-v") {
			continue
		}
		version, valid := stableVersion(
			strings.TrimPrefix(release.TagName, "catalogue-v"),
		)
		if !valid || found && compareVersion(version, selectedVersion) <= 0 {
			continue
		}
		for _, asset := range release.Assets {
			if asset.Name == bundleAssetName {
				selected = release
				selectedAsset = asset
				selectedVersion = version
				found = true
				break
			}
		}
	}
	if found {
		return selected, selectedAsset, nil
	}
	return githubRelease{}, githubAsset{}, errors.New(
		"no published stable catalogue release was found",
	)
}

func stableVersion(value string) ([3]uint64, bool) {
	if !semanticVersion.MatchString(value) || strings.Contains(value, "-") {
		return [3]uint64{}, false
	}
	parts := strings.Split(value, ".")
	var version [3]uint64
	for index, part := range parts {
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return [3]uint64{}, false
		}
		version[index] = number
	}
	return version, true
}

func compareVersion(left [3]uint64, right [3]uint64) int {
	for index := range left {
		switch {
		case left[index] < right[index]:
			return -1
		case left[index] > right[index]:
			return 1
		}
	}
	return 0
}

func parseDigest(value string) ([]byte, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return nil, errors.New("catalogue asset has no SHA-256 digest")
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil || len(digest) != sha256.Size {
		return nil, errors.New("catalogue asset has an invalid SHA-256 digest")
	}
	return digest, nil
}

func safeBundleName(name string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	return cleaned == name &&
		strings.HasPrefix(name, "bundles/") &&
		!strings.Contains(name, "..") &&
		strings.HasSuffix(name, ".zip")
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d-byte limit", limit)
	}
	return data, nil
}

func readFileBounded(name string, limit int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readBounded(file, limit)
}

func writeJSONAtomic(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(name, append(data, '\n'), 0o600)
}

func writeFileAtomic(name string, data []byte, mode fs.FileMode) error {
	directory := filepath.Dir(name)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".catalogue-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, name); err != nil {
		return err
	}
	committed = true
	return nil
}
