package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/updatecheck"
)

const testCatalogID = "370a2228-322d-4089-846b-62fb8c15d154"

func TestIndexInjectsSessionToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	New(&fakeService{}, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if body := response.Body.String(); !strings.Contains(
		body,
		`name="launcher-token" content="test-token"`,
	) {
		t.Fatal("index is missing the session token")
	}
}

func TestDesignAssetsAreServed(t *testing.T) {
	handler := New(
		&fakeService{},
		"test-token",
		WithCatalogAssets(fstest.MapFS{
			"pantalk-ghost/icon.svg": {
				Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
			},
			"pantalk-ghost/screenshot.png": {
				Data: []byte("\x89PNG\r\n\x1a\n"),
			},
			"buzznode/screenshot.png": {
				Data: []byte("\x89PNG\r\n\x1a\n"),
			},
		}),
	)
	tests := []struct {
		path         string
		contentType  string
		body         string
		cacheControl string
	}{
		{
			path:         "/main.js",
			contentType:  "text/javascript",
			body:         "components/launcher-app.js",
			cacheControl: "no-store",
		},
		{
			path:         "/styles.css",
			contentType:  "text/css",
			body:         "/assets/hero.png",
			cacheControl: "no-store",
		},
		{
			path:         "/components/agent-card.js",
			contentType:  "text/javascript",
			body:         "customElements.define('agent-card'",
			cacheControl: "no-store",
		},
		{
			path:         "/components/marketplace-detail.js",
			contentType:  "text/javascript",
			body:         "customElements.define('marketplace-detail'",
			cacheControl: "no-store",
		},
		{
			path:         "/assets/logo.png",
			contentType:  "image/png",
			cacheControl: "public, max-age=3600",
		},
		{
			path:        "/catalog-assets/pantalk-ghost/icon.svg",
			contentType: "image/svg+xml",
		},
		{
			path:        "/catalog-assets/pantalk-ghost/screenshot.png",
			contentType: "image/png",
		},
		{
			path:        "/catalog-assets/buzznode/screenshot.png",
			contentType: "image/png",
		},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, body = %q",
					response.Code,
					response.Body.String(),
				)
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(
				contentType,
				test.contentType,
			) {
				t.Fatalf("Content-Type = %q", contentType)
			}
			if test.body != "" && !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("body missing %q", test.body)
			}
			if cacheControl := response.Header().Get("Cache-Control"); cacheControl !=
				test.cacheControl {
				t.Fatalf(
					"Cache-Control = %q, want %q",
					cacheControl,
					test.cacheControl,
				)
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, "/support.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("obsolete support.js status = %d, want 404", response.Code)
	}
}

func TestAPIRequiresSessionToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/instances", nil)
	response := httptest.NewRecorder()

	New(&fakeService{}, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLauncherReturnsUpdateStatus(t *testing.T) {
	checkedAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	request := apiRequest(http.MethodGet, "/api/launcher", nil)
	response := httptest.NewRecorder()

	New(
		&fakeService{},
		"test-token",
		WithUpdateStatus(func() updatecheck.Status {
			return updatecheck.Status{
				CurrentVersion:  "0.2.0",
				LatestVersion:   "0.3.0",
				ReleaseURL:      "https://github.com/pdparchitect/launcher/releases/tag/v0.3.0",
				UpdateAvailable: true,
				CheckedAt:       &checkedAt,
			}
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var status updatecheck.Status
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if status.CurrentVersion != "0.2.0" ||
		status.LatestVersion != "0.3.0" ||
		!status.UpdateAvailable ||
		status.ReleaseURL == "" ||
		status.CheckedAt == nil ||
		!status.CheckedAt.Equal(checkedAt) {
		t.Fatalf("Launcher status = %#v", status)
	}
}

func TestDoctorReturnsMissingRuntimeSetupInstructions(t *testing.T) {
	service := &fakeService{doctorErr: &fakeRuntimeInstallError{}}
	request := apiRequest(http.MethodGet, "/api/doctor", nil)
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var body struct {
		Ready bool                 `json:"ready"`
		Setup runtimeSetupResponse `json:"setup"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Ready ||
		body.Setup.State != "missing" ||
		body.Setup.Runtime != "Apple container" ||
		body.Setup.InstallURL !=
			"https://github.com/apple/container/releases/latest" ||
		body.Setup.Guidance == "" ||
		body.Setup.CanStart {
		t.Fatalf("doctor response = %#v", body)
	}
}

func TestDoctorReturnsStartableRuntimeSetup(t *testing.T) {
	service := &fakeService{doctorErr: &fakeRuntimeStartError{}}
	request := apiRequest(http.MethodGet, "/api/doctor", nil)
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var body struct {
		Ready bool                 `json:"ready"`
		Setup runtimeSetupResponse `json:"setup"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Ready ||
		body.Setup.State != "stopped" ||
		body.Setup.Runtime != "Apple container" ||
		!body.Setup.CanStart ||
		body.Setup.InstallURL != "" {
		t.Fatalf("doctor response = %#v", body)
	}
}

func TestStartRuntimeStartsServiceAndReturnsReadyReport(t *testing.T) {
	service := &fakeService{}
	startError := &fakeRuntimeStartError{service: service}
	service.doctorErr = startError
	request := apiRequest(http.MethodPost, "/api/runtime/start", []byte(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var body struct {
		Ready  bool               `json:"ready"`
		Report agent.DoctorReport `json:"report"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !startError.started || !body.Ready || body.Report.Runtime != "docker" {
		t.Fatalf("runtime start response = %#v, started = %v", body, startError.started)
	}
}

func TestListInstancesReturnsDesktopURL(t *testing.T) {
	service := &fakeService{views: []agent.View{{
		Instance:        testInstance(),
		State:           launchruntime.StatusRunning,
		UpdateAvailable: true,
		AvailableImage:  "pantalk/ghost:new",
		Metrics: launchruntime.Metrics{
			CPUPercent:       0.22,
			CPUAvailable:     true,
			MemoryPercent:    0.34,
			MemoryAvailable:  true,
			MemoryUsageBytes: 169764454,
			MemoryLimitBytes: 50541021716,
		},
		Uptime: 5 * time.Minute,
	}}}
	request := apiRequest(http.MethodGet, "/api/instances", nil)
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var body struct {
		Instances []instanceResponse `json:"instances"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Instances) != 1 ||
		body.Instances[0].Interfaces["desktop"].URL !=
			"http://127.0.0.1:16902/" ||
		body.Instances[0].Metrics == nil ||
		*body.Instances[0].Metrics.CPUPercent != 0.22 ||
		*body.Instances[0].Metrics.MemoryPercent != 0.34 ||
		body.Instances[0].Metrics.UptimeSeconds != 300 ||
		!body.Instances[0].UpdateAvailable ||
		body.Instances[0].AvailableImage != "pantalk/ghost:new" {
		t.Fatalf("instances = %#v", body.Instances)
	}
}

func TestCreateInstanceStartsByDefault(t *testing.T) {
	service := &fakeService{created: testInstance()}
	request := apiRequest(
		http.MethodPost,
		"/api/instances",
		[]byte(`{"catalogId":"370a2228-322d-4089-846b-62fb8c15d154","name":"Ada"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if !service.createOptions.Start ||
		service.createOptions.CatalogID != testCatalogID ||
		service.createOptions.Name != "Ada" {
		t.Fatalf("CreateOptions = %#v", service.createOptions)
	}
}

func TestInstallInstanceStreamsProgress(t *testing.T) {
	service := &fakeService{created: testInstance()}
	request := apiRequest(
		http.MethodPost,
		"/api/instances/install",
		[]byte(`{"catalogId":"370a2228-322d-4089-846b-62fb8c15d154","name":"Ada"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(
		contentType,
		"application/x-ndjson",
	) {
		t.Fatalf("Content-Type = %q", contentType)
	}
	for _, expected := range []string{
		`"type":"progress"`,
		`"stage":"pulling"`,
		`"stage":"starting"`,
		`"type":"complete"`,
		`"name":"Ada"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("stream missing %q: %s", expected, response.Body.String())
		}
	}
}

func TestServerLogsRequests(t *testing.T) {
	var logs bytes.Buffer
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	New(&fakeService{}, "test-token", WithLogger(&logs)).ServeHTTP(response, request)

	for _, expected := range []string{"GET", "/", "200"} {
		if !strings.Contains(logs.String(), expected) {
			t.Fatalf("logs missing %q: %q", expected, logs.String())
		}
	}
}

func TestAPIAllowsHTTPSOriginForForwardedDevelopmentURL(t *testing.T) {
	request := apiRequest(http.MethodGet, "/api/instances", nil)
	request.Host = "launcher.example.test"
	request.Header.Set("Origin", "https://launcher.example.test")
	response := httptest.NewRecorder()

	New(&fakeService{}, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestStartInstanceInvokesLifecycleService(t *testing.T) {
	service := &fakeService{created: testInstance()}
	request := apiRequest(
		http.MethodPost,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/start",
		[]byte(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if service.started != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("started = %q", service.started)
	}
}

func TestUpdateInstanceInvokesLifecycleService(t *testing.T) {
	service := &fakeService{created: testInstance()}
	request := apiRequest(
		http.MethodPost,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/update",
		[]byte(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(
		contentType,
		"application/x-ndjson",
	) {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if service.updated != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("updated = %q", service.updated)
	}
	for _, expected := range []string{
		`"type":"progress"`,
		`"stage":"pulling"`,
		`"stage":"replacing"`,
		`"stage":"ready"`,
		`"type":"complete"`,
		`"image":"pantalk/ghost:new"`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("update stream missing %q: %q", expected, response.Body.String())
		}
	}
}

func TestRenameInstanceUpdatesDisplayName(t *testing.T) {
	service := &fakeService{created: testInstance()}
	request := apiRequest(
		http.MethodPatch,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		[]byte(`{"name":"Grace"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if service.renamedReference != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		service.renamedName != "Grace" {
		t.Fatalf(
			"Rename() = %q, %q",
			service.renamedReference,
			service.renamedName,
		)
	}
}

func TestOpenInstanceFilesUsesManagedAgentPath(t *testing.T) {
	service := &fakeService{
		filesPath: "/tmp/launcher/agents/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	var opened string
	request := apiRequest(
		http.MethodPost,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/files",
		[]byte(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(
		service,
		"test-token",
		WithPathOpener(func(path string) error {
			opened = path
			return nil
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if opened != service.filesPath ||
		!strings.Contains(response.Body.String(), service.filesPath) {
		t.Fatalf("opened = %q, body = %q", opened, response.Body.String())
	}
}

func TestOpenInstanceViewerUsesConfiguredViewerOpener(t *testing.T) {
	instance := testInstance()
	service := &fakeService{
		view: agent.View{
			Instance: instance,
			State:    launchruntime.StatusRunning,
		},
	}
	var opened string
	var openedURL string
	var openedKind string
	request := apiRequest(
		http.MethodPost,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/viewer",
		[]byte(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(
		service,
		"test-token",
		WithViewerOpener(func(target ViewerTarget) error {
			opened = target.ID
			openedURL = target.URL
			openedKind = target.Kind
			return nil
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if opened != instance.ID {
		t.Fatalf("opened = %q", opened)
	}
	// The spawner must receive a resolved URL so the viewer process never has
	// to inspect the container again just to learn where the agent lives.
	wantURL := instance.Interfaces["desktop"].URL()
	if openedURL != wantURL {
		t.Fatalf("opened URL = %q, want %q", openedURL, wantURL)
	}
	if openedKind != "kasmweb" {
		t.Fatalf("opened kind = %q, want kasmweb", openedKind)
	}
}

func TestOpenInstanceViewerRejectsStoppedAgent(t *testing.T) {
	instance := testInstance()
	service := &fakeService{
		view: agent.View{
			Instance: instance,
			State:    launchruntime.StatusStopped,
		},
	}
	request := apiRequest(
		http.MethodPost,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/viewer",
		[]byte(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	New(
		service,
		"test-token",
		WithViewerOpener(func(ViewerTarget) error { return nil }),
	).ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestRecentInstanceLogsReturnsJSON(t *testing.T) {
	service := &fakeService{logs: "agent ready\n"}
	request := apiRequest(
		http.MethodGet,
		"/api/instances/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/logs",
		nil,
	)
	response := httptest.NewRecorder()

	New(service, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"logs":"agent ready\n"`) ||
		service.logLines != 200 {
		t.Fatalf("logs response = %q", response.Body.String())
	}
}

func TestCatalogueAssetsCanComeFromDownloadedFilesystem(t *testing.T) {
	handler := New(
		&fakeService{},
		"test-token",
		WithCatalogAssets(fstest.MapFS{
			"remote/icon.svg": {
				Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
			},
		}),
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/catalog-assets/remote/icon.svg",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "<svg") {
		t.Fatalf(
			"catalogue asset response = %d, %q",
			response.Code,
			response.Body.String(),
		)
	}
}

func apiRequest(method string, target string, body []byte) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.Header.Set("X-Launcher-Token", "test-token")
	return request
}

type fakeService struct {
	views            []agent.View
	view             agent.View
	created          domain.Instance
	createOptions    agent.CreateOptions
	started          string
	updated          string
	renamedReference string
	renamedName      string
	logs             string
	logLines         int
	doctorErr        error
	filesPath        string
}

func (service *fakeService) Doctor(context.Context) (agent.DoctorReport, error) {
	if service.doctorErr != nil {
		return agent.DoctorReport{}, service.doctorErr
	}
	return agent.DoctorReport{
		Runtime: "docker", Version: "test", DataRoot: "/tmp/launcher",
		DefaultImage: "pantalk/ghost:test",
	}, nil
}
func (*fakeService) Catalog() []agent.CatalogEntry {
	return []agent.CatalogEntry{{
		ID: testCatalogID, Slug: "pantalk-ghost",
		Name: "Pantalk Ghost", Publisher: "Pantalk",
		Image: "pantalk/ghost:test",
	}}
}
func (service *fakeService) Create(
	_ context.Context,
	options agent.CreateOptions,
) (domain.Instance, error) {
	service.createOptions = options
	if options.Progress != nil {
		options.Progress(agent.CreateProgress{
			Stage: "pulling", Message: "Pulling agent image",
		})
		options.Progress(agent.CreateProgress{
			Stage: "starting", Message: "Starting agent",
		})
	}
	return service.created, nil
}
func (service *fakeService) List(context.Context) ([]agent.View, error) {
	return service.views, nil
}
func (service *fakeService) Get(context.Context, string) (agent.View, error) {
	if service.view.ID != "" {
		return service.view, nil
	}
	return agent.View{Instance: testInstance()}, nil
}
func (service *fakeService) Start(
	_ context.Context,
	reference string,
) (domain.Instance, error) {
	service.started = reference
	return testInstance(), nil
}
func (*fakeService) Stop(context.Context, string) (domain.Instance, error) {
	return testInstance(), nil
}
func (service *fakeService) Update(
	_ context.Context,
	reference string,
) (domain.Instance, error) {
	service.updated = reference
	instance := testInstance()
	instance.Image = "pantalk/ghost:new"
	return instance, nil
}
func (service *fakeService) UpdateWithProgress(
	ctx context.Context,
	reference string,
	progress func(agent.UpdateProgress),
) (domain.Instance, error) {
	for _, update := range []agent.UpdateProgress{
		{Stage: agent.UpdateStagePulling, Message: "Pulling updated image"},
		{Stage: agent.UpdateStageReplacing, Message: "Replacing container"},
		{Stage: agent.UpdateStageReady, Message: "Agent update is ready"},
	} {
		progress(update)
	}
	return service.Update(ctx, reference)
}
func (*fakeService) Delete(context.Context, string) error { return nil }
func (service *fakeService) Rename(
	_ context.Context,
	reference string,
	name string,
) (domain.Instance, error) {
	service.renamedReference = reference
	service.renamedName = name
	instance := testInstance()
	instance.Name = name
	return instance, nil
}
func (service *fakeService) RecentLogs(
	_ context.Context,
	_ string,
	lines int,
) (string, error) {
	service.logLines = lines
	return service.logs, nil
}
func (service *fakeService) AgentFiles(
	context.Context,
	string,
) (string, error) {
	return service.filesPath, nil
}
func (*fakeService) Logs(context.Context, string, bool) error {
	return nil
}

type fakeRuntimeInstallError struct{}

func (*fakeRuntimeInstallError) Error() string {
	return "Apple container runtime is missing"
}
func (*fakeRuntimeInstallError) RuntimeName() string { return "Apple container" }
func (*fakeRuntimeInstallError) InstallURL() string {
	return "https://github.com/apple/container/releases/latest"
}
func (*fakeRuntimeInstallError) InstallGuidance() string {
	return "Download and run the signed installer package."
}

type fakeRuntimeStartError struct {
	service *fakeService
	started bool
}

func (*fakeRuntimeStartError) Error() string {
	return "Apple container service is stopped"
}
func (*fakeRuntimeStartError) RuntimeName() string { return "Apple container" }
func (runtimeError *fakeRuntimeStartError) StartService(context.Context) error {
	runtimeError.started = true
	runtimeError.service.doctorErr = nil
	return nil
}

func testInstance() domain.Instance {
	return domain.Instance{
		ID:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CatalogID:     testCatalogID,
		Name:          "Ada",
		Image:         "pantalk/ghost:test",
		ContainerName: "launcher-ghost-aaaaaaaaaaaa",
		Interfaces: map[string]domain.Interface{
			"desktop": {Kind: "kasmweb", Port: 16902, Path: "/"},
		},
		DesiredState: domain.DesiredRunning,
		CreatedAt:    time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}
