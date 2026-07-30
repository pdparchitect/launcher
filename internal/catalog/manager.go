package catalog

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing/fstest"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	DefaultRefreshInterval = 30 * time.Minute
	maxManifestSize        = 1 << 20
	maxFeedSize            = 1 << 20
	maxApplicationBundle   = 32 << 20
)

var semanticVersion = regexp.MustCompile(
	`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`,
)

//go:embed sources.json
var sourceFiles embed.FS

type Artifact struct {
	Reference      string
	ManifestDigest string
	ArtifactType   string
	LayerType      string
	Repository     string
	SubjectDigest  string
	Content        []byte
}

type Resolver interface {
	Fetch(context.Context, string) (Artifact, error)
}

type ManagerOptions struct {
	Resolver        Resolver
	SourceData      []byte
	RefreshInterval time.Duration
	Now             func() time.Time
}

type snapshot struct {
	manifests []Manifest
	assets    fs.FS
}

type cacheRecord struct {
	Reference      string `json:"reference"`
	ManifestDigest string `json:"manifestDigest"`
	Repository     string `json:"repository,omitempty"`
	SubjectDigest  string `json:"subjectDigest,omitempty"`
	Blob           string `json:"blob"`
}

type cacheState struct {
	CheckedAt    time.Time              `json:"checkedAt"`
	Feeds        map[string]cacheRecord `json:"feeds"`
	Applications map[string]cacheRecord `json:"applications"`
}

type pendingRecord struct {
	record  cacheRecord
	content []byte
}

type applicationResolution struct {
	reference string
	record    cacheRecord
	bundle    applicationBundle
	pending   *pendingRecord
	warning   string
	available bool
}

type Manager struct {
	mutex           sync.RWMutex
	refreshMutex    sync.Mutex
	root            string
	resolver        Resolver
	sources         Sources
	refreshInterval time.Duration
	now             func() time.Time
	current         snapshot
	state           cacheState
	warnings        []string
}

func NewManager(root string, options ManagerOptions) (*Manager, error) {
	sourceData := options.SourceData
	if len(sourceData) == 0 {
		var err error
		sourceData, err = sourceFiles.ReadFile("sources.json")
		if err != nil {
			return nil, fmt.Errorf("read embedded registry sources: %w", err)
		}
	}
	sources, err := parseSources(sourceData)
	if err != nil {
		return nil, err
	}
	if options.Resolver == nil {
		options.Resolver = NewOCIResolver(nil)
	}
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = DefaultRefreshInterval
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	manager := &Manager{
		root:            filepath.Join(root, "registry"),
		resolver:        options.Resolver,
		sources:         sources,
		refreshInterval: options.RefreshInterval,
		now:             options.Now,
		current: snapshot{
			assets: fstest.MapFS{},
		},
		state: cacheState{
			Feeds:        map[string]cacheRecord{},
			Applications: map[string]cacheRecord{},
		},
	}
	manager.restore()
	return manager, nil
}

func (manager *Manager) List() []Manifest {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return append([]Manifest(nil), manager.current.manifests...)
}

func (manager *Manager) Warnings() []string {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return append([]string(nil), manager.warnings...)
}

// Open implements fs.FS for application artwork. Each opened file retains the
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
	previous := cloneState(manager.state)
	manager.mutex.RUnlock()
	now := manager.now().UTC()
	if !force && !previous.CheckedAt.IsZero() &&
		now.Sub(previous.CheckedAt) < manager.refreshInterval {
		return false, nil
	}

	next := cacheState{
		CheckedAt:    now,
		Feeds:        make(map[string]cacheRecord),
		Applications: make(map[string]cacheRecord),
	}
	pending := make(map[string]pendingRecord)
	var warnings []string
	applicationReferences := make(map[string]struct{})

	for _, reference := range manager.sources.Feeds {
		artifact, fetchErr := manager.resolver.Fetch(ctx, reference)
		var feed Feed
		var record cacheRecord
		switch {
		case fetchErr == nil:
			if validateErr := validateFeedArtifact(artifact); validateErr != nil {
				fetchErr = validateErr
			} else if feed, fetchErr = parseFeed(artifact.Content); fetchErr == nil {
				record = recordFromArtifact(artifact)
				pending[record.Blob] = pendingRecord{
					record:  record,
					content: artifact.Content,
				}
			}
		}
		if fetchErr != nil {
			cached, exists := previous.Feeds[reference]
			if !exists {
				warnings = append(warnings, fmt.Sprintf(
					"publisher feed %q: %v",
					reference,
					fetchErr,
				))
				continue
			}
			content, readErr := manager.readRecord(cached, maxFeedSize)
			if readErr != nil {
				warnings = append(warnings, fmt.Sprintf(
					"publisher feed %q: %v; cached copy: %v",
					reference,
					fetchErr,
					readErr,
				))
				continue
			}
			feed, readErr = parseFeed(content)
			if readErr != nil {
				warnings = append(warnings, fmt.Sprintf(
					"publisher feed %q cached copy: %v",
					reference,
					readErr,
				))
				continue
			}
			record = cached
			warnings = append(warnings, fmt.Sprintf(
				"publisher feed %q is using its cached copy: %v",
				reference,
				fetchErr,
			))
		}
		next.Feeds[reference] = record
		for _, applicationReference := range feed.Applications {
			applicationReferences[applicationReference] = struct{}{}
		}
	}

	if len(next.Feeds) == 0 {
		return false, errors.New("no publisher feed is available")
	}

	bundles := make(map[string]applicationBundle)
	references := make([]string, 0, len(applicationReferences))
	for reference := range applicationReferences {
		references = append(references, reference)
	}
	sort.Strings(references)
	results := make(chan applicationResolution, len(references))
	concurrency := make(chan struct{}, 4)
	var resolveGroup sync.WaitGroup
	for _, reference := range references {
		resolveGroup.Add(1)
		go func() {
			defer resolveGroup.Done()
			concurrency <- struct{}{}
			defer func() { <-concurrency }()
			cached, exists := previous.Applications[reference]
			results <- manager.resolveApplication(
				ctx,
				reference,
				cached,
				exists,
			)
		}()
	}
	resolveGroup.Wait()
	close(results)
	resolutions := make(map[string]applicationResolution, len(references))
	for result := range results {
		resolutions[result.reference] = result
	}
	for _, reference := range references {
		result := resolutions[reference]
		if result.warning != "" {
			warnings = append(warnings, result.warning)
		}
		if !result.available {
			continue
		}
		next.Applications[reference] = result.record
		bundles[reference] = result.bundle
		if result.pending != nil {
			pending[result.pending.record.Blob] = *result.pending
		}
	}

	if len(bundles) == 0 {
		return false, errors.New("no Launcher application is available")
	}
	manifests, assets, err := manifestsFromBundles(bundles)
	if err != nil {
		return false, err
	}
	for _, item := range pending {
		if err := manager.writeRecord(item.record, item.content); err != nil {
			return false, err
		}
	}
	if err := writeJSONAtomic(manager.statePath(), next); err != nil {
		return false, fmt.Errorf("save application registry state: %w", err)
	}
	changed := !sameRecords(previous, next)
	manager.mutex.Lock()
	manager.current = snapshot{manifests: manifests, assets: assets}
	manager.state = next
	manager.warnings = warnings
	manager.mutex.Unlock()
	return changed, nil
}

func (manager *Manager) resolveApplication(
	ctx context.Context,
	reference string,
	cached cacheRecord,
	hasCached bool,
) applicationResolution {
	result := applicationResolution{reference: reference}
	artifact, fetchErr := manager.resolver.Fetch(ctx, reference)
	if fetchErr == nil {
		fetchErr = validateApplicationArtifact(artifact)
	}
	if fetchErr == nil {
		result.bundle, fetchErr = loadApplicationBundle(
			artifact.Content,
			artifact.Repository,
			artifact.SubjectDigest,
		)
	}
	if fetchErr == nil {
		result.record = recordFromArtifact(artifact)
		result.pending = &pendingRecord{
			record:  result.record,
			content: artifact.Content,
		}
		result.available = true
		return result
	}
	if !hasCached {
		result.warning = fmt.Sprintf(
			"application %q: %v",
			reference,
			fetchErr,
		)
		return result
	}
	content, readErr := manager.readRecord(cached, maxApplicationBundle)
	if readErr == nil {
		result.bundle, readErr = loadApplicationBundle(
			content,
			cached.Repository,
			cached.SubjectDigest,
		)
	}
	if readErr != nil {
		result.warning = fmt.Sprintf(
			"application %q: %v; cached copy: %v",
			reference,
			fetchErr,
			readErr,
		)
		return result
	}
	result.record = cached
	result.available = true
	result.warning = fmt.Sprintf(
		"application %q is using its cached copy: %v",
		reference,
		fetchErr,
	)
	return result
}

func (manager *Manager) restore() {
	data, err := os.ReadFile(manager.statePath())
	if err != nil {
		return
	}
	var state cacheState
	if decodeStrictJSON(data, &state) != nil {
		return
	}
	if state.Feeds == nil || state.Applications == nil {
		return
	}
	bundles := make(map[string]applicationBundle, len(state.Applications))
	for reference, record := range state.Applications {
		if reference != record.Reference {
			return
		}
		content, readErr := manager.readRecord(record, maxApplicationBundle)
		if readErr != nil {
			return
		}
		bundle, loadErr := loadApplicationBundle(
			content,
			record.Repository,
			record.SubjectDigest,
		)
		if loadErr != nil {
			return
		}
		bundles[reference] = bundle
	}
	manifests, assets, err := manifestsFromBundles(bundles)
	if err != nil {
		return
	}
	manager.current = snapshot{manifests: manifests, assets: assets}
	manager.state = state
}

func validateFeedArtifact(artifact Artifact) error {
	if artifact.ArtifactType != FeedArtifactType {
		return fmt.Errorf(
			"artifact type %q is not %q",
			artifact.ArtifactType,
			FeedArtifactType,
		)
	}
	if artifact.LayerType != FeedDocumentType {
		return fmt.Errorf(
			"layer type %q is not %q",
			artifact.LayerType,
			FeedDocumentType,
		)
	}
	if artifact.SubjectDigest != "" {
		return errors.New("publisher feed artifact must not have a subject")
	}
	return nil
}

func validateApplicationArtifact(artifact Artifact) error {
	if artifact.ArtifactType != ApplicationArtifactType {
		return fmt.Errorf(
			"artifact type %q is not %q",
			artifact.ArtifactType,
			ApplicationArtifactType,
		)
	}
	if artifact.LayerType != ApplicationBundleType {
		return fmt.Errorf(
			"layer type %q is not %q",
			artifact.LayerType,
			ApplicationBundleType,
		)
	}
	if artifact.Repository == "" || artifact.SubjectDigest == "" {
		return errors.New("application artifact must have an image subject")
	}
	return nil
}

func recordFromArtifact(artifact Artifact) cacheRecord {
	safeDigest := strings.ReplaceAll(artifact.ManifestDigest, ":", "-")
	return cacheRecord{
		Reference:      artifact.Reference,
		ManifestDigest: artifact.ManifestDigest,
		Repository:     artifact.Repository,
		SubjectDigest:  artifact.SubjectDigest,
		Blob:           filepath.ToSlash(filepath.Join("blobs", safeDigest)),
	}
}

func (manager *Manager) readRecord(
	record cacheRecord,
	limit int64,
) ([]byte, error) {
	if !safeBlobName(record.Blob) {
		return nil, errors.New("cached blob path is invalid")
	}
	return readFileBounded(
		filepath.Join(manager.root, filepath.FromSlash(record.Blob)),
		limit,
	)
}

func (manager *Manager) writeRecord(
	record cacheRecord,
	data []byte,
) error {
	if !safeBlobName(record.Blob) {
		return errors.New("registry blob path is invalid")
	}
	target := filepath.Join(manager.root, filepath.FromSlash(record.Blob))
	if err := writeFileAtomic(target, data, 0o600); err != nil {
		return fmt.Errorf("cache registry blob: %w", err)
	}
	return nil
}

func (manager *Manager) statePath() string {
	return filepath.Join(manager.root, "state.json")
}

func cloneState(state cacheState) cacheState {
	clone := cacheState{
		CheckedAt:    state.CheckedAt,
		Feeds:        make(map[string]cacheRecord, len(state.Feeds)),
		Applications: make(map[string]cacheRecord, len(state.Applications)),
	}
	for reference, record := range state.Feeds {
		clone.Feeds[reference] = record
	}
	for reference, record := range state.Applications {
		clone.Applications[reference] = record
	}
	return clone
}

func sameRecords(left cacheState, right cacheState) bool {
	return reflect.DeepEqual(left.Feeds, right.Feeds) &&
		reflect.DeepEqual(left.Applications, right.Applications)
}

func safeBlobName(name string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	return cleaned == name &&
		strings.HasPrefix(name, "blobs/sha256-") &&
		!strings.Contains(name, "..")
}

type OCIResolver struct {
	client *auth.Client
}

func NewOCIResolver(client *http.Client) *OCIResolver {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	authClient := &auth.Client{
		Client: client,
		Cache:  auth.NewCache(),
	}
	authClient.SetUserAgent("pdparchitect-launcher")
	return &OCIResolver{client: authClient}
}

func (resolver *OCIResolver) Fetch(
	ctx context.Context,
	reference string,
) (Artifact, error) {
	parsed, err := registry.ParseReference(reference)
	if err != nil {
		return Artifact{}, fmt.Errorf(
			"parse OCI reference %q: %w",
			reference,
			err,
		)
	}
	if parsed.Reference == "" {
		return Artifact{}, fmt.Errorf(
			"OCI reference %q has no tag or digest",
			reference,
		)
	}
	repositoryName := parsed.Registry + "/" + parsed.Repository
	repository, err := remote.NewRepository(repositoryName)
	if err != nil {
		return Artifact{}, fmt.Errorf(
			"open OCI repository %q: %w",
			repositoryName,
			err,
		)
	}
	repository.Client = resolver.client
	descriptor, err := repository.Resolve(ctx, parsed.Reference)
	if err != nil {
		return Artifact{}, fmt.Errorf(
			"resolve OCI artifact %q: %w",
			reference,
			err,
		)
	}
	if descriptor.Size < 1 || descriptor.Size > maxManifestSize {
		return Artifact{}, fmt.Errorf(
			"OCI artifact %q manifest size %d is invalid",
			reference,
			descriptor.Size,
		)
	}
	manifestData, err := content.FetchAll(ctx, repository, descriptor)
	if err != nil {
		return Artifact{}, fmt.Errorf(
			"fetch OCI artifact %q manifest: %w",
			reference,
			err,
		)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return Artifact{}, fmt.Errorf(
			"decode OCI artifact %q manifest: %w",
			reference,
			err,
		)
	}
	if len(manifest.Layers) != 1 {
		return Artifact{}, fmt.Errorf(
			"OCI artifact %q must contain exactly one layer",
			reference,
		)
	}
	layer := manifest.Layers[0]
	limit := int64(maxApplicationBundle)
	if manifest.ArtifactType == FeedArtifactType {
		limit = maxFeedSize
	}
	if layer.Size < 1 || layer.Size > limit {
		return Artifact{}, fmt.Errorf(
			"OCI artifact %q layer size %d is invalid",
			reference,
			layer.Size,
		)
	}
	payload, err := content.FetchAll(ctx, repository, layer)
	if err != nil {
		return Artifact{}, fmt.Errorf(
			"fetch OCI artifact %q layer: %w",
			reference,
			err,
		)
	}
	var subjectDigest string
	if manifest.Subject != nil {
		subjectDigest = manifest.Subject.Digest.String()
	}
	return Artifact{
		Reference:      reference,
		ManifestDigest: descriptor.Digest.String(),
		ArtifactType:   manifest.ArtifactType,
		LayerType:      layer.MediaType,
		Repository:     repositoryName,
		SubjectDigest:  subjectDigest,
		Content:        payload,
	}, nil
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
	temporary, err := os.CreateTemp(directory, ".registry-*")
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
