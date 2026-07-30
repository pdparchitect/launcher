package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
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

func TestInterfaceLeavesWindowControlsToNativeWindow(t *testing.T) {
	source := readWebSources(
		t,
		"desktop-window.js",
		"components/launcher-app.js",
		"styles.css",
	)
	for _, expected := range []string{
		"isDesktop()",
		"BrowserOpenURL",
		"ClipboardGetText",
		".is-macos-desktop .native-window-drag-region",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing retained desktop behavior %q", expected)
		}
	}
	for _, forbidden := range []string{
		"WindowMinimise",
		"WindowToggleMaximise",
		"invoke('Quit')",
		"data-window-action",
		"window-controls",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("interface still implements native window behavior %q", forbidden)
		}
	}
}

func TestOnlyMacOSNativeShellUsesWebDragRegion(t *testing.T) {
	styles := readWebSources(t, "styles.css")
	for _, expected := range []string{
		".is-macos-desktop .native-window-drag-region {",
		"--wails-draggable: drag;",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("macOS native shell missing drag bridge rule %q", expected)
		}
	}
	for _, forbidden := range []string{
		".dialog-heading {\n  --wails-draggable:",
		"--wails-draggable: no-drag",
		"body::after",
	} {
		if strings.Contains(styles, forbidden) {
			t.Fatalf("interface retains frameless-window support %q", forbidden)
		}
	}
}

func TestInterfaceUsesAntialiasedWebKitFontRendering(t *testing.T) {
	styles := readWebSources(t, "styles.css")

	if !strings.Contains(styles, "-webkit-font-smoothing: antialiased;") {
		t.Fatal("interface missing crisp WebKit font rendering")
	}
}

// The document must never rubber-band — that is the whole window bouncing and
// revealing its background. Content scrollers keep the bounce, which is what
// makes them feel native, so they use `contain` (no chaining) not `none`.
func TestInterfaceKeepsElasticScrollingInsideContentOnly(t *testing.T) {
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
		"overscroll-behavior: contain",
	} {
		if !strings.Contains(panelRule, expected) {
			t.Fatalf("main panel styles missing %q", expected)
		}
	}
	if strings.Contains(panelRule, "overscroll-behavior: none") {
		t.Fatal("main panel uses none, which also suppresses the elastic bounce")
	}
}

func TestMacOSDocumentScrollerKeepsInsetsWithoutRubberBanding(t *testing.T) {
	styles := readWebSources(t, "styles.css")
	macStart := strings.Index(
		styles,
		"/* macOS insets a web view's overlay scrollbar",
	)
	if macStart < 0 {
		t.Fatal("interface missing macOS document-scroller styles")
	}
	macEnd := strings.Index(
		styles[macStart:],
		"/* Browser preview (?chrome=macos)",
	)
	if macEnd < 0 {
		t.Fatal("macOS document-scroller styles are incomplete")
	}
	macRules := styles[macStart : macStart+macEnd]

	for _, expected := range []string{
		".is-swiftui-host launcher-app,",
		"height: auto",
		"min-height: 100%",
		"overflow: visible",
		".is-swiftui-host .main-panel {",
		"overflow-y: visible",
	} {
		if !strings.Contains(macRules, expected) {
			t.Fatalf(
				"macOS document scroller missing native-inset behavior %q",
				expected,
			)
		}
	}
	if strings.Contains(macRules, "overscroll-behavior: auto") {
		t.Fatal("macOS document scroller enables whole-window rubber-banding")
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

// Scrollbars are left to the platform, so the interface must not restyle them:
// on macOS the native overlay scroller is what matches the native sidebar.
// The ?chrome=macos preview is the only way to see the packaged macOS layout
// from a browser, so the wiring has to stay connected end to end: the flag is
// read, the layout class is applied, and the stylesheet acts on it.
// A shared constant used without being imported is a ReferenceError that only
// fires on the code path that touches it — silently skipping everything after
// it. Auto-formatting reorders import lists, so this cannot be eyeballed.
func TestInterfaceImportsEverySharedConstantItUses(t *testing.T) {
	module := readWebSources(t, "native-sidebar.js")
	consumer := readWebSources(t, "components/launcher-app.js")

	exported := regexp.MustCompile(`export const ([A-Z][A-Z0-9_]+)`).
		FindAllStringSubmatch(module, -1)
	if len(exported) == 0 {
		t.Fatal("found no exported constants to check")
	}

	importBlock := consumer
	if end := strings.Index(consumer, "from '../native-sidebar.js'"); end >= 0 {
		importBlock = consumer[:end]
	}

	for _, match := range exported {
		name := match[1]
		used := regexp.MustCompile(`\b` + name + `\b`).MatchString(consumer)
		if !used {
			continue
		}
		if !strings.Contains(importBlock, name) {
			t.Fatalf(
				"launcher-app.js uses %s but never imports it from native-sidebar.js",
				name,
			)
		}
	}
}

// Screens are swapped in place rather than navigated to, so nothing resets the
// scroll position on its own: paging down a long agent list and switching to
// Marketplace would otherwise open it halfway down.
func TestChangingScreenReturnsToTheTop(t *testing.T) {
	source := readWebSources(t, "components/launcher-app.js")

	start := strings.Index(source, "setScreen(screen) {")
	if start < 0 {
		t.Fatal("interface has no setScreen")
	}
	body := source[start:]
	body = body[:strings.Index(body, "\n  }")]
	if !strings.Contains(body, "this.scrollToTop()") {
		t.Fatal("setScreen must return the new screen to the top")
	}

	// Which scroller is live depends on the shell, so both are reset.
	for _, expected := range []string{
		"this.querySelector('.main-panel')?.scrollTo({ top: 0 })",
		"globalThis.scrollTo({ top: 0 })",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("scroll reset missing %q", expected)
		}
	}
}

// The hero runs to the window edges on every platform, passing beneath the
// sidebar, which is translucent so it seeps through. It is not a per-platform
// treatment, so it must not be gated behind a layout class.
func TestInterfaceBleedsTheHeroToTheWindowEdges(t *testing.T) {
	styles := readWebSources(t, "styles.css")

	for _, expected := range []string{
		// Cancels .content's padding rather than using viewport units, which
		// ignore that padding and land the hero off by half of it.
		"margin-left: calc(-1 * (var(--sidebar-width) + 26px));",
		"padding-left: calc(var(--sidebar-width) + 48px);",
		// The artwork bleeds past the top of the page, the copy does not: the
		// padding adds back what the negative margin took, so the first line
		// clears whatever the window is putting above the page.
		"padding: calc(var(--content-top) + 22px) 48px 44px;",
		// A single column, since .sidebar is position:fixed and overlays it.
		"grid-template-columns: minmax(0, 1fr);",
		// Without this the artwork is hidden behind the panel, not seen through it.
		"backdrop-filter: blur(24px) saturate(140%);",
		// The art runs past the section and dissolves, instead of being boxed
		// in by the hero's height.
		"inset: 0 0 -340px;",
		"z-index: -1;",
		"mask-image: linear-gradient(",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("hero bleed not wired: missing %q", expected)
		}
	}
	// An opaque hero or clipped overflow would each hide the backdrop.
	heroRule := styles[strings.Index(styles, "\n.hero {"):]
	heroRule = heroRule[:strings.Index(heroRule, "\n}")]
	for _, unwanted := range []string{"overflow: hidden", "background:"} {
		if strings.Contains(heroRule, unwanted) {
			t.Fatalf(".hero must not set %q or it covers/clips its own artwork", unwanted)
		}
	}
	if strings.Contains(styles, "border: 1px solid var(--line-bright);\n  background: #080a07;") {
		t.Fatal("hero still draws the separating border")
	}
}

func TestInterfacePreviewsMacosChromeFromTheBrowser(t *testing.T) {
	app := readWebSources(t, "components/launcher-app.js")
	styles := readWebSources(t, "styles.css")

	for _, expected := range []string{
		`.get('chrome')`,
		"function macOSChromePreviewRequested()",
		"macOSChromePreviewRequested()",
		"applyNativeSidebarLayout(true)",
		"'is-sidebar-preview'",
	} {
		if !strings.Contains(app, expected) {
			t.Fatalf("macOS chrome preview not wired: missing %q", expected)
		}
	}
	for _, expected := range []string{
		".launcher-shell.is-sidebar-preview .sidebar {",
	} {
		if !strings.Contains(styles, expected) {
			t.Fatalf("macOS chrome preview not styled: missing %q", expected)
		}
	}
}

func TestInterfaceLeavesScrollbarsToThePlatform(t *testing.T) {
	styles := readWebSources(t, "styles.css")

	for _, unwanted := range []string{
		"scrollbar-width",
		"scrollbar-color",
		"::-webkit-scrollbar",
	} {
		if strings.Contains(styles, unwanted) {
			t.Fatalf("interface restyles scrollbars with %q", unwanted)
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
		"desktop-window.js",
		"api.js",
	)
	for _, expected := range []string{
		"metrics?.cpuPercent",
		"metrics?.memoryPercent",
		"metrics?.uptimeSeconds",
		"item.id === agent.catalogId",
		"setInterval(() => this.refreshAgents(), 5000)",
		"async openAgent(agent)",
		"desktopWindow.openExternal(agent.url)",
		"await this.api.openViewer(agent.id)",
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

func TestAgentOpeningDoesNotEmbedRemoteDesktop(t *testing.T) {
	source := readWebSources(
		t,
		"index.html",
		"components/launcher-app.js",
	)
	for _, forbidden := range []string{
		"agent-viewer-dialog",
		"data-viewer-frame",
		"pop-out-agent",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("interface still embeds remote desktop using %q", forbidden)
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

func TestLauncherUpdateAppearsInBannerAndHomeOverview(t *testing.T) {
	source := readWebSources(
		t,
		"api.js",
		"desktop-window.js",
		"components/launcher-app.js",
		"styles.css",
	)
	for _, expected := range []string{
		"return this.request('/api/launcher')",
		"data-launcher-update",
		"LAUNCHER UPDATE AVAILABLE",
		"VIEW RELEASE",
		"REMIND ME LATER",
		"refreshLauncherUpdate()",
		"launcher-dismissed-update",
		"desktopWindow.openExternal(releaseURL)",
		"overview-stat--update",
		"V${this.launcherStatus.latestVersion} AVAILABLE",
		".launcher-update-banner {",
		"grid-template-columns: repeat(4, minmax(0, 1fr));",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("interface missing Launcher update behavior %q", expected)
		}
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
		"components/marketplace-detail.js",
		"components/deploy-dialog.js",
		"components/agent-actions-dialog.js",
		"components/runtime-setup-dialog.js",
	)
	for _, expected := range []string{
		`customElements.define('launcher-app'`,
		`customElements.define('agent-card'`,
		`customElements.define('marketplace-card'`,
		`customElements.define('marketplace-detail'`,
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

func TestMarketplaceEntriesOpenDetailedImagePages(t *testing.T) {
	source := readWebSources(
		t,
		"components/launcher-app.js",
		"components/marketplace-card.js",
		"components/marketplace-detail.js",
		"styles.css",
	)
	for _, expected := range []string{
		`data-screen="marketplace-detail"`,
		"view-marketplace-entry",
		"showMarketplaceEntry(entry)",
		"entry.media?.screenshots?.[0]",
		"agent.catalogId === entry.id",
		"instance.state === 'running'",
		`class="hero marketplace-detail__hero"`,
		`class="hero__art marketplace-detail__hero-art"`,
		`class="hero__copy"`,
		`class="hero__stat"`,
		"--marketplace-detail-art",
		`data-screenshots`,
		".marketplace-screenshot-list img",
		`data-screen-link="marketplace"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("marketplace detail page missing %q", expected)
		}
	}
}

func TestMarketplaceDistinguishesLoadingFromUnavailable(t *testing.T) {
	source := readWebSources(
		t,
		"components/launcher-app.js",
		"styles.css",
	)
	for _, expected := range []string{
		"this.catalogLoading = true",
		"this.catalogLoading = false",
		"LOADING MARKETPLACE",
		"Fetching application publishers and their latest releases.",
		`loading.setAttribute('role', 'status')`,
		`loading.setAttribute('aria-live', 'polite')`,
		".loading-state > span",
		"marketplace-loading",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("marketplace loading state missing %q", expected)
		}
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
	var openedURL string
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
	if openedURL != instance.URL() {
		t.Fatalf("opened URL = %q, want %q", openedURL, instance.URL())
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
		Port:          16902,
		DesiredState:  domain.DesiredRunning,
		CreatedAt:     time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}
