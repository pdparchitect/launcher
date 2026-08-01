package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
	"github.com/pdparchitect/launcher/internal/imagecache"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
)

const (
	defaultPort                = 16902
	defaultRuntimeProbeTimeout = 3 * time.Second
)

type Runtime interface {
	Doctor(context.Context) (string, error)
	Pull(context.Context, string, string) error
	ResolveImage(context.Context, string) (launchruntime.LocalImage, error)
	DeleteImage(context.Context, string) error
	EnsureNetwork(context.Context, string) error
	DeleteNetwork(context.Context, string) error
	NetworkAttached(context.Context, string, string) (bool, error)
	NetworkInfo(
		context.Context,
		string,
		string,
	) (launchruntime.NetworkInfo, error)
	Create(context.Context, launchruntime.CreateRequest) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Remove(context.Context, string, string) error
	Status(context.Context, string) (launchruntime.Status, error)
	Stats(context.Context, string) (launchruntime.Metrics, error)
	RecentLogs(context.Context, string, int) (string, error)
	Logs(context.Context, string, bool) error
	Exec(context.Context, string, launchruntime.ExecOptions) error
}

type pullProgressRuntime interface {
	PullWithProgress(context.Context, string, string, func(string)) error
}

type mountDataRuntime interface {
	DeleteMountData(context.Context, string, catalog.Manifest) error
}

type PortAllocator interface {
	Allocate(int, map[int]struct{}) (int, error)
}

type Options struct {
	ID                  func() (string, error)
	Now                 func() time.Time
	CopyFiles           func(string, string) error
	Ports               PortAllocator
	Platform            string
	RuntimeName         string
	RuntimePath         string
	RuntimeProbeTimeout time.Duration
	ImageCache          *imagecache.Store
	HealthCheck         func(context.Context, string) error
}

type CreateOptions struct {
	CatalogID string
	Name      string
	Image     string
	Start     bool
	Progress  func(CreateProgress)
}

type DuplicateOptions struct {
	Name  string
	Start bool
}

type ExecOptions struct {
	Command []string
	Stdin   io.Reader
	TTY     bool
}

type CreateStage string

const (
	CreateStagePreparing CreateStage = "preparing"
	CreateStagePulling   CreateStage = "pulling"
	CreateStageCreating  CreateStage = "creating"
	CreateStageStarting  CreateStage = "starting"
	CreateStageReady     CreateStage = "ready"
)

type CreateProgress struct {
	Stage   CreateStage `json:"stage"`
	Message string      `json:"message"`
}

type UpdateStage string

const (
	UpdateStagePreparing UpdateStage = "preparing"
	UpdateStagePulling   UpdateStage = "pulling"
	UpdateStageStopping  UpdateStage = "stopping"
	UpdateStageReplacing UpdateStage = "replacing"
	UpdateStageStarting  UpdateStage = "starting"
	UpdateStageRestoring UpdateStage = "restoring"
	UpdateStageReady     UpdateStage = "ready"
)

type UpdateProgress struct {
	Stage   UpdateStage `json:"stage"`
	Message string      `json:"message"`
}

type View struct {
	domain.Instance
	CatalogSlug     string
	UpdateAvailable bool
	AvailableImage  string
	State           launchruntime.Status
	Metrics         launchruntime.Metrics
	MetricsError    string
	Uptime          time.Duration
}

type MountDetails struct {
	Name    string
	Target  string
	Storage string
	Source  string
}

type Details struct {
	View
	Files        string
	Mounts       []MountDetails
	Network      launchruntime.NetworkInfo
	NetworkError string
}

type CatalogEntry struct {
	ID          string                       `json:"id"`
	Slug        string                       `json:"slug"`
	Name        string                       `json:"name"`
	Publisher   string                       `json:"publisher"`
	Description string                       `json:"description"`
	Tags        []string                     `json:"tags"`
	Media       catalog.Media                `json:"media"`
	Image       string                       `json:"image"`
	Interfaces  map[string]catalog.Interface `json:"interfaces"`
	Memory      string                       `json:"memory,omitempty"`
}

type DoctorReport struct {
	Runtime    string `json:"runtime"`
	Version    string `json:"version"`
	Executable string `json:"executable"`
	DataRoot   string `json:"dataRoot"`
}

type Service struct {
	store        *store.Store
	runtime      Runtime
	catalogMutex sync.RWMutex
	manifests    map[string]catalog.Manifest
	// The catalogue in the order the registry published it, which the index
	// above cannot preserve.
	catalogue    []catalog.Manifest
	slugs        map[string]string
	options      Options
	imageCache   *imagecache.Store
	cleanupMutex sync.Mutex
}

func New(
	dataStore *store.Store,
	containerRuntime Runtime,
	manifests []catalog.Manifest,
	options Options,
) *Service {
	if options.ID == nil {
		options.ID = randomID
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Ports == nil {
		options.Ports = NetworkPortAllocator{}
	}
	if options.CopyFiles == nil {
		options.CopyFiles = copyDirectory
	}
	if options.Platform == "" {
		options.Platform = containerPlatform(runtime.GOARCH)
	}
	if options.RuntimeName == "" {
		options.RuntimeName = "docker"
	}
	if options.RuntimeProbeTimeout <= 0 {
		options.RuntimeProbeTimeout = defaultRuntimeProbeTimeout
	}
	if options.ImageCache == nil {
		options.ImageCache = imagecache.New(dataStore.Root())
	}
	if options.HealthCheck == nil {
		options.HealthCheck = waitForHealth
	}
	service := &Service{
		store: dataStore, runtime: containerRuntime,
		options: options, imageCache: options.ImageCache,
	}
	service.ReplaceCatalog(manifests)
	return service
}

func (service *Service) Catalog() []CatalogEntry {
	service.catalogMutex.RLock()
	defer service.catalogMutex.RUnlock()
	entries := make([]CatalogEntry, 0, len(service.catalogue))
	for _, manifest := range service.catalogue {
		entries = append(entries, CatalogEntry{
			ID: manifest.ID, Slug: manifest.Slug, Name: manifest.Name,
			Publisher: manifest.Publisher, Description: manifest.Description,
			Tags: manifest.Tags, Media: manifest.Media, Image: manifest.Image,
			Interfaces: manifest.Interfaces, Memory: manifest.Memory,
		})
	}
	return entries
}

func (service *Service) ReplaceCatalog(manifests []catalog.Manifest) {
	manifestIndex := make(map[string]catalog.Manifest, len(manifests))
	slugs := make(map[string]string, len(manifests))
	catalogue := make([]catalog.Manifest, len(manifests))
	copy(catalogue, manifests)
	for _, manifest := range manifests {
		manifestIndex[manifest.ID] = manifest
		slugs[manifest.Slug] = manifest.ID
	}
	service.catalogMutex.Lock()
	service.manifests = manifestIndex
	service.catalogue = catalogue
	service.slugs = slugs
	service.catalogMutex.Unlock()
}

func (service *Service) Doctor(ctx context.Context) (DoctorReport, error) {
	version, err := service.runtime.Doctor(ctx)
	if err != nil {
		return DoctorReport{}, err
	}
	runtimePath := service.options.RuntimePath
	if reporter, ok := service.runtime.(interface{ RuntimePath() string }); ok {
		if detectedPath := reporter.RuntimePath(); detectedPath != "" {
			runtimePath = detectedPath
		}
	}
	report := DoctorReport{
		Runtime: service.options.RuntimeName, Version: version,
		Executable: runtimePath,
		DataRoot:   service.store.Root(),
	}
	return report, nil
}

func (service *Service) Create(
	ctx context.Context,
	options CreateOptions,
) (domain.Instance, error) {
	options.Name = strings.TrimSpace(options.Name)
	if err := domain.ValidateName(options.Name); err != nil {
		return domain.Instance{}, err
	}
	options.CatalogID = strings.TrimSpace(options.CatalogID)
	if options.CatalogID == "" {
		return domain.Instance{}, errors.New("catalogue application is required")
	}
	manifest, exists := service.manifest(options.CatalogID)
	if !exists {
		return domain.Instance{}, fmt.Errorf(
			"catalogue entry %q is not built in", options.CatalogID,
		)
	}
	options.CatalogID = manifest.ID
	image := strings.TrimSpace(options.Image)
	if image == "" {
		image = manifest.Image
	}
	instances, err := service.store.List()
	if err != nil {
		return domain.Instance{}, err
	}
	interfaces, err := service.resolveInterfaces(manifest, instances, nil)
	if err != nil {
		return domain.Instance{}, err
	}
	id, err := service.options.ID()
	if err != nil {
		return domain.Instance{}, fmt.Errorf("generate agent ID: %w", err)
	}
	if !domain.ValidID(id) {
		return domain.Instance{}, errors.New(
			"generated agent ID must contain 32 lowercase hexadecimal characters",
		)
	}
	desiredState := domain.DesiredStopped
	if options.Start {
		desiredState = domain.DesiredRunning
	}
	instance := domain.Instance{
		ID: id, CatalogID: options.CatalogID, Name: options.Name, Image: image,
		ContainerName: "launcher-" + manifest.Slug + "-" + id[:12],
		Interfaces:    interfaces,
		DesiredState:  desiredState,
		CreatedAt:     service.options.Now().UTC(),
	}
	runtimeManifest := manifest
	runtimeManifest.Image = image
	instance.RuntimeManifest = &runtimeManifest
	options.report(CreateStagePreparing, "Preparing local agent storage")
	paths, err := service.store.Create(instance, manifest)
	if err != nil {
		return domain.Instance{}, err
	}
	created := false
	networkPrepared := false
	runtimeCreateAttempted := false
	defer func() {
		if !created {
			if networkPrepared {
				_ = service.runtime.DeleteNetwork(
					context.WithoutCancel(ctx), instance.ID,
				)
			}
			if runtimeCreateAttempted {
				if dataRuntime, ok := service.runtime.(mountDataRuntime); ok {
					_ = dataRuntime.DeleteMountData(
						context.WithoutCancel(ctx), instance.ID, manifest,
					)
				}
			}
			_ = service.store.Delete(instance.ID)
		}
	}()
	options.report(CreateStagePulling, "Pulling agent image")
	pull := service.runtime.Pull
	if progressRuntime, ok := service.runtime.(pullProgressRuntime); ok {
		pull = func(ctx context.Context, image string, platform string) error {
			return progressRuntime.PullWithProgress(
				ctx,
				image,
				platform,
				func(message string) {
					options.report(CreateStagePulling, message)
				},
			)
		}
	}
	if err := pull(ctx, image, service.options.Platform); err != nil {
		return domain.Instance{}, err
	}
	if err := service.trackImage(ctx, image); err != nil {
		return domain.Instance{}, err
	}
	options.report(CreateStageCreating, "Creating agent container")
	if err := service.runtime.EnsureNetwork(ctx, instance.ID); err != nil {
		return domain.Instance{}, fmt.Errorf("prepare agent network: %w", err)
	}
	networkPrepared = true
	runtimeCreateAttempted = true
	if err := service.runtime.Create(ctx, launchruntime.CreateRequest{
		InstanceID: instance.ID, ContainerName: instance.ContainerName,
		Network: launchruntime.ManagedNetworkName(instance.ID),
		Image:   instance.Image, Ports: runtimePorts(instance, manifest),
		Platform: service.options.Platform,
		Paths:    paths.Mounts, Manifest: manifest,
	}); err != nil {
		return domain.Instance{}, err
	}
	created = true
	if options.Start {
		options.report(CreateStageStarting, "Starting agent")
		if err := service.runtime.Start(ctx, instance.ContainerName); err != nil {
			instance.DesiredState = domain.DesiredStopped
			_ = service.store.Save(instance)
			return instance, err
		}
		if err := service.checkDeclaredHealth(ctx, instance, manifest); err != nil {
			return instance, fmt.Errorf("check created agent: %w", err)
		}
	}
	options.report(CreateStageReady, "Agent is ready")
	return instance, nil
}

func (options CreateOptions) report(stage CreateStage, message string) {
	if options.Progress != nil {
		options.Progress(CreateProgress{Stage: stage, Message: message})
	}
}

func (service *Service) List(ctx context.Context) ([]View, error) {
	views, _, err := service.ListWithIssues(ctx)
	return views, err
}

func (service *Service) ListWithIssues(
	ctx context.Context,
) ([]View, []store.Issue, error) {
	instances, issues, err := service.store.ListWithIssues()
	if err != nil {
		return nil, nil, err
	}
	views := make([]View, len(instances))
	var probes sync.WaitGroup
	probes.Add(len(instances))
	for index, instance := range instances {
		go func() {
			defer probes.Done()
			views[index] = service.probeView(ctx, instance)
		}()
	}
	probes.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	sort.Slice(views, func(left, right int) bool {
		return strings.ToLower(views[left].Name) < strings.ToLower(views[right].Name)
	})
	return views, issues, nil
}

func (service *Service) probeView(
	ctx context.Context,
	instance domain.Instance,
) View {
	probeCtx, cancel := context.WithTimeout(
		ctx,
		service.options.RuntimeProbeTimeout,
	)
	defer cancel()
	state, err := service.runtime.Status(probeCtx, instance.ContainerName)
	if err == nil {
		return service.view(probeCtx, instance, state)
	}
	state = launchruntime.StatusStopped
	if instance.DesiredState == domain.DesiredRunning {
		state = launchruntime.StatusRestarting
	}
	view := service.view(probeCtx, instance, state)
	view.MetricsError = fmt.Sprintf("inspect agent runtime: %v", err)
	return view
}

func (service *Service) Get(ctx context.Context, reference string) (View, error) {
	instance, err := service.store.Get(reference)
	if err != nil {
		return View{}, err
	}
	return service.probeView(ctx, instance), nil
}

func (service *Service) Details(
	ctx context.Context,
	reference string,
) (Details, error) {
	view, err := service.Get(ctx, reference)
	if err != nil {
		return Details{}, err
	}
	root, err := service.store.AgentRoot(view.ID)
	if err != nil {
		return Details{}, err
	}
	details := Details{View: view, Files: root}
	if manifest, manifestErr := service.runtimeManifest(view.Instance); manifestErr == nil {
		paths := service.store.Paths(view.ID, manifest)
		for _, mount := range manifest.Mounts {
			storage := mount.Storage
			if storage == "" {
				storage = catalog.MountStorageHost
			}
			details.Mounts = append(details.Mounts, MountDetails{
				Name: mount.Name, Target: mount.Target, Storage: storage,
				Source: paths.Mounts[mount.Name],
			})
		}
	}
	networkCtx, cancel := context.WithTimeout(
		ctx,
		service.options.RuntimeProbeTimeout,
	)
	defer cancel()
	details.Network, err = service.runtime.NetworkInfo(
		networkCtx,
		view.ContainerName,
		view.ID,
	)
	if err != nil {
		details.Network.Name = launchruntime.ManagedNetworkName(view.ID)
		details.NetworkError = err.Error()
	}
	return details, nil
}

func (service *Service) replacementContainerName(
	instance domain.Instance,
	slug string,
) (string, error) {
	updateID, err := service.options.ID()
	if err != nil {
		return "", fmt.Errorf("generate replacement ID: %w", err)
	}
	if !domain.ValidID(updateID) {
		return "", errors.New(
			"generated replacement ID must contain 32 lowercase hexadecimal characters",
		)
	}
	return fmt.Sprintf(
		"launcher-%s-%s-update-%s",
		slug,
		instance.ID[:12],
		updateID[:12],
	), nil
}

func (service *Service) runtimeManifest(
	instance domain.Instance,
) (catalog.Manifest, error) {
	if instance.RuntimeManifest != nil {
		return *instance.RuntimeManifest, nil
	}
	current, exists := service.manifest(instance.CatalogID)
	if !exists {
		return catalog.Manifest{}, fmt.Errorf(
			"catalogue entry %q is unavailable and the agent has no stored runtime manifest",
			instance.CatalogID,
		)
	}
	current.Image = instance.Image
	return current, nil
}

func (service *Service) manifest(identity string) (catalog.Manifest, bool) {
	service.catalogMutex.RLock()
	defer service.catalogMutex.RUnlock()
	if manifest, exists := service.manifests[identity]; exists {
		return manifest, true
	}
	canonical, exists := service.slugs[identity]
	if !exists {
		return catalog.Manifest{}, false
	}
	manifest, exists := service.manifests[canonical]
	return manifest, exists
}

func (service *Service) view(
	ctx context.Context,
	instance domain.Instance,
	state launchruntime.Status,
) View {
	view := View{Instance: instance, State: state}
	if manifest, exists := service.manifest(instance.CatalogID); exists {
		view.CatalogSlug = manifest.Slug
		if manifest.Image != "" && manifest.Image != instance.Image {
			view.UpdateAvailable = true
			view.AvailableImage = manifest.Image
		}
	}
	if state != launchruntime.StatusRunning {
		return view
	}
	metrics, err := service.runtime.Stats(ctx, instance.ContainerName)
	if err != nil {
		view.MetricsError = err.Error()
		return view
	}
	view.Metrics = metrics
	startedAt := metrics.StartedAt
	if startedAt.IsZero() {
		startedAt = instance.CreatedAt
	}
	if uptime := service.options.Now().UTC().Sub(startedAt); uptime > 0 {
		view.Uptime = uptime
	}
	return view
}

func (service *Service) Start(
	ctx context.Context,
	reference string,
) (domain.Instance, error) {
	instance, err := service.store.Get(reference)
	if err != nil {
		return domain.Instance{}, err
	}
	status, err := service.runtime.Status(ctx, instance.ContainerName)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("inspect agent before start: %w", err)
	}
	if status == launchruntime.StatusMissing {
		return service.recoverMissingRuntime(ctx, instance)
	}
	isolated, err := service.runtime.NetworkAttached(
		ctx,
		instance.ContainerName,
		instance.ID,
	)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("inspect agent network: %w", err)
	}
	if !isolated {
		return service.migrateNetworkAndStart(ctx, instance)
	}
	if err := service.runtime.Start(ctx, instance.ContainerName); err != nil {
		return domain.Instance{}, err
	}
	instance.DesiredState = domain.DesiredRunning
	return instance, service.store.Save(instance)
}

func (service *Service) recoverMissingRuntime(
	ctx context.Context,
	instance domain.Instance,
) (domain.Instance, error) {
	manifest, err := service.runtimeManifest(instance)
	if err != nil {
		return domain.Instance{}, err
	}
	if err := service.validateLegacyVolumeStorage(instance, manifest); err != nil {
		return domain.Instance{}, err
	}
	paths, err := service.store.EnsurePaths(instance.ID, manifest)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("prepare agent storage for recovery: %w", err)
	}
	if err := service.runtime.EnsureNetwork(ctx, instance.ID); err != nil {
		return domain.Instance{}, fmt.Errorf("prepare agent network for recovery: %w", err)
	}
	if err := service.createRuntimeContainer(ctx, instance, manifest, paths); err != nil {
		return domain.Instance{}, fmt.Errorf("recreate missing agent container: %w", err)
	}
	if err := service.runtime.Start(ctx, instance.ContainerName); err != nil {
		return domain.Instance{}, fmt.Errorf("start recovered agent container: %w", err)
	}
	if err := service.checkDeclaredHealth(ctx, instance, manifest); err != nil {
		return domain.Instance{}, fmt.Errorf("check recovered agent: %w", err)
	}
	instance.DesiredState = domain.DesiredRunning
	if instance.RuntimeManifest == nil {
		manifest.Image = instance.Image
		instance.RuntimeManifest = &manifest
	}
	if err := service.store.Save(instance); err != nil {
		return instance, fmt.Errorf("save recovered agent: %w", err)
	}
	return instance, nil
}

func (service *Service) migrateNetworkAndStart(
	ctx context.Context,
	instance domain.Instance,
) (domain.Instance, error) {
	manifest, err := service.runtimeManifest(instance)
	if err != nil {
		return domain.Instance{}, err
	}
	if err := service.validateLegacyVolumeStorage(instance, manifest); err != nil {
		return domain.Instance{}, err
	}
	paths, err := service.store.EnsurePaths(instance.ID, manifest)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("prepare agent storage: %w", err)
	}
	candidateName, err := service.replacementContainerName(instance, manifest.Slug)
	if err != nil {
		return domain.Instance{}, err
	}
	if err := service.runtime.EnsureNetwork(ctx, instance.ID); err != nil {
		return domain.Instance{}, fmt.Errorf("prepare agent network: %w", err)
	}
	if err := service.runtime.Stop(ctx, instance.ContainerName); err != nil {
		return domain.Instance{}, fmt.Errorf("stop agent for network migration: %w", err)
	}
	candidate := instance
	candidate.ContainerName = candidateName
	candidate.DesiredState = domain.DesiredRunning
	if candidate.RuntimeManifest == nil {
		candidate.RuntimeManifest = &manifest
	}
	candidateAttempted := false
	restore := func(action string, cause error) error {
		recoveryCtx := context.WithoutCancel(ctx)
		var recoveryErrors []error
		if candidateAttempted {
			if removeErr := service.runtime.Remove(
				recoveryCtx,
				candidate.ContainerName,
				instance.ID,
			); removeErr != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf(
					"remove network migration candidate: %w",
					removeErr,
				))
			}
		}
		if startErr := service.runtime.Start(
			recoveryCtx,
			instance.ContainerName,
		); startErr != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf(
				"restart previous container: %w",
				startErr,
			))
		}
		if len(recoveryErrors) == 0 {
			return fmt.Errorf("%s: %w; previous container restored", action, cause)
		}
		return joinedUpdateError(action, cause, recoveryErrors...)
	}
	candidateAttempted = true
	if err := service.createRuntimeContainer(ctx, candidate, manifest, paths); err != nil {
		return domain.Instance{}, restore("migrate agent network", err)
	}
	if err := service.runtime.Start(ctx, candidate.ContainerName); err != nil {
		return domain.Instance{}, restore("start migrated agent", err)
	}
	if err := service.checkDeclaredHealth(ctx, candidate, manifest); err != nil {
		return domain.Instance{}, restore("check migrated agent", err)
	}
	if err := service.store.Save(candidate); err != nil {
		return domain.Instance{}, restore("save migrated agent", err)
	}
	if err := service.runtime.Remove(
		context.WithoutCancel(ctx),
		instance.ContainerName,
		instance.ID,
	); err != nil {
		return candidate, fmt.Errorf(
			"agent network migrated, but remove previous container: %w",
			err,
		)
	}
	return candidate, nil
}

func (service *Service) Stop(
	ctx context.Context,
	reference string,
) (domain.Instance, error) {
	instance, err := service.store.Get(reference)
	if err != nil {
		return domain.Instance{}, err
	}
	if err := service.runtime.Stop(ctx, instance.ContainerName); err != nil {
		return domain.Instance{}, err
	}
	instance.DesiredState = domain.DesiredStopped
	return instance, service.store.Save(instance)
}

func (service *Service) Update(
	ctx context.Context,
	reference string,
) (domain.Instance, error) {
	return service.UpdateWithProgress(ctx, reference, nil)
}

func (service *Service) UpdateWithProgress(
	ctx context.Context,
	reference string,
	progress func(UpdateProgress),
) (domain.Instance, error) {
	report := func(stage UpdateStage, message string) {
		if progress != nil {
			progress(UpdateProgress{Stage: stage, Message: message})
		}
	}
	report(UpdateStagePreparing, "Checking the current agent")
	instance, err := service.store.Get(reference)
	if err != nil {
		return domain.Instance{}, err
	}
	manifest, exists := service.manifest(instance.CatalogID)
	if !exists {
		return domain.Instance{}, fmt.Errorf(
			"catalogue entry %q is not built in", instance.CatalogID,
		)
	}
	targetImage := strings.TrimSpace(manifest.Image)
	if targetImage == "" {
		return domain.Instance{}, errors.New("catalogue image is empty")
	}
	if targetImage == instance.Image {
		report(UpdateStageReady, "Agent is already up to date")
		return instance, nil
	}
	if instance.RuntimeManifest != nil {
		if err := validateRuntimeTransition(*instance.RuntimeManifest, manifest); err != nil {
			return domain.Instance{}, err
		}
	} else if err := service.validateLegacyVolumeStorage(instance, manifest); err != nil {
		return domain.Instance{}, err
	}
	// Agents installed before image tracking have no ledger entry. Recording
	// the current image here lets the reconciler remove it after this update.
	_ = service.trackImage(ctx, instance.Image)
	status, err := service.runtime.Status(ctx, instance.ContainerName)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("inspect agent before update: %w", err)
	}
	shouldStart := instance.DesiredState == domain.DesiredRunning ||
		status == launchruntime.StatusRunning ||
		status == launchruntime.StatusRestarting
	report(UpdateStagePulling, "Pulling the updated agent image")
	pull := service.runtime.Pull
	if progressRuntime, ok := service.runtime.(pullProgressRuntime); ok {
		pull = func(ctx context.Context, image string, platform string) error {
			return progressRuntime.PullWithProgress(
				ctx,
				image,
				platform,
				func(message string) {
					report(UpdateStagePulling, message)
				},
			)
		}
	}
	if err := pull(ctx, targetImage, service.options.Platform); err != nil {
		return domain.Instance{}, fmt.Errorf("pull agent update: %w", err)
	}
	if err := service.trackImage(ctx, targetImage); err != nil {
		return domain.Instance{}, err
	}
	if err := service.runtime.EnsureNetwork(ctx, instance.ID); err != nil {
		return domain.Instance{}, fmt.Errorf("prepare agent network: %w", err)
	}
	paths, err := service.store.EnsurePaths(instance.ID, manifest)
	if err != nil {
		return domain.Instance{}, fmt.Errorf("prepare updated agent storage: %w", err)
	}
	instances, err := service.store.List()
	if err != nil {
		return domain.Instance{}, err
	}
	updated := instance
	updated.Image = targetImage
	updated.ContainerName, err = service.replacementContainerName(
		instance,
		manifest.Slug,
	)
	if err != nil {
		return domain.Instance{}, err
	}
	updatedManifest := manifest
	updatedManifest.Image = targetImage
	updated.RuntimeManifest = &updatedManifest
	requestedPorts := make(map[string]int, len(instance.Interfaces))
	for id, resolved := range instance.Interfaces {
		requestedPorts[id] = resolved.Port
	}
	oldExists := status != launchruntime.StatusMissing
	oldActive := status == launchruntime.StatusRunning ||
		status == launchruntime.StatusRestarting ||
		status == launchruntime.StatusPaused
	candidateAttempted := false
	restorePrevious := func(action string, cause error) error {
		report(UpdateStageRestoring, "Update failed; restoring the previous runtime")
		recoveryCtx := context.WithoutCancel(ctx)
		var recoveryErrors []error
		if candidateAttempted {
			if removeErr := service.runtime.Remove(
				recoveryCtx,
				updated.ContainerName,
				instance.ID,
			); removeErr != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf(
					"remove failed update candidate: %w",
					removeErr,
				))
			}
		}
		if oldExists && shouldStart {
			if startErr := service.runtime.Start(
				recoveryCtx,
				instance.ContainerName,
			); startErr != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf(
					"restart previous container: %w",
					startErr,
				))
			}
		}
		if len(recoveryErrors) == 0 && oldExists {
			return fmt.Errorf("%s: %w; previous container restored", action, cause)
		}
		return joinedUpdateError(action, cause, recoveryErrors...)
	}
	if status == launchruntime.StatusRunning ||
		status == launchruntime.StatusRestarting ||
		status == launchruntime.StatusPaused {
		report(UpdateStageStopping, "Stopping the current agent")
		if err := service.runtime.Stop(ctx, instance.ContainerName); err != nil {
			return domain.Instance{}, fmt.Errorf("stop agent for update: %w", err)
		}
	}
	updated.Interfaces, err = service.resolveInterfaces(
		manifest,
		instancesExcept(instances, instance.ID),
		requestedPorts,
	)
	if err != nil {
		if oldExists && (oldActive || shouldStart) {
			return domain.Instance{}, restorePrevious(
				"resolve updated interfaces",
				err,
			)
		}
		return domain.Instance{}, fmt.Errorf("resolve updated interfaces: %w", err)
	}
	if shouldStart {
		updated.DesiredState = domain.DesiredRunning
	}
	report(UpdateStageReplacing, "Creating the updated runtime candidate")
	candidateAttempted = true
	if err := service.createRuntimeContainer(ctx, updated, manifest, paths); err != nil {
		return domain.Instance{}, restorePrevious(
			"create updated agent container",
			err,
		)
	}
	if shouldStart {
		report(UpdateStageStarting, "Starting and checking the updated agent")
		if err := service.runtime.Start(ctx, updated.ContainerName); err != nil {
			return domain.Instance{}, restorePrevious("start updated agent", err)
		}
		if err := service.checkDeclaredHealth(ctx, updated, manifest); err != nil {
			return domain.Instance{}, restorePrevious("check updated agent", err)
		}
	}
	if err := service.store.Save(updated); err != nil {
		return domain.Instance{}, restorePrevious("save updated agent", err)
	}
	if oldExists {
		if err := service.runtime.Remove(
			context.WithoutCancel(ctx),
			instance.ContainerName,
			instance.ID,
		); err != nil {
			report(
				UpdateStageRestoring,
				"Update cleanup failed; restoring the previous runtime",
			)
			recoveryCtx := context.WithoutCancel(ctx)
			if saveErr := service.store.Save(instance); saveErr != nil {
				return updated, errors.Join(
					fmt.Errorf("remove previous container after update: %w", err),
					fmt.Errorf("restore previous agent metadata: %w", saveErr),
				)
			}
			var recoveryErrors []error
			if removeErr := service.runtime.Remove(
				recoveryCtx,
				updated.ContainerName,
				instance.ID,
			); removeErr != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf(
					"remove committed update candidate: %w",
					removeErr,
				))
			}
			if shouldStart {
				if startErr := service.runtime.Start(
					recoveryCtx,
					instance.ContainerName,
				); startErr != nil {
					recoveryErrors = append(recoveryErrors, fmt.Errorf(
						"restart previous container: %w",
						startErr,
					))
				}
			}
			return domain.Instance{}, joinedUpdateError(
				"remove previous container after update",
				err,
				recoveryErrors...,
			)
		}
	}
	report(UpdateStageReady, "Agent update is ready")
	return updated, nil
}

func (service *Service) createRuntimeContainer(
	ctx context.Context,
	instance domain.Instance,
	manifest catalog.Manifest,
	paths store.Paths,
) error {
	return service.runtime.Create(ctx, launchruntime.CreateRequest{
		InstanceID: instance.ID, ContainerName: instance.ContainerName,
		Network: launchruntime.ManagedNetworkName(instance.ID),
		Image:   instance.Image, Ports: runtimePorts(instance, manifest),
		Platform: service.options.Platform,
		Paths:    paths.Mounts, Manifest: manifest,
	})
}

func (service *Service) resolveInterfaces(
	manifest catalog.Manifest,
	instances []domain.Instance,
	requested map[string]int,
) (map[string]domain.Interface, error) {
	usedPorts := make(map[int]struct{})
	for _, instance := range instances {
		for _, resolved := range instance.Interfaces {
			usedPorts[resolved.Port] = struct{}{}
		}
	}
	ids := make([]string, 0, len(manifest.Interfaces))
	for id := range manifest.Interfaces {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(left int, right int) bool {
		leftRequested := requested != nil && requested[ids[left]] != 0
		rightRequested := requested != nil && requested[ids[right]] != 0
		if leftRequested != rightRequested {
			return leftRequested
		}
		return ids[left] < ids[right]
	})
	hostPorts := make(map[int]int)
	resolved := make(map[string]domain.Interface, len(ids))
	for _, id := range ids {
		definition := manifest.Interfaces[id]
		hostPort, exists := hostPorts[definition.Port]
		if !exists {
			var requestedPort int
			if requested != nil {
				requestedPort = requested[id]
			}
			var err error
			hostPort, err = service.options.Ports.Allocate(requestedPort, usedPorts)
			if err != nil {
				return nil, fmt.Errorf("allocate interface %q: %w", id, err)
			}
			hostPorts[definition.Port] = hostPort
			usedPorts[hostPort] = struct{}{}
		}
		resolved[id] = domain.Interface{
			Kind: definition.Kind,
			Port: hostPort,
			Path: definition.Path,
		}
	}
	return resolved, nil
}

func runtimePorts(
	instance domain.Instance,
	manifest catalog.Manifest,
) map[int]int {
	ports := make(map[int]int)
	for id, definition := range manifest.Interfaces {
		resolved, exists := instance.Interfaces[id]
		if exists {
			ports[definition.Port] = resolved.Port
		}
	}
	return ports
}

func validateRuntimeTransition(
	previous catalog.Manifest,
	target catalog.Manifest,
) error {
	previousMounts := make(map[string]catalog.Mount, len(previous.Mounts))
	for _, mount := range previous.Mounts {
		previousMounts[mount.Name] = mount
	}
	for _, mount := range target.Mounts {
		old, exists := previousMounts[mount.Name]
		if !exists {
			continue
		}
		oldStorage := old.Storage
		if oldStorage == "" {
			oldStorage = catalog.MountStorageHost
		}
		newStorage := mount.Storage
		if newStorage == "" {
			newStorage = catalog.MountStorageHost
		}
		if oldStorage != newStorage {
			return fmt.Errorf(
				"update changes mount %q storage from %s to %s; migrate its data before updating",
				mount.Name,
				oldStorage,
				newStorage,
			)
		}
	}
	return nil
}

func (service *Service) validateLegacyVolumeStorage(
	instance domain.Instance,
	manifest catalog.Manifest,
) error {
	if instance.RuntimeManifest != nil {
		return nil
	}
	conflicts, err := service.store.ExistingHostPathsForVolumes(
		instance.ID,
		manifest,
	)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}
	return fmt.Errorf(
		"legacy agent has host data for mount(s) %s now declared as runtime volumes; migrate that data before replacing the runtime",
		strings.Join(conflicts, ", "),
	)
}

func instancesExcept(
	instances []domain.Instance,
	id string,
) []domain.Instance {
	filtered := make([]domain.Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.ID != id {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

func (service *Service) Rename(
	ctx context.Context,
	reference string,
	name string,
) (domain.Instance, error) {
	if err := ctx.Err(); err != nil {
		return domain.Instance{}, err
	}
	instance, err := service.store.Get(reference)
	if err != nil {
		return domain.Instance{}, err
	}
	instance.Name = strings.TrimSpace(name)
	if err := domain.ValidateName(instance.Name); err != nil {
		return domain.Instance{}, err
	}
	if err := service.store.Save(instance); err != nil {
		return domain.Instance{}, err
	}
	return instance, nil
}

func (service *Service) RecentLogs(
	ctx context.Context,
	reference string,
	lines int,
) (string, error) {
	if lines < 1 || lines > 1000 {
		return "", errors.New("log line count must be between 1 and 1000")
	}
	instance, err := service.store.Get(reference)
	if err != nil {
		return "", err
	}
	return service.runtime.RecentLogs(ctx, instance.ContainerName, lines)
}

func (service *Service) AgentFiles(
	ctx context.Context,
	reference string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	instance, err := service.store.Get(reference)
	if err != nil {
		return "", err
	}
	return service.store.AgentRoot(instance.ID)
}

func (service *Service) Delete(ctx context.Context, reference string) error {
	instance, err := service.store.Get(reference)
	if err != nil {
		return err
	}
	// Deletion must not depend on the optional cleanup ledger, but recording the
	// image first makes pre-ledger installations eligible for later cleanup.
	_ = service.trackImage(ctx, instance.Image)
	if err := service.runtime.Remove(
		ctx, instance.ContainerName, instance.ID,
	); err != nil {
		return err
	}
	if err := service.runtime.DeleteNetwork(ctx, instance.ID); err != nil {
		return err
	}
	manifest, manifestErr := service.runtimeManifest(instance)
	if manifestErr == nil {
		if dataRuntime, ok := service.runtime.(mountDataRuntime); ok {
			if err := dataRuntime.DeleteMountData(
				ctx, instance.ID, manifest,
			); err != nil {
				return err
			}
		}
	}
	return service.store.Delete(instance.ID)
}

func (service *Service) Logs(
	ctx context.Context,
	reference string,
	follow bool,
) error {
	instance, err := service.store.Get(reference)
	if err != nil {
		return err
	}
	return service.runtime.Logs(ctx, instance.ContainerName, follow)
}

func (service *Service) Exec(
	ctx context.Context,
	reference string,
	options ExecOptions,
) error {
	if len(options.Command) == 0 || strings.TrimSpace(options.Command[0]) == "" {
		return errors.New("agent command is required")
	}
	instance, err := service.store.Get(reference)
	if err != nil {
		return err
	}
	status, err := service.runtime.Status(ctx, instance.ContainerName)
	if err != nil {
		return fmt.Errorf("inspect agent before exec: %w", err)
	}
	if status != launchruntime.StatusRunning {
		return fmt.Errorf(
			"%s is %s; exec requires a running agent",
			instance.Name,
			status,
		)
	}
	return service.runtime.Exec(
		ctx,
		instance.ContainerName,
		launchruntime.ExecOptions{
			Command: options.Command,
			Stdin:   options.Stdin,
			TTY:     options.TTY,
		},
	)
}

type NetworkPortAllocator struct{}

func (NetworkPortAllocator) Allocate(
	preferred int,
	used map[int]struct{},
) (int, error) {
	if preferred != 0 {
		if preferred < 1 || preferred > 65535 {
			return 0, errors.New("port must be between 1 and 65535")
		}
		if _, exists := used[preferred]; exists || !portAvailable(preferred) {
			return 0, fmt.Errorf("port %d is already in use", preferred)
		}
		return preferred, nil
	}
	for port := defaultPort; port <= 65535; port++ {
		if _, exists := used[port]; !exists && portAvailable(port) {
			return port, nil
		}
	}
	return 0, errors.New("no available local port found")
}

func portAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func randomID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func containerPlatform(architecture string) string {
	switch architecture {
	case "amd64":
		return "linux/amd64"
	case "arm64":
		return "linux/arm64"
	default:
		return ""
	}
}
