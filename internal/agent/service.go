package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
)

const (
	defaultPort                = 16902
	defaultCatalogID           = "370a2228-322d-4089-846b-62fb8c15d154"
	defaultRuntimeProbeTimeout = 3 * time.Second
)

type Runtime interface {
	Doctor(context.Context) (string, error)
	Pull(context.Context, string, string) error
	Create(context.Context, launchruntime.CreateRequest) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Remove(context.Context, string, string) error
	Status(context.Context, string) (launchruntime.Status, error)
	Stats(context.Context, string) (launchruntime.Metrics, error)
	RecentLogs(context.Context, string, int) (string, error)
	Logs(context.Context, string, bool) error
}

type pullProgressRuntime interface {
	PullWithProgress(context.Context, string, string, func(string)) error
}

type PortAllocator interface {
	Allocate(int, map[int]struct{}) (int, error)
}

type Options struct {
	ID                  func() (string, error)
	Now                 func() time.Time
	Ports               PortAllocator
	Platform            string
	RuntimeName         string
	RuntimePath         string
	DefaultCatalogID    string
	RuntimeProbeTimeout time.Duration
}

type CreateOptions struct {
	CatalogID string
	Name      string
	Image     string
	Port      int
	Start     bool
	Progress  func(CreateProgress)
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

type CatalogEntry struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	Publisher   string        `json:"publisher"`
	Description string        `json:"description"`
	Tags        []string      `json:"tags"`
	Media       catalog.Media `json:"media"`
	Image       string        `json:"image"`
	Viewer      string        `json:"viewer"`
	Memory      string        `json:"memory,omitempty"`
}

type DoctorReport struct {
	Runtime      string `json:"runtime"`
	Version      string `json:"version"`
	Executable   string `json:"executable"`
	DataRoot     string `json:"dataRoot"`
	DefaultImage string `json:"defaultImage"`
}

type Service struct {
	store        *store.Store
	runtime      Runtime
	catalogMutex sync.RWMutex
	manifests    map[string]catalog.Manifest
	slugs        map[string]string
	options      Options
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
	if options.Platform == "" {
		options.Platform = containerPlatform(runtime.GOARCH)
	}
	if options.RuntimeName == "" {
		options.RuntimeName = "docker"
	}
	if options.RuntimeProbeTimeout <= 0 {
		options.RuntimeProbeTimeout = defaultRuntimeProbeTimeout
	}
	if options.DefaultCatalogID == "" {
		options.DefaultCatalogID = defaultCatalogID
	}
	service := &Service{
		store: dataStore, runtime: containerRuntime,
		options: options,
	}
	service.ReplaceCatalog(manifests)
	return service
}

func (service *Service) Catalog() []CatalogEntry {
	service.catalogMutex.RLock()
	defer service.catalogMutex.RUnlock()
	entries := make([]CatalogEntry, 0, len(service.manifests))
	for _, manifest := range service.manifests {
		entries = append(entries, CatalogEntry{
			ID: manifest.ID, Slug: manifest.Slug, Name: manifest.Name,
			Publisher: manifest.Publisher, Description: manifest.Description,
			Tags: manifest.Tags, Media: manifest.Media, Image: manifest.Image,
			Viewer: manifest.Viewer, Memory: manifest.Memory,
		})
	}
	sort.Slice(entries, func(left, right int) bool {
		return strings.ToLower(entries[left].Name) <
			strings.ToLower(entries[right].Name)
	})
	return entries
}

func (service *Service) ReplaceCatalog(manifests []catalog.Manifest) {
	manifestIndex := make(map[string]catalog.Manifest, len(manifests))
	slugs := make(map[string]string, len(manifests))
	for _, manifest := range manifests {
		manifestIndex[manifest.ID] = manifest
		slugs[manifest.Slug] = manifest.ID
	}
	service.catalogMutex.Lock()
	service.manifests = manifestIndex
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
	if manifest, exists := service.manifest(service.options.DefaultCatalogID); exists {
		report.DefaultImage = manifest.Image
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
	if options.CatalogID == "" {
		options.CatalogID = service.options.DefaultCatalogID
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
	usedPorts := make(map[int]struct{}, len(instances))
	for _, instance := range instances {
		usedPorts[instance.Port] = struct{}{}
	}
	port, err := service.options.Ports.Allocate(options.Port, usedPorts)
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
		Port:          port, DesiredState: desiredState, CreatedAt: service.options.Now().UTC(),
	}
	options.report(CreateStagePreparing, "Preparing local agent storage")
	paths, err := service.store.Create(instance, manifest)
	if err != nil {
		return domain.Instance{}, err
	}
	created := false
	defer func() {
		if !created {
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
	options.report(CreateStageCreating, "Creating agent container")
	if err := service.runtime.Create(ctx, launchruntime.CreateRequest{
		InstanceID: instance.ID, ContainerName: instance.ContainerName,
		Image: instance.Image, Port: instance.Port, Platform: service.options.Platform,
		Paths: paths.Mounts, Manifest: manifest,
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
	instances, err := service.store.List()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	sort.Slice(views, func(left, right int) bool {
		return strings.ToLower(views[left].Name) < strings.ToLower(views[right].Name)
	})
	return views, nil
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
	if err := service.runtime.Start(ctx, instance.ContainerName); err != nil {
		return domain.Instance{}, err
	}
	instance.DesiredState = domain.DesiredRunning
	return instance, service.store.Save(instance)
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
	if status == launchruntime.StatusRunning ||
		status == launchruntime.StatusRestarting ||
		status == launchruntime.StatusPaused {
		report(UpdateStageStopping, "Stopping the current agent")
		if err := service.runtime.Stop(ctx, instance.ContainerName); err != nil {
			return domain.Instance{}, fmt.Errorf("stop agent for update: %w", err)
		}
	}
	report(UpdateStageReplacing, "Replacing the runtime container")
	if err := service.runtime.Remove(
		ctx,
		instance.ContainerName,
		instance.ID,
	); err != nil {
		return domain.Instance{}, fmt.Errorf("remove old agent container: %w", err)
	}
	paths := service.store.Paths(instance.ID, manifest)
	updated := instance
	updated.Image = targetImage
	if shouldStart {
		updated.DesiredState = domain.DesiredRunning
	}
	if err := service.createRuntimeContainer(ctx, updated, manifest, paths); err != nil {
		report(
			UpdateStageRestoring,
			"Update failed; restoring the previous runtime container",
		)
		rollbackErr := service.restoreRuntimeContainer(
			ctx,
			instance,
			manifest,
			paths,
			shouldStart,
		)
		if rollbackErr != nil {
			return domain.Instance{}, fmt.Errorf(
				"create updated agent container: %w; restoring previous container: %v",
				err,
				rollbackErr,
			)
		}
		return domain.Instance{}, fmt.Errorf(
			"create updated agent container: %w; previous container restored",
			err,
		)
	}
	if err := service.store.Save(updated); err != nil {
		return domain.Instance{}, fmt.Errorf("save updated agent image: %w", err)
	}
	if shouldStart {
		report(UpdateStageStarting, "Starting the updated agent")
		if err := service.runtime.Start(ctx, updated.ContainerName); err != nil {
			updated.DesiredState = domain.DesiredStopped
			_ = service.store.Save(updated)
			return updated, fmt.Errorf("start updated agent: %w", err)
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
		Image: instance.Image, Port: instance.Port, Platform: service.options.Platform,
		Paths: paths.Mounts, Manifest: manifest,
	})
}

func (service *Service) restoreRuntimeContainer(
	ctx context.Context,
	instance domain.Instance,
	manifest catalog.Manifest,
	paths store.Paths,
	start bool,
) error {
	if err := service.createRuntimeContainer(ctx, instance, manifest, paths); err != nil {
		return err
	}
	if start {
		return service.runtime.Start(ctx, instance.ContainerName)
	}
	return nil
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
	if err := service.runtime.Remove(
		ctx, instance.ContainerName, instance.ID,
	); err != nil {
		return err
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
