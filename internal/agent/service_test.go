package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
)

const (
	testGhostCatalogID = "370a2228-322d-4089-846b-62fb8c15d154"
	testBuzzCatalogID  = "4398d440-4e4f-4137-b25e-303bfeb2a276"
)

func TestCreateStartStopAndDelete(t *testing.T) {
	containerRuntime := &fakeRuntime{status: launchruntime.StatusCreated}
	service := newTestService(t, containerRuntime)
	instance, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Image: "pantalk/ghost:test",
		Start: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if instance.CatalogID != testGhostCatalogID ||
		instance.DesiredState != domain.DesiredRunning {
		t.Fatalf("instance = %#v", instance)
	}
	if !containerRuntime.pullCalled || !containerRuntime.createCalled ||
		!containerRuntime.startCalled {
		t.Fatalf("runtime calls = %#v", containerRuntime)
	}
	if containerRuntime.pullPlatform != "linux/amd64" {
		t.Fatalf("pull platform = %q, want linux/amd64", containerRuntime.pullPlatform)
	}
	if _, err := service.Stop(t.Context(), "Ada"); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if _, err := service.Start(t.Context(), instance.ID); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := service.Delete(t.Context(), "Ada"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.store.Get(instance.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted instance error = %v", err)
	}
}

func TestCreateRollsBackWhenRuntimeCreateFails(t *testing.T) {
	service := newTestService(t, &fakeRuntime{
		createErr: errors.New("create failed"),
	})
	if _, err := service.Create(t.Context(), CreateOptions{Name: "Ada"}); err == nil {
		t.Fatal("Create() error = nil")
	}
	instances, err := service.store.List()
	if err != nil || len(instances) != 0 {
		t.Fatalf("instances after rollback = %#v, %v", instances, err)
	}
}

func TestCreateReportsLifecycleProgress(t *testing.T) {
	service := newTestService(t, &fakeRuntime{})
	var stages []CreateStage

	_, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Start: true,
		Progress: func(progress CreateProgress) {
			stages = append(stages, progress.Stage)
		},
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	want := []CreateStage{
		CreateStagePreparing,
		CreateStagePulling,
		CreateStageCreating,
		CreateStageStarting,
		CreateStageReady,
	}
	if len(stages) != len(want) {
		t.Fatalf("stages = %#v, want %#v", stages, want)
	}
	for index := range want {
		if stages[index] != want[index] {
			t.Fatalf("stages = %#v, want %#v", stages, want)
		}
	}
}

func TestCreateReportsRuntimePullOutput(t *testing.T) {
	service := newTestService(t, &fakeRuntime{
		pullProgress: []string{
			"downloading layer 1",
			"extracting layer 1",
		},
	})
	var messages []string

	_, err := service.Create(t.Context(), CreateOptions{
		Name: "Ada",
		Progress: func(progress CreateProgress) {
			if progress.Stage == CreateStagePulling {
				messages = append(messages, progress.Message)
			}
		},
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, expected := range []string{
		"Pulling agent image",
		"downloading layer 1",
		"extracting layer 1",
	} {
		if !containsText(messages, expected) {
			t.Fatalf("pull messages = %#v, missing %q", messages, expected)
		}
	}
}

func TestCatalogIncludesEveryManifestAndPresentationMetadata(t *testing.T) {
	containerRuntime := &fakeRuntime{}
	service := newTestService(t, containerRuntime)
	ghost := service.manifests[testGhostCatalogID]
	buzz := ghost
	buzz.ID = testBuzzCatalogID
	buzz.Slug = "buzznode"
	buzz.Name = "Buzznode"
	buzz.Description = "A local development agent."
	buzz.Tags = []string{"DEVELOPMENT", "TERMINAL"}
	service = New(
		store.New(t.TempDir()),
		containerRuntime,
		[]catalog.Manifest{ghost, buzz},
		Options{DefaultCatalogID: testGhostCatalogID},
	)

	entries := service.Catalog()

	if len(entries) != 2 ||
		entries[0].ID != testBuzzCatalogID ||
		entries[0].Slug != "buzznode" ||
		entries[1].ID != testGhostCatalogID ||
		entries[1].Slug != "pantalk-ghost" {
		t.Fatalf("Catalog() = %#v", entries)
	}
	if entries[1].Description != ghost.Description ||
		entries[1].Memory != ghost.Memory ||
		len(entries[1].Media.Screenshots) != 1 {
		t.Fatalf("Ghost entry = %#v", entries[1])
	}
}

func TestReplaceCatalogChangesFutureCatalogueLookups(t *testing.T) {
	service := newTestService(t, &fakeRuntime{})
	updated := service.manifests[testGhostCatalogID]
	updated.Name = "Updated Ghost"
	updated.Image = "pantalk/ghost:updated"

	service.ReplaceCatalog([]catalog.Manifest{updated})

	entries := service.Catalog()
	if len(entries) != 1 ||
		entries[0].Name != "Updated Ghost" ||
		entries[0].Image != "pantalk/ghost:updated" {
		t.Fatalf("Catalog() = %#v", entries)
	}
	manifest, exists := service.manifest("pantalk-ghost")
	if !exists || manifest.Image != "pantalk/ghost:updated" {
		t.Fatalf("manifest() = %#v, %v", manifest, exists)
	}
}

func TestCreateResolvesSlugToStableCatalogueID(t *testing.T) {
	service := newTestService(t, &fakeRuntime{})

	instance, err := service.Create(t.Context(), CreateOptions{
		CatalogID: "pantalk-ghost",
		Name:      "Ada",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if instance.CatalogID != testGhostCatalogID {
		t.Fatalf("CatalogID = %q", instance.CatalogID)
	}
}

func TestListIncludesLiveRuntimeMetrics(t *testing.T) {
	containerRuntime := &fakeRuntime{
		status: launchruntime.StatusRunning,
		metrics: launchruntime.Metrics{
			CPUPercent:      1.25,
			CPUAvailable:    true,
			MemoryPercent:   12.5,
			MemoryAvailable: true,
			StartedAt: time.Date(
				2026, 7, 27, 11, 55, 0, 0, time.UTC,
			),
		},
	}
	service := newTestService(t, containerRuntime)
	if _, err := service.Create(
		t.Context(),
		CreateOptions{Name: "Ada"},
	); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	views, err := service.List(t.Context())

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(views) != 1 ||
		views[0].Metrics.CPUPercent != 1.25 ||
		views[0].Metrics.MemoryPercent != 12.5 ||
		views[0].Uptime != 5*time.Minute {
		t.Fatalf("List() = %#v", views)
	}
}

func TestGetReportsCatalogueImageUpdate(t *testing.T) {
	service := newTestService(t, &fakeRuntime{})
	instance, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Image: "pantalk/ghost:old",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	view, err := service.Get(t.Context(), instance.ID)

	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !view.UpdateAvailable ||
		view.AvailableImage != "pantalk/ghost:default" {
		t.Fatalf("update information = %#v", view)
	}
}

func TestUpdateRecreatesRunningAgentWithoutReplacingItsStorage(t *testing.T) {
	containerRuntime := &fakeRuntime{}
	service := newTestService(t, containerRuntime)
	instance, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Image: "pantalk/ghost:old",
		Start: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	containerRuntime.resetCalls()

	updated, err := service.Update(t.Context(), instance.ID)

	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	wantCalls := []string{"pull", "stop", "remove", "create", "start"}
	if strings.Join(containerRuntime.calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("runtime calls = %#v, want %#v", containerRuntime.calls, wantCalls)
	}
	if updated.Image != "pantalk/ghost:default" ||
		updated.ID != instance.ID ||
		updated.ContainerName != instance.ContainerName ||
		updated.Port != instance.Port ||
		updated.DesiredState != domain.DesiredRunning {
		t.Fatalf("Update() = %#v", updated)
	}
	if containerRuntime.pullImage != "pantalk/ghost:default" ||
		containerRuntime.pullPlatform != "linux/amd64" ||
		containerRuntime.createRequest.Image != "pantalk/ghost:default" ||
		containerRuntime.createRequest.Paths["workspace"] == "" {
		t.Fatalf("runtime update = %#v", containerRuntime)
	}
	stored, err := service.store.Get(instance.ID)
	if err != nil || stored.Image != "pantalk/ghost:default" {
		t.Fatalf("stored instance = %#v, %v", stored, err)
	}
	view, err := service.Get(t.Context(), instance.ID)
	if err != nil || view.UpdateAvailable {
		t.Fatalf("updated view = %#v, %v", view, err)
	}
}

func TestUpdateLeavesStoppedAgentStopped(t *testing.T) {
	containerRuntime := &fakeRuntime{status: launchruntime.StatusStopped}
	service := newTestService(t, containerRuntime)
	instance, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Image: "pantalk/ghost:old",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	containerRuntime.resetCalls()

	updated, err := service.Update(t.Context(), instance.ID)

	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	wantCalls := []string{"pull", "remove", "create"}
	if strings.Join(containerRuntime.calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("runtime calls = %#v, want %#v", containerRuntime.calls, wantCalls)
	}
	if updated.DesiredState != domain.DesiredStopped {
		t.Fatalf("DesiredState = %q", updated.DesiredState)
	}
}

func TestUpdateReportsLifecycleAndPullProgress(t *testing.T) {
	containerRuntime := &fakeRuntime{
		pullProgress: []string{
			"downloading updated layer",
			"extracting updated layer",
		},
	}
	service := newTestService(t, containerRuntime)
	instance, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Image: "pantalk/ghost:old",
		Start: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	containerRuntime.resetCalls()
	var updates []UpdateProgress

	_, err = service.UpdateWithProgress(
		t.Context(),
		instance.ID,
		func(progress UpdateProgress) {
			updates = append(updates, progress)
		},
	)

	if err != nil {
		t.Fatalf("UpdateWithProgress() error = %v", err)
	}
	for _, expected := range []string{
		"preparing:Checking the current agent",
		"pulling:Pulling the updated agent image",
		"pulling:downloading updated layer",
		"pulling:extracting updated layer",
		"stopping:Stopping the current agent",
		"replacing:Replacing the runtime container",
		"starting:Starting the updated agent",
		"ready:Agent update is ready",
	} {
		found := false
		for _, update := range updates {
			if string(update.Stage)+":"+update.Message == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("update progress = %#v, missing %q", updates, expected)
		}
	}
}

func TestAgentFilesReturnsManagedAgentRoot(t *testing.T) {
	service := newTestService(t, &fakeRuntime{})
	instance, err := service.Create(
		t.Context(),
		CreateOptions{Name: "Ada"},
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	root, err := service.AgentFiles(t.Context(), instance.ID)

	if err != nil {
		t.Fatalf("AgentFiles() error = %v", err)
	}
	if filepath.Base(root) != instance.ID ||
		filepath.Base(filepath.Dir(root)) != "agents" {
		t.Fatalf("AgentFiles() = %q", root)
	}
	if _, err := os.Stat(filepath.Join(root, "workspace")); err != nil {
		t.Fatalf("workspace mount is unavailable: %v", err)
	}
}

func TestUpdateRestoresPreviousContainerWhenReplacementCreationFails(
	t *testing.T,
) {
	containerRuntime := &fakeRuntime{}
	service := newTestService(t, containerRuntime)
	instance, err := service.Create(t.Context(), CreateOptions{
		Name:  "Ada",
		Image: "pantalk/ghost:old",
		Start: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	containerRuntime.resetCalls()
	containerRuntime.createErrors = []error{
		errors.New("replacement failed"),
		nil,
	}

	_, err = service.Update(t.Context(), instance.ID)

	if err == nil || !strings.Contains(err.Error(), "previous container restored") {
		t.Fatalf("Update() error = %v", err)
	}
	wantCalls := []string{
		"pull", "stop", "remove", "create", "create", "start",
	}
	if strings.Join(containerRuntime.calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("runtime calls = %#v, want %#v", containerRuntime.calls, wantCalls)
	}
	if len(containerRuntime.createRequests) != 2 ||
		containerRuntime.createRequests[0].Image != "pantalk/ghost:default" ||
		containerRuntime.createRequests[1].Image != "pantalk/ghost:old" {
		t.Fatalf("create requests = %#v", containerRuntime.createRequests)
	}
	stored, readErr := service.store.Get(instance.ID)
	if readErr != nil || stored.Image != "pantalk/ghost:old" {
		t.Fatalf("stored instance = %#v, %v", stored, readErr)
	}
}

func TestListBoundsSlowRuntimeProbes(t *testing.T) {
	containerRuntime := &fakeRuntime{
		statusFunc: func(
			ctx context.Context,
			_ string,
		) (launchruntime.Status, error) {
			<-ctx.Done()
			return launchruntime.StatusMissing, ctx.Err()
		},
	}
	service := newTestService(t, containerRuntime)
	service.options.RuntimeProbeTimeout = 20 * time.Millisecond
	if _, err := service.Create(
		t.Context(),
		CreateOptions{Name: "Ada"},
	); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	started := time.Now()
	views, err := service.List(t.Context())

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("List() took %v after its runtime probe deadline", elapsed)
	}
	if len(views) != 1 ||
		!strings.Contains(views[0].MetricsError, "context deadline exceeded") {
		t.Fatalf("List() = %#v", views)
	}
}

func TestListProbesAgentsConcurrently(t *testing.T) {
	var started atomic.Int32
	var releaseOnce sync.Once
	release := make(chan struct{})
	containerRuntime := &fakeRuntime{
		statusFunc: func(
			ctx context.Context,
			_ string,
		) (launchruntime.Status, error) {
			if started.Add(1) == 2 {
				releaseOnce.Do(func() { close(release) })
			}
			select {
			case <-release:
				return launchruntime.StatusStopped, nil
			case <-ctx.Done():
				return launchruntime.StatusMissing, ctx.Err()
			}
		},
	}
	service := newTestService(t, containerRuntime)
	service.options.RuntimeProbeTimeout = 200 * time.Millisecond
	manifest := service.manifests[testGhostCatalogID]
	for index, instance := range []domain.Instance{
		testStoredInstance(
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"Ada",
			"launcher-ada",
			16902,
		),
		testStoredInstance(
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"Grace",
			"launcher-grace",
			16903,
		),
	} {
		if _, err := service.store.Create(instance, manifest); err != nil {
			t.Fatalf("store instance %d: %v", index, err)
		}
	}

	views, err := service.List(t.Context())

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(views) != 2 ||
		views[0].MetricsError != "" ||
		views[1].MetricsError != "" {
		t.Fatalf("List() = %#v", views)
	}
}

func testStoredInstance(
	id string,
	name string,
	containerName string,
	port int,
) domain.Instance {
	return domain.Instance{
		ID:            id,
		CatalogID:     testGhostCatalogID,
		Name:          name,
		Image:         "pantalk/ghost:test",
		ContainerName: containerName,
		Port:          port,
		DesiredState:  domain.DesiredStopped,
		CreatedAt:     time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}

func TestRenameChangesOnlyTheDisplayName(t *testing.T) {
	service := newTestService(t, &fakeRuntime{})
	instance, err := service.Create(t.Context(), CreateOptions{Name: "Ada"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	renamed, err := service.Rename(t.Context(), instance.ID, "Grace")

	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if renamed.Name != "Grace" ||
		renamed.ContainerName != instance.ContainerName ||
		renamed.ID != instance.ID {
		t.Fatalf("Rename() = %#v", renamed)
	}
	stored, err := service.store.Get("Grace")
	if err != nil || stored.ID != instance.ID {
		t.Fatalf("Get(renamed) = %#v, %v", stored, err)
	}
}

func TestRecentLogsReadsFromTheAgentRuntime(t *testing.T) {
	containerRuntime := &fakeRuntime{recentLogs: "agent ready\n"}
	service := newTestService(t, containerRuntime)
	instance, err := service.Create(t.Context(), CreateOptions{Name: "Ada"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	logs, err := service.RecentLogs(t.Context(), instance.ID, 200)

	if err != nil || logs != "agent ready\n" {
		t.Fatalf("RecentLogs() = %q, %v", logs, err)
	}
	if containerRuntime.recentLogName != instance.ContainerName ||
		containerRuntime.recentLogLines != 200 {
		t.Fatalf("runtime log request = %q, %d",
			containerRuntime.recentLogName,
			containerRuntime.recentLogLines,
		)
	}
}

func newTestService(t *testing.T, containerRuntime *fakeRuntime) *Service {
	t.Helper()
	manifest := catalog.Manifest{
		ID:          testGhostCatalogID,
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
		Image:         "pantalk/ghost:default",
		Viewer:        "kasmvnc",
		ContainerPort: 6901,
		SharedMemory:  "1g",
		Environment:   map[string]string{"PANTALK_AUTOSTART": "true"},
		Mounts:        []catalog.Mount{{Name: "workspace", Target: "/workspace"}},
	}
	return New(store.New(t.TempDir()), containerRuntime, []catalog.Manifest{manifest}, Options{
		ID: func() (string, error) {
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		},
		Now: func() time.Time {
			return time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
		},
		Ports:       fixedPortAllocator{port: 16902},
		Platform:    "linux/amd64",
		RuntimeName: "docker",
	})
}

type fixedPortAllocator struct{ port int }

func (allocator fixedPortAllocator) Allocate(int, map[int]struct{}) (int, error) {
	return allocator.port, nil
}

type fakeRuntime struct {
	status         launchruntime.Status
	pullCalled     bool
	createCalled   bool
	startCalled    bool
	stopCalled     bool
	removeCalled   bool
	createRequest  launchruntime.CreateRequest
	createRequests []launchruntime.CreateRequest
	createErr      error
	createErrors   []error
	metrics        launchruntime.Metrics
	statsErr       error
	recentLogs     string
	recentLogName  string
	recentLogLines int
	statusFunc     func(context.Context, string) (launchruntime.Status, error)
	statsFunc      func(context.Context, string) (launchruntime.Metrics, error)
	pullProgress   []string
	pullImage      string
	pullPlatform   string
	calls          []string
}

func (*fakeRuntime) Doctor(context.Context) (string, error) { return "test", nil }
func (runtime *fakeRuntime) Pull(
	_ context.Context,
	image string,
	platform string,
) error {
	runtime.pullCalled = true
	runtime.pullImage = image
	runtime.pullPlatform = platform
	runtime.calls = append(runtime.calls, "pull")
	return nil
}
func (runtime *fakeRuntime) PullWithProgress(
	_ context.Context,
	image string,
	platform string,
	progress func(string),
) error {
	runtime.pullCalled = true
	runtime.pullImage = image
	runtime.pullPlatform = platform
	runtime.calls = append(runtime.calls, "pull")
	for _, message := range runtime.pullProgress {
		progress(message)
	}
	return nil
}
func (runtime *fakeRuntime) Create(
	_ context.Context,
	request launchruntime.CreateRequest,
) error {
	runtime.createCalled = true
	runtime.createRequest = request
	runtime.createRequests = append(runtime.createRequests, request)
	runtime.calls = append(runtime.calls, "create")
	if len(runtime.createErrors) > 0 {
		err := runtime.createErrors[0]
		runtime.createErrors = runtime.createErrors[1:]
		return err
	}
	return runtime.createErr
}
func (runtime *fakeRuntime) Start(context.Context, string) error {
	runtime.startCalled = true
	runtime.status = launchruntime.StatusRunning
	runtime.calls = append(runtime.calls, "start")
	return nil
}
func (runtime *fakeRuntime) Stop(context.Context, string) error {
	runtime.stopCalled = true
	runtime.status = launchruntime.StatusStopped
	runtime.calls = append(runtime.calls, "stop")
	return nil
}
func (runtime *fakeRuntime) Remove(context.Context, string, string) error {
	runtime.removeCalled = true
	runtime.status = launchruntime.StatusMissing
	runtime.calls = append(runtime.calls, "remove")
	return nil
}
func (runtime *fakeRuntime) Status(
	ctx context.Context,
	name string,
) (launchruntime.Status, error) {
	if runtime.statusFunc != nil {
		return runtime.statusFunc(ctx, name)
	}
	return runtime.status, nil
}
func (runtime *fakeRuntime) Stats(
	ctx context.Context,
	name string,
) (launchruntime.Metrics, error) {
	if runtime.statsFunc != nil {
		return runtime.statsFunc(ctx, name)
	}
	return runtime.metrics, runtime.statsErr
}
func (*fakeRuntime) Logs(context.Context, string, bool) error { return nil }
func (runtime *fakeRuntime) RecentLogs(
	_ context.Context,
	name string,
	lines int,
) (string, error) {
	runtime.recentLogName = name
	runtime.recentLogLines = lines
	return runtime.recentLogs, nil
}

func containsText(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (runtime *fakeRuntime) resetCalls() {
	runtime.pullCalled = false
	runtime.createCalled = false
	runtime.startCalled = false
	runtime.stopCalled = false
	runtime.removeCalled = false
	runtime.pullImage = ""
	runtime.pullPlatform = ""
	runtime.createRequests = nil
	runtime.calls = nil
}
