package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

const testCatalogID = "370a2228-322d-4089-846b-62fb8c15d154"

func TestIndexPreservesDesignAndInjectsSessionToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	New(&fakeService{}, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{
		`name="launcher-token" content="test-token"`,
		`<launcher-app></launcher-app>`,
		`rel="modulepreload" href="/desktop-window.js"`,
		`rel="modulepreload" href="/components/deploy-dialog.js"`,
		`<script type="module" src="/main.js">`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q", expected)
		}
	}
	for _, loadingScreen := range []string{
		"boot-screen",
		"Starting your local agent orchestrator",
	} {
		if strings.Contains(body, loadingScreen) {
			t.Fatalf("body still contains loading screen %q", loadingScreen)
		}
	}
	if strings.Contains(body, "fonts.googleapis.com") {
		t.Fatal("index still contains a render-blocking remote font stylesheet")
	}
}

func TestWindowChromeUsesDesktopRuntimeAdapter(t *testing.T) {
	source := readWebSources(
		t,
		"desktop-window.js",
		"components/launcher-app.js",
		"styles.css",
	)
	for _, expected := range []string{
		"WindowMinimise",
		"WindowToggleMaximise",
		"invoke('Quit')",
		`data-window-action="close"`,
		"border: 1px solid transparent",
		"border-color: var(--line-bright)",
		"--wails-draggable: drag",
		"--wails-draggable: no-drag",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing desktop window behavior %q", expected)
		}
	}
	controls := readWebSources(t, "components/launcher-app.js")
	minimise := strings.Index(controls, `data-window-action="minimise"`)
	maximise := strings.Index(controls, `data-window-action="maximise"`)
	closeWindow := strings.Index(controls, `data-window-action="close"`)
	if minimise < 0 || maximise < minimise || closeWindow < maximise {
		t.Fatal("window controls must be ordered minimise, maximise, close")
	}
}

func TestLauncherWindowUsesDialogBorder(t *testing.T) {
	styles := readWebSources(t, "styles.css")
	start := strings.Index(styles, "body::after {")
	if start < 0 {
		t.Fatal("interface missing Launcher window frame")
	}
	end := strings.Index(styles[start:], "}")
	if end < 0 {
		t.Fatal("Launcher window frame style is incomplete")
	}
	frame := styles[start : start+end]
	for _, expected := range []string{
		"position: fixed",
		"inset: 0",
		"border: 1px solid var(--line-bright)",
		"pointer-events: none",
	} {
		if !strings.Contains(frame, expected) {
			t.Fatalf("Launcher window frame missing %q", expected)
		}
	}
}

func TestInterfaceDisablesElasticDocumentScrolling(t *testing.T) {
	styles := readWebSources(t, "styles.css")

	documentStart := strings.Index(styles, "html,\nbody {")
	if documentStart < 0 {
		t.Fatal("interface missing document viewport styles")
	}
	documentEnd := strings.Index(styles[documentStart:], "}")
	if documentEnd < 0 {
		t.Fatal("document viewport styles are incomplete")
	}
	documentRule := styles[documentStart : documentStart+documentEnd]
	for _, expected := range []string{
		"height: 100%",
		"overflow: hidden",
		"overscroll-behavior: none",
	} {
		if !strings.Contains(documentRule, expected) {
			t.Fatalf("document viewport styles missing %q", expected)
		}
	}

	panelStart := strings.Index(styles, ".main-panel {")
	if panelStart < 0 {
		t.Fatal("interface missing main panel styles")
	}
	panelEnd := strings.Index(styles[panelStart:], "}")
	if panelEnd < 0 {
		t.Fatal("main panel styles are incomplete")
	}
	panelRule := styles[panelStart : panelStart+panelEnd]
	for _, expected := range []string{
		"height: 100%",
		"overflow-y: auto",
		"overscroll-behavior: none",
	} {
		if !strings.Contains(panelRule, expected) {
			t.Fatalf("main panel styles missing %q", expected)
		}
	}
}

func TestInterfacePreventsAccidentalChromeSelection(t *testing.T) {
	styles := readWebSources(t, "styles.css")

	globalStart := strings.Index(styles, "* {")
	if globalStart < 0 {
		t.Fatal("interface missing global element styles")
	}
	globalEnd := strings.Index(styles[globalStart:], "}")
	if globalEnd < 0 {
		t.Fatal("global element styles are incomplete")
	}
	globalRule := styles[globalStart : globalStart+globalEnd]
	for _, expected := range []string{
		"-webkit-user-select: none",
		"user-select: none",
	} {
		if !strings.Contains(globalRule, expected) {
			t.Fatalf("global element styles missing %q", expected)
		}
	}

	for _, expected := range []string{
		"input,\ntextarea,\npre,\ncode,",
		"[data-settings-root],",
		"[data-current-image],",
		"[data-available-image]",
		"-webkit-user-select: text",
		"user-select: text",
		"img {",
		"-webkit-user-drag: none",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("interface missing selection behavior %q", expected)
		}
	}
}

func TestInterfaceUsesSubtleScrollbars(t *testing.T) {
	styles := readWebSources(t, "styles.css")

	for _, expected := range []string{
		"scrollbar-width: thin",
		"scrollbar-color: rgba(135, 140, 130, 0.42) transparent",
		"*::-webkit-scrollbar {",
		"width: 6px",
		"height: 6px",
		"*::-webkit-scrollbar-track {",
		"background: transparent",
		"*::-webkit-scrollbar-thumb {",
		"background-clip: padding-box",
		"*::-webkit-scrollbar-thumb:hover {",
		"*::-webkit-scrollbar-corner {",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("interface missing subtle scrollbar behavior %q", expected)
		}
	}
}

func TestSettingsScreenRemainsImplementedButHiddenFromNavigation(t *testing.T) {
	source := readWebSources(
		t,
		"components/launcher-app.js",
		"styles.css",
	)
	for _, expected := range []string{
		`data-screen="settings"`,
		"LAUNCHER SETTINGS",
		"Settings are intentionally hidden from the main navigation",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing Settings implementation %q", expected)
		}
	}
	if strings.Contains(source, "['settings',") {
		t.Fatal("Settings remains visible in the navigation")
	}
}

func TestDeploymentUsesNativeDialogInsteadOfBrowserPrompt(t *testing.T) {
	source := readWebSources(
		t,
		"components/deploy-dialog.js",
		"components/launcher-app.js",
	)
	for _, expected := range []string{
		"<dialog",
		"data-launcher-deploy-dialog",
		"showModal()",
		"DEPLOY AGENT",
		"'install-agent'",
		"setProgress(progress)",
		"data-install-log-output",
		"progress-track--indeterminate",
		"'DOWNLOADING'",
		"appendLog(update.stage, update.message)",
		`data-close type="button"`,
		`data-cancel type="button"`,
		"if (!this.busy)",
		"this.close()",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing deployment dialog behavior %q", expected)
		}
	}
	if strings.Contains(source, "window.prompt") {
		t.Fatal("deployment still uses the browser prompt")
	}
	if strings.Contains(source, "pulling: 48") {
		t.Fatal("deployment still presents image pulling as a stuck percentage")
	}
	if strings.Contains(source, `src="{{`) {
		t.Fatal("dynamic image source triggers a raw template URL request")
	}
}

func TestUpdateDialogStreamsProgressAndRetainsLogs(t *testing.T) {
	source := readWebSources(
		t,
		"api.js",
		"components/agent-actions-dialog.js",
		"components/launcher-app.js",
	)
	for _, expected := range []string{
		"progressRequest(",
		"`/api/instances/${encodeURIComponent(id)}/update`",
		"data-update-progress",
		"data-update-progress-bar",
		"UPDATE LOG",
		"setUpdateProgress(update)",
		"appendUpdateLog(update.stage, update.message)",
		"'DOWNLOADING'",
		"'RECOVERING'",
		"dialog.setUpdateProgress(progress)",
		"setTimeout(() => dialog.close(), 500)",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing update progress behavior %q", expected)
		}
	}
}

func TestAgentCardsUseRuntimeMetricsAndStableCatalogueIDs(t *testing.T) {
	source := readWebSources(
		t,
		"components/agent-card.js",
		"components/launcher-app.js",
		"components/agent-viewer-dialog.js",
		"api.js",
	)
	for _, expected := range []string{
		"metrics?.cpuPercent",
		"metrics?.memoryPercent",
		"metrics?.uptimeSeconds",
		"item.id === agent.catalogId",
		"setInterval(() => this.refreshAgents(), 5000)",
		"<agent-viewer-dialog></agent-viewer-dialog>",
		"this.querySelector('agent-viewer-dialog').open(agent, entry?.viewer)",
		"OPEN IN WINDOW",
		"data-viewer-frame",
		"'show_control_bar', 'true'",
		"'resize', 'remote'",
		"'enable_threading', 'false'",
		"window-management",
		"entry?.viewer",
		"this.api.openViewer(agent.id)",
		"`/api/instances/${encodeURIComponent(id)}/viewer`",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing runtime behavior %q", expected)
		}
	}
	for _, placeholder := range []string{"cpu: 0", "mem: 0"} {
		if strings.Contains(source, placeholder) {
			t.Fatalf("interface still contains placeholder %q", placeholder)
		}
	}
}

func TestDesktopEmbeddingAvoidsCrossFrameWailsInjection(t *testing.T) {
	source := readWebSources(
		t,
		"main.js",
		"components/agent-viewer-dialog.js",
	)
	for _, expected := range []string{
		"document.addEventListener('contextmenu'",
		"event.target.closest('iframe')",
		"autoplay; microphone; camera;",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing embedded-frame safeguard %q", expected)
		}
	}
}

func TestAgentActionsUseNativeDialogAndFunctionalButtons(t *testing.T) {
	source := readWebSources(
		t,
		"api.js",
		"components/agent-actions-dialog.js",
		"components/launcher-app.js",
	)
	for _, expected := range []string{
		"data-launcher-actions-dialog",
		"RENAME AGENT",
		"VIEW LOGS",
		"OPEN LOCAL FILES",
		"UPDATE AGENT",
		"DELETE AGENT",
		"'rename-agent'",
		"'load-agent-logs'",
		"'open-agent-files'",
		"'update-agent'",
		"'delete-agent'",
		"this.renameAgent(event.detail)",
		"this.loadAgentLogs(event.detail.agent)",
		"this.openAgentFiles(event.detail.agent)",
		"this.updateAgent(event.detail.agent)",
		"this.api.openFiles(agent.id)",
		"`/api/instances/${encodeURIComponent(id)}/files`",
		"this.deleteAgent(event.detail.agent)",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing agent action behavior %q", expected)
		}
	}
	if strings.Contains(source, ">⚙</button>") {
		t.Fatal("agent card still contains the placeholder settings button")
	}
}

func TestBackgroundRefreshDoesNotShowBlockingAlert(t *testing.T) {
	source := readWebSources(
		t,
		"api.js",
		"components/launcher-app.js",
	)
	for _, expected := range []string{
		"async refreshAgents()",
		"console.warn('Agent refresh failed:'",
		"Launcher request failed (${response.status})",
		"new AbortController()",
		"Launcher request timed out",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing refresh behavior %q", expected)
		}
	}
	if strings.Contains(source, "window.alert") {
		t.Fatal("background refresh can still show a blocking alert")
	}
}

func TestFailedStartShowsRuntimeLogToast(t *testing.T) {
	source := readWebSources(t, "components/launcher-app.js")
	for _, expected := range []string{
		"this.startWatches = new Map()",
		"await this.checkStartWatches()",
		"reportAfter: now + 750",
		"setTimeout(() => this.refreshAgents(), 750)",
		"await this.api.logs(id)",
		"failed to start",
		"'The container stopped during startup'",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing failed-start feedback %q", expected)
		}
	}
}

func TestRuntimeSetupDialogGuidesInstallationAndRechecksRuntime(t *testing.T) {
	source := readWebSources(
		t,
		"api.js",
		"desktop-window.js",
		"components/launcher-app.js",
		"components/runtime-setup-dialog.js",
		"styles.css",
	)
	for _, expected := range []string{
		"<runtime-setup-dialog></runtime-setup-dialog>",
		"<dialog class=\"launcher-dialog runtime-setup-dialog\"",
		"OPEN INSTALLATION PAGE",
		"installer-signed.pkg",
		"CHECK AGAIN",
		"START RUNTIME",
		"startRuntime()",
		"'/api/runtime/start'",
		"desktopWindow.openExternal(installURL)",
		"BrowserOpenURL",
		"this.openRuntimeSetup()",
		".sidebar__footer--unavailable .runtime-light",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing runtime setup behavior %q", expected)
		}
	}
}

func TestAgentPaginationReflectsActualResultCount(t *testing.T) {
	source := readWebSources(t, "components/launcher-app.js")
	for _, expected := range []string{
		"Math.ceil(matchingAgents.length / pageSize)",
		"{ length: pageCount }",
		"pageStart + pageSize",
		"this.page = 1",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing dynamic pagination %q", expected)
		}
	}
	if strings.Contains(source, "const pages = [1, 2, 3, 4]") {
		t.Fatal("interface still contains hard-coded page buttons")
	}
}

func TestInterfaceUsesStandardsOnlyWebComponents(t *testing.T) {
	source := readWebSources(
		t,
		"index.html",
		"main.js",
		"api.js",
		"components/launcher-app.js",
		"components/agent-card.js",
		"components/marketplace-card.js",
		"components/deploy-dialog.js",
		"components/agent-actions-dialog.js",
		"components/runtime-setup-dialog.js",
	)
	for _, expected := range []string{
		`customElements.define('launcher-app'`,
		`customElements.define('agent-card'`,
		`customElements.define('marketplace-card'`,
		`customElements.define('deploy-dialog'`,
		`customElements.define('agent-actions-dialog'`,
		`customElements.define('runtime-setup-dialog'`,
		`<script type="module" src="/main.js">`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing Web Component %q", expected)
		}
	}
	for _, generated := range []string{
		"<sc-for",
		"<sc-if",
		"<x-dc",
		"DCLogic",
		"text/x-dc",
		"support.js",
		"{{",
	} {
		if strings.Contains(source, generated) {
			t.Fatalf("interface still contains generated runtime syntax %q", generated)
		}
	}
}

func TestDesignAssetsAreServed(t *testing.T) {
	handler := New(&fakeService{}, "test-token")
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

func readWebSources(t *testing.T, names ...string) string {
	t.Helper()
	var source strings.Builder
	for _, name := range names {
		data, err := webFiles.ReadFile("web/" + name)
		if err != nil {
			t.Fatalf("read embedded interface %q: %v", name, err)
		}
		source.Write(data)
		source.WriteByte('\n')
	}
	return source.String()
}

func TestAPIRequiresSessionToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/instances", nil)
	response := httptest.NewRecorder()

	New(&fakeService{}, "test-token").ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
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
		body.Instances[0].URL != "http://127.0.0.1:16902" ||
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
		WithViewerOpener(func(reference string) error {
			opened = reference
			return nil
		}),
	).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if opened != instance.ID {
		t.Fatalf("opened = %q", opened)
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
		WithViewerOpener(func(string) error { return nil }),
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
		Port:          16902,
		DesiredState:  domain.DesiredRunning,
		CreatedAt:     time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}
