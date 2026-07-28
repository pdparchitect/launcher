package cli

import (
	"bytes"
	"context"
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
	}}}
	var stdout bytes.Buffer
	app := New(service, &fakeOpener{}, &stdout, &bytes.Buffer{}, "test")
	if code := app.Run(t.Context(), []string{"catalog"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Pantalk Ghost") {
		t.Fatalf("stdout = %q", stdout.String())
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

func TestCreateStartsByDefault(t *testing.T) {
	service := &fakeService{created: testInstance()}
	app := New(service, &fakeOpener{}, &bytes.Buffer{}, &bytes.Buffer{}, "test")
	if code := app.Run(
		t.Context(),
		[]string{"create", "--name", "Ada", "--image", "pantalk/ghost:test"},
	); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if !service.createOptions.Start ||
		service.createOptions.CatalogID != "pantalk-ghost" {
		t.Fatalf("CreateOptions = %#v", service.createOptions)
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

func TestOpenPrintsDesktopURL(t *testing.T) {
	service := &fakeService{view: agent.View{Instance: testInstance()}}
	opener := &fakeOpener{}
	var stdout bytes.Buffer
	app := New(service, opener, &stdout, &bytes.Buffer{}, "test")
	if code := app.Run(t.Context(), []string{"open", "Ada"}); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if opener.url != "http://127.0.0.1:16902" ||
		!strings.Contains(stdout.String(), opener.url) {
		t.Fatalf("opened = %q, stdout = %q", opener.url, stdout.String())
	}
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

func TestDesktopCommandOpensDesktop(t *testing.T) {
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

type fakeService struct {
	catalog       []agent.CatalogEntry
	created       domain.Instance
	createOptions agent.CreateOptions
	views         []agent.View
	view          agent.View
	deleteCalled  bool
	doctorErr     error
}

func (service *fakeService) Doctor(context.Context) (agent.DoctorReport, error) {
	if service.doctorErr != nil {
		return agent.DoctorReport{}, service.doctorErr
	}
	return agent.DoctorReport{
		Runtime: "container", Version: "test",
		Executable: "/usr/local/bin/container", DataRoot: "/tmp/launcher",
		DefaultImage: "pantalk/ghost:test",
	}, nil
}
func (service *fakeService) Catalog() []agent.CatalogEntry { return service.catalog }
func (service *fakeService) Create(
	_ context.Context,
	options agent.CreateOptions,
) (domain.Instance, error) {
	service.createOptions = options
	return service.created, nil
}
func (service *fakeService) List(context.Context) ([]agent.View, error) {
	return service.views, nil
}
func (service *fakeService) Get(context.Context, string) (agent.View, error) {
	return service.view, nil
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
func (*fakeService) Logs(context.Context, string, bool) error { return nil }

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
		Port:          16902,
		DesiredState:  domain.DesiredRunning,
		CreatedAt:     time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
}
