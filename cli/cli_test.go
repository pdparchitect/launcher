package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

func TestCatalogListsGhost(t *testing.T) {
	service := &fakeService{catalog: []agent.CatalogEntry{{
		ID:   "370a2228-322d-4089-846b-62fb8c15d154",
		Slug: "pantalk-ghost", Name: "Pantalk Ghost", Publisher: "Pantalk",
		Image: "ghcr.io/pantalk/ghost@sha256:catalogue-image",
	}}}
	var stdout bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &bytes.Buffer{}, "test")
	if code := app.Run(t.Context(), []string{"catalog"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	for _, expected := range []string{
		"IMAGE",
		"Pantalk Ghost",
		"ghcr.io/pantalk/ghost@sha256:catalogue-image",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("stdout = %q, missing %q", stdout.String(), expected)
		}
	}
}

func TestGuidePrintsCurrentAgentInstructions(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New(&fakeService{}, &fakeOpener{}, &stdout, &stderr, "test")

	if code := app.Run(t.Context(), []string{"guide"}); code != 0 {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
	for _, expected := range []string{
		"# Launcher agent guide",
		"launcher create --app SLUG_OR_ID NAME",
		"Never guess or silently choose an application.",
		"launcher viewer NAME",
		"launcher preview --output PATH NAME",
		"launcher duplicate SOURCE NEW_NAME",
		"launcher exec NAME COMMAND [ARG...]",
		"launcher delete --force NAME",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("guide missing %q", expected)
		}
	}
}

func TestGuideRejectsArguments(t *testing.T) {
	var stderr bytes.Buffer
	app := New(&fakeService{}, &fakeOpener{}, &bytes.Buffer{}, &stderr, "test")

	if code := app.Run(t.Context(), []string{"guide", "extra"}); code == 0 {
		t.Fatal("Run() code = 0")
	}
	if !strings.Contains(stderr.String(), "does not accept arguments") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCatalogCanRefreshBeforeListing(t *testing.T) {
	service := &fakeService{}
	var stdout bytes.Buffer
	refreshed := false
	app := New(
		service,
		&fakeOpener{},
		&stdout,
		&bytes.Buffer{},
		"test",
		WithCatalogRefresh(func(context.Context) (bool, error) {
			refreshed = true
			service.catalog = []agent.CatalogEntry{{
				Slug: "remote", Name: "Remote Agent", Publisher: "Test",
			}}
			return true, nil
		}),
	)

	if code := app.Run(
		t.Context(),
		[]string{"catalog", "--refresh"},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !refreshed ||
		!strings.Contains(stdout.String(), "Application registry refreshed.") ||
		!strings.Contains(stdout.String(), "Remote Agent") {
		t.Fatalf("refreshed = %v, stdout = %q", refreshed, stdout.String())
	}
}

func TestListPrintsAgentTable(t *testing.T) {
	service := &fakeService{views: []agent.View{{
		Instance: testInstance(),
		State:    launchruntime.StatusRunning,
	}}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &stderr, "test")
	if code := app.Run(t.Context(), []string{"list"}); code != 0 {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
	for _, value := range []string{"NAME", "Ada", "running", "16902"} {
		if !strings.Contains(stdout.String(), value) {
			t.Fatalf("stdout = %q, missing %q", stdout.String(), value)
		}
	}
}

func TestStatusPrintsFilesInterfacesNetworkAndMetrics(t *testing.T) {
	instance := testInstance()
	instance.Interfaces["preview"] = domain.Interface{
		Kind: "preview", Port: 16903, Path: "/preview.jpg",
	}
	service := &fakeService{details: agent.Details{
		View: agent.View{
			Instance: instance,
			State:    launchruntime.StatusRunning,
			Metrics: launchruntime.Metrics{
				CPUPercent: 1.25, CPUAvailable: true,
				MemoryPercent: 12.5, MemoryAvailable: true,
				MemoryUsageBytes: 128 * 1024 * 1024,
				MemoryLimitBytes: 1024 * 1024 * 1024,
			},
			Uptime: 5 * time.Minute,
		},
		Files: "/tmp/launcher/agents/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Mounts: []agent.MountDetails{{
			Name: "workspace", Target: "/workspace", Storage: "host",
			Source: "/tmp/launcher/agents/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/workspace",
		}},
		Network: launchruntime.NetworkInfo{
			Name:     "launcher-agent-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Attached: true, Addresses: []string{"172.20.0.2"},
		},
	}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &stderr, "test")

	if code := app.Run(t.Context(), []string{"status", "Ada"}); code != 0 {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}

	for _, expected := range []string{
		"Files:         /tmp/launcher/agents/",
		"Mount workspace:",
		"Interface desktop: http://127.0.0.1:16902/ (kasmweb)",
		"Interface preview: http://127.0.0.1:16903/preview.jpg (preview)",
		"Network:       launcher-agent-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"Attached:      true",
		"IP address:    172.20.0.2",
		"Uptime:        5m0s",
		"CPU:           1.25%",
		"Memory:        12.50% (128.0 MiB / 1.0 GiB)",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("stdout = %q, missing %q", stdout.String(), expected)
		}
	}
}

func TestCreateStartsByDefault(t *testing.T) {
	service := &fakeService{created: testInstance()}
	app := New(service, &fakeOpener{}, &bytes.Buffer{}, &bytes.Buffer{}, "test")
	if code := app.Run(
		t.Context(),
		[]string{
			"create", "--app", "pantalk-ghost", "--name", "Ada",
			"--image", "pantalk/ghost:test",
		},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !service.createOptions.Start ||
		service.createOptions.CatalogID != "pantalk-ghost" {
		t.Fatalf("CreateOptions = %#v", service.createOptions)
	}
}

func TestCreateRequiresApplication(t *testing.T) {
	service := &fakeService{created: testInstance()}
	var stderr bytes.Buffer
	app := New(service, &fakeOpener{}, &bytes.Buffer{}, &stderr, "test")

	if code := app.Run(t.Context(), []string{"create", "Ada"}); code == 0 {
		t.Fatal("Run() code = 0")
	}
	if service.createCalled || !strings.Contains(stderr.String(), "requires --app") {
		t.Fatalf(
			"CreateOptions = %#v, stderr = %q",
			service.createOptions,
			stderr.String(),
		)
	}
}

func TestDuplicateCopiesAgentStoppedByDefault(t *testing.T) {
	duplicate := testInstance()
	duplicate.Name = "Ada Copy"
	duplicate.DesiredState = domain.DesiredStopped
	service := &fakeService{duplicated: duplicate}
	var stdout bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &bytes.Buffer{}, "test")

	if code := app.Run(
		t.Context(),
		[]string{"duplicate", "Ada", "Ada Copy"},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if service.duplicateReference != "Ada" ||
		service.duplicateOptions.Name != "Ada Copy" ||
		service.duplicateOptions.Start {
		t.Fatalf(
			"reference = %q, options = %#v",
			service.duplicateReference,
			service.duplicateOptions,
		)
	}
	for _, expected := range []string{
		"Duplicated Ada as Ada Copy",
		`launcher start "Ada Copy"`,
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("stdout = %q, missing %q", stdout.String(), expected)
		}
	}
}

func TestDuplicateCanStartTheCopy(t *testing.T) {
	duplicate := testInstance()
	duplicate.Name = "Ada Copy"
	service := &fakeService{duplicated: duplicate}
	app := New(service, &fakeOpener{}, &bytes.Buffer{}, &bytes.Buffer{}, "test")

	if code := app.Run(
		t.Context(),
		[]string{"duplicate", "--start", "Ada", "Ada Copy"},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !service.duplicateOptions.Start {
		t.Fatalf("DuplicateOptions = %#v", service.duplicateOptions)
	}
}

func TestDeleteRequiresForce(t *testing.T) {
	service := &fakeService{}
	var stderr bytes.Buffer
	app := New(service, &fakeOpener{}, &bytes.Buffer{}, &stderr, "test")
	if code := app.Run(t.Context(), []string{"delete", "Ada"}); code == 0 {
		t.Fatal("Run() code = 0")
	}
	if service.deleteCalled || !strings.Contains(stderr.String(), "--force") {
		t.Fatalf("deleteCalled = %v, stderr = %q", service.deleteCalled, stderr.String())
	}
}

func TestCleanupPrintsImageCleanupReport(t *testing.T) {
	service := &fakeService{cleanupReport: agent.ImageCleanupReport{
		Tracked: 4, Protected: 2, Deferred: 1, Removed: 1,
	}}
	var stdout bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &bytes.Buffer{}, "test")

	if code := app.Run(t.Context(), []string{"cleanup"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if service.cleanupMinimumAge != agent.DefaultImageRetention ||
		!strings.Contains(stdout.String(), "Removed 1 unused image") {
		t.Fatalf(
			"cleanup age = %v, stdout = %q",
			service.cleanupMinimumAge,
			stdout.String(),
		)
	}
}

func TestOpenPrintsDesktopURL(t *testing.T) {
	service := &fakeService{view: agent.View{Instance: testInstance()}}
	opener := &fakeOpener{}
	var stdout bytes.Buffer
	app := New(service, opener, &stdout, &bytes.Buffer{}, "test")
	if code := app.Run(t.Context(), []string{"open", "Ada"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if opener.url != "http://127.0.0.1:16902/" ||
		!strings.Contains(stdout.String(), opener.url) {
		t.Fatalf("opened = %q, stdout = %q", opener.url, stdout.String())
	}
}

func TestPreviewSavesDeclaredImageAfterRetry(t *testing.T) {
	image := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		_ *http.Request,
	) {
		requests++
		if requests == 1 {
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		response.Header().Set("Content-Type", "image/jpeg")
		_, _ = response.Write(image)
	}))
	defer server.Close()
	service := &fakeService{view: testPreviewView(t, server.URL, launchruntime.StatusRunning)}
	var stdout bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &bytes.Buffer{}, "test")
	app.preview.client = server.Client()
	app.preview.attempts = 2
	app.preview.retryDelay = 0
	destination := filepath.Join(t.TempDir(), "ada.jpg")

	if code := app.Run(t.Context(), []string{
		"preview", "--output", destination, "Ada",
	}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	data, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, image) {
		t.Fatalf("saved preview = %v, %v", data, err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved preview mode = %v", info.Mode().Perm())
	}
	if requests != 2 || !strings.Contains(stdout.String(), destination) {
		t.Fatalf("requests = %d, stdout = %q", requests, stdout.String())
	}
}

func TestPreviewRefusesToOverwriteWithoutForce(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "ada.jpg")
	if err := os.WriteFile(destination, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var stderr bytes.Buffer
	app := New(&fakeService{}, &fakeOpener{}, &bytes.Buffer{}, &stderr, "test")

	if code := app.Run(t.Context(), []string{
		"preview", "--output", destination, "Ada",
	}); code == 0 {
		t.Fatal("Run() code = 0")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "original" ||
		!strings.Contains(stderr.String(), "use --force") {
		t.Fatalf("destination = %q, error = %v, stderr = %q", data, err, stderr.String())
	}
}

func TestPreviewForceReplacesExistingFile(t *testing.T) {
	image := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = response.Write(image)
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "ada.jpg")
	if err := os.WriteFile(destination, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	app := New(
		&fakeService{view: testPreviewView(t, server.URL, launchruntime.StatusRunning)},
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
	)
	app.preview.client = server.Client()

	if code := app.Run(t.Context(), []string{
		"preview", "--output", destination, "--force", "Ada",
	}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	data, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, image) {
		t.Fatalf("saved preview = %v, %v", data, err)
	}
}

func TestPreviewRejectsNonImageAndOversizedResponses(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		maxBytes int64
		want     string
	}{
		{name: "non-image", data: []byte("not an image"), maxBytes: 1024, want: "instead of an image"},
		{
			name:     "oversized",
			data:     []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00},
			maxBytes: 4,
			want:     "size limit",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "preview.jpg")

			err := writePreviewAtomically(
				destination,
				bytes.NewReader(test.data),
				false,
				test.maxBytes,
			)

			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("writePreviewAtomically() error = %v", err)
			}
			if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("destination error = %v", statErr)
			}
		})
	}
}

func TestPreviewRequiresRunningAgentWithDeclaredInterface(t *testing.T) {
	tests := []struct {
		name string
		view agent.View
		want string
	}{
		{
			name: "stopped",
			view: agent.View{
				Instance: testInstance(),
				State:    launchruntime.StatusStopped,
			},
			want: "must be running",
		},
		{
			name: "missing preview",
			view: agent.View{
				Instance: testInstance(),
				State:    launchruntime.StatusRunning,
			},
			want: "does not expose a preview interface",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			app := New(
				&fakeService{view: test.view},
				&fakeOpener{},
				&bytes.Buffer{},
				&stderr,
				"test",
			)
			destination := filepath.Join(t.TempDir(), "ada.jpg")

			if code := app.Run(t.Context(), []string{
				"preview", "--output", destination, "Ada",
			}); code == 0 {
				t.Fatal("Run() code = 0")
			}
			if !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func testPreviewView(
	t *testing.T,
	serverURL string,
	state launchruntime.Status,
) agent.View {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi() error = %v", err)
	}
	instance := testInstance()
	instance.Interfaces["preview"] = domain.Interface{
		Kind: "preview", Port: port, Path: "/preview.jpg",
	}
	return agent.View{Instance: instance, State: state}
}

func TestDoctorOffersRuntimeInstaller(t *testing.T) {
	installError := &fakeInstallError{}
	service := &fakeService{doctorErr: installError}
	opener := &fakeOpener{}
	var stdout bytes.Buffer
	app := New(
		service,
		opener,
		&stdout,
		&bytes.Buffer{},
		"test",
		WithInput(strings.NewReader("y\n")),
	)
	if code := app.Run(t.Context(), []string{"doctor"}); code == 0 {
		t.Fatal("Run() code = 0, want unavailable status")
	}
	if opener.url != installError.InstallURL() ||
		!strings.Contains(stdout.String(), "signed installer") {
		t.Fatalf("opened = %q, stdout = %q", opener.url, stdout.String())
	}
}

func TestDoctorPrintsResolvedRuntimeExecutable(t *testing.T) {
	service := &fakeService{}
	var stdout bytes.Buffer
	app := New(
		service,
		&fakeOpener{},
		&stdout,
		&bytes.Buffer{},
		"test",
	)
	if code := app.Run(t.Context(), []string{"doctor", "--no-prompt"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !strings.Contains(
		stdout.String(),
		"Executable:    /usr/local/bin/container",
	) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestServePassesLocalServerOptions(t *testing.T) {
	service := &fakeService{}
	var received ServeOptions
	app := New(
		service,
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		WithServer(func(_ context.Context, options ServeOptions) error {
			received = options
			return nil
		}),
	)
	if code := app.Run(
		t.Context(),
		[]string{"serve", "--listen", "127.0.0.1:17000", "--no-open"},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if received.Listen != "127.0.0.1:17000" || received.Open {
		t.Fatalf("ServeOptions = %#v", received)
	}
}

func TestNoCommandOpensDesktopWhenAvailable(t *testing.T) {
	service := &fakeService{}
	called := false
	app := New(
		service,
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		WithDesktop(func(context.Context) error {
			called = true
			return nil
		}),
	)

	if code := app.Run(t.Context(), nil); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !called {
		t.Fatal("desktop was not opened")
	}
}

func TestNoCommandPrintsHelpWhenTerminalIsAttached(t *testing.T) {
	service := &fakeService{}
	called := false
	var stdout bytes.Buffer
	app := New(
		service,
		&fakeOpener{},
		&stdout,
		&bytes.Buffer{},
		"test",
		WithTerminalAttached(true),
		WithDesktop(func(context.Context) error {
			called = true
			return nil
		}),
	)

	if code := app.Run(t.Context(), nil); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if called || !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("desktop called = %v, stdout = %q", called, stdout.String())
	}
}

func TestDesktopCommandOpensDesktop(t *testing.T) {
	service := &fakeService{}
	called := false
	app := New(
		service,
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		WithTerminalAttached(true),
		WithDesktop(func(context.Context) error {
			called = true
			return nil
		}),
	)

	if code := app.Run(t.Context(), []string{"desktop"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !called {
		t.Fatal("desktop was not opened")
	}
}

func TestViewerCommandOpensFramedAgentViewer(t *testing.T) {
	service := &fakeService{}
	var reference string
	app := New(
		service,
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		WithViewer(func(_ context.Context, received string) error {
			reference = received
			return nil
		}),
	)

	if code := app.Run(
		t.Context(),
		[]string{"viewer", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if reference != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("viewer reference = %q", reference)
	}
}

func TestViewerCommandWithURLSkipsRuntimeResolution(t *testing.T) {
	service := &fakeService{}
	var gotID, gotName, gotURL, gotKind string
	resolved := false
	app := New(
		service,
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		WithViewer(func(context.Context, string) error {
			resolved = true
			return nil
		}),
		WithViewerTarget(func(
			_ context.Context,
			id, name, url, kind string,
		) error {
			gotID, gotName, gotURL, gotKind = id, name, url, kind
			return nil
		}),
	)

	if code := app.Run(t.Context(), []string{
		"viewer",
		"-id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-url", "http://127.0.0.1:16902",
		"-name", "Pantalk Ghost",
		"-kind", "kasmweb",
	}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	// Taking the resolving path would mean another container inspect, which is
	// the latency this flag exists to avoid.
	if resolved {
		t.Fatal("viewer -url must not resolve the agent through the runtime")
	}
	if gotURL != "http://127.0.0.1:16902" {
		t.Fatalf("url = %q", gotURL)
	}
	if gotName != "Pantalk Ghost" || gotKind != "kasmweb" {
		t.Fatalf("name = %q, kind = %q", gotName, gotKind)
	}
	if gotID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("id = %q", gotID)
	}
}

func TestExecPassesCommandAndTerminalStreamsToService(t *testing.T) {
	service := &fakeService{}
	input := strings.NewReader("input")
	app := New(
		service,
		&fakeOpener{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		WithInput(input),
	)

	if code := app.Run(t.Context(), []string{
		"exec",
		"--tty",
		"Ada",
		"sh",
		"-c",
		"printf '%s' \"$HOME\"",
	}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if service.execReference != "Ada" {
		t.Fatalf("exec reference = %q", service.execReference)
	}
	wantCommand := []string{"sh", "-c", "printf '%s' \"$HOME\""}
	if !slices.Equal(service.execOptions.Command, wantCommand) {
		t.Fatalf(
			"exec command = %#v, want %#v",
			service.execOptions.Command,
			wantCommand,
		)
	}
	if service.execOptions.Stdin != input || !service.execOptions.TTY {
		t.Fatalf("exec options = %#v", service.execOptions)
	}
}

type fakeService struct {
	catalog            []agent.CatalogEntry
	created            domain.Instance
	createCalled       bool
	createOptions      agent.CreateOptions
	duplicated         domain.Instance
	duplicateReference string
	duplicateOptions   agent.DuplicateOptions
	views              []agent.View
	view               agent.View
	details            agent.Details
	deleteCalled       bool
	doctorErr          error
	cleanupReport      agent.ImageCleanupReport
	cleanupMinimumAge  time.Duration
	execReference      string
	execOptions        agent.ExecOptions
}

func (service *fakeService) Doctor(context.Context) (agent.DoctorReport, error) {
	if service.doctorErr != nil {
		return agent.DoctorReport{}, service.doctorErr
	}
	return agent.DoctorReport{
		Runtime: "container", Version: "test",
		Executable: "/usr/local/bin/container", DataRoot: "/tmp/launcher",
	}, nil
}
func (service *fakeService) Catalog() []agent.CatalogEntry { return service.catalog }
func (service *fakeService) Create(
	_ context.Context,
	options agent.CreateOptions,
) (domain.Instance, error) {
	service.createCalled = true
	service.createOptions = options
	return service.created, nil
}
func (service *fakeService) Duplicate(
	_ context.Context,
	reference string,
	options agent.DuplicateOptions,
) (domain.Instance, error) {
	service.duplicateReference = reference
	service.duplicateOptions = options
	return service.duplicated, nil
}
func (service *fakeService) List(context.Context) ([]agent.View, error) {
	return service.views, nil
}
func (service *fakeService) Get(context.Context, string) (agent.View, error) {
	return service.view, nil
}
func (service *fakeService) Details(context.Context, string) (agent.Details, error) {
	return service.details, nil
}
func (*fakeService) Start(context.Context, string) (domain.Instance, error) {
	return testInstance(), nil
}
func (*fakeService) Stop(context.Context, string) (domain.Instance, error) {
	return testInstance(), nil
}
func (service *fakeService) Delete(context.Context, string) error {
	service.deleteCalled = true
	return nil
}
func (service *fakeService) CleanupImages(
	_ context.Context,
	minimumAge time.Duration,
) (agent.ImageCleanupReport, error) {
	service.cleanupMinimumAge = minimumAge
	return service.cleanupReport, nil
}
func (*fakeService) Logs(context.Context, string, bool) error { return nil }
func (service *fakeService) Exec(
	_ context.Context,
	reference string,
	options agent.ExecOptions,
) error {
	service.execReference = reference
	service.execOptions = options
	return nil
}

type fakeOpener struct{ url string }

func (opener *fakeOpener) Open(url string) error {
	opener.url = url
	return nil
}

type fakeInstallError struct{}

func (*fakeInstallError) Error() string       { return "runtime missing" }
func (*fakeInstallError) RuntimeName() string { return "Apple container" }
func (*fakeInstallError) InstallURL() string {
	return "https://github.com/apple/container/releases/latest"
}
func (*fakeInstallError) InstallGuidance() string {
	return "Download and run the signed installer."
}

func testInstance() domain.Instance {
	return domain.Instance{
		ID:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CatalogID:     "370a2228-322d-4089-846b-62fb8c15d154",
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
