package desktop

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	"github.com/pdparchitect/launcher/internal/httpapi"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/wailsapp/wails/v2/pkg/menu"
)

func TestViewerPageNavigatesToRunningAgentURL(t *testing.T) {
	view := agent.View{
		Instance: domain.Instance{
			ID:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Name: "Pantalk Ghost",
			Interfaces: map[string]domain.Interface{
				"desktop": {Kind: "kasmweb", Port: 16902, Path: "/"},
			},
		},
		State: launchruntime.StatusRunning,
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	viewerHandler(
		view.Name,
		view.Interfaces["desktop"].URL(),
		"kasmweb",
	).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Location") != "" {
		t.Fatalf("unexpected Location = %q", response.Header().Get("Location"))
	}
	body := response.Body.String()
	for _, expected := range []string{
		"window.location.replace(",
		"http://127.0.0.1:16902",
		"resize=remote",
		"show_control_bar=true",
		"enable_threading=false",
		"Open Pantalk Ghost",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("viewer page missing %q: %s", expected, body)
		}
	}
}

// Every viewer window is a separate process showing the same container, so a
// second one for the same agent is only ever in the way. The bookkeeping is
// what decides between starting one and raising the one already there, and it
// has to survive a click that lands while the first window is still opening.
func TestOpeningAnAgentTwiceRaisesTheWindowAlreadyOpen(t *testing.T) {
	ghostID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	buzznodeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	focused := 0
	raisable := true
	windows := &viewerWindows{
		open: map[string]int{},
		focus: func(pid int) bool {
			focused = pid
			return raisable
		},
	}

	if windows.focusOrClaim(ghostID) {
		t.Fatal("first open must start a process")
	}
	if !windows.focusOrClaim(ghostID) {
		t.Fatal("a click while the window is opening must not start a second")
	}
	if focused != 0 {
		t.Fatalf("nothing to focus yet, focused pid = %d", focused)
	}

	windows.track(ghostID, 4242)
	if !windows.focusOrClaim(ghostID) {
		t.Fatal("an open window must be focused, not duplicated")
	}
	if focused != 4242 {
		t.Fatalf("focused pid = %d, want the tracked process", focused)
	}

	// A different agent is a different window.
	if windows.focusOrClaim(buzznodeID) {
		t.Fatal("each agent gets its own window")
	}

	// Closing the window ends the process, which frees the agent ID again.
	windows.release(ghostID)
	if windows.focusOrClaim(ghostID) {
		t.Fatal("a closed window must open again")
	}

	// A window that cannot be raised is worth no more than none at all.
	windows.track(ghostID, 99)
	raisable = false
	if windows.focusOrClaim(ghostID) {
		t.Fatal("an unfocusable window must fall back to opening one")
	}
}

func TestViewerProcessReceivesImmutableAgentID(t *testing.T) {
	target := httpapi.ViewerTarget{
		ID:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name: "Pantalk Ghost",
		URL:  "http://127.0.0.1:16902",
		Kind: "kasmweb",
	}

	want := []string{
		"viewer",
		"--id", target.ID,
		"--url", target.URL,
		"--name", target.Name,
		"--kind", target.Kind,
	}
	if got := viewerCommandArguments(target); !slices.Equal(got, want) {
		t.Fatalf("viewer arguments = %#v, want %#v", got, want)
	}
}

func TestViewerActionsOpenAgentFiles(t *testing.T) {
	service := &fakeViewerService{filesPath: "/tmp/launcher/agents/a/workspace"}
	var opened string
	actions := newViewerActions(
		t.Context(),
		service,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"Pantalk Ghost",
		ViewerOptions{OpenPath: func(path string) error {
			opened = path
			return nil
		}},
	)

	if err := actions.openFiles(); err != nil {
		t.Fatalf("openFiles() error = %v", err)
	}
	if service.filesReference != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("files reference = %q", service.filesReference)
	}
	if opened != service.filesPath {
		t.Fatalf("opened path = %q, want %q", opened, service.filesPath)
	}
}

func TestViewerStopClosesOnlyAfterAgentStops(t *testing.T) {
	service := &fakeViewerService{}
	closed := false
	actions := newViewerActions(
		t.Context(),
		service,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"Pantalk Ghost",
		ViewerOptions{},
	)
	actions.setHost(func() { closed = true }, nil, nil)
	stopItem := viewerApplicationMenu(actions).Items[1].SubMenu.Items[3]

	stopItem.Click(&menu.CallbackData{MenuItem: stopItem})
	if service.stopReference != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("stop reference = %q", service.stopReference)
	}
	if !closed {
		t.Fatal("viewer did not close after the agent stopped")
	}

	service.stopErr = errors.New("runtime unavailable")
	closed = false
	stopItem.Click(&menu.CallbackData{MenuItem: stopItem})
	if closed {
		t.Fatal("viewer closed after the agent failed to stop")
	}
}

func TestViewerRenameUsesAgentIDAndUpdatesWindowTitle(t *testing.T) {
	service := &fakeViewerService{}
	var promptDefault string
	var windowName string
	actions := newViewerActions(
		t.Context(),
		service,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"Pantalk Ghost",
		ViewerOptions{},
	)
	actions.setRenameHost(
		func(_, _, defaultValue string) (string, bool) {
			promptDefault = defaultValue
			return "Grace", true
		},
		func(name string) { windowName = name },
	)
	renameItem := viewerApplicationMenu(actions).Items[1].SubMenu.Items[1]

	renameItem.Click(&menu.CallbackData{MenuItem: renameItem})

	if promptDefault != "Pantalk Ghost" {
		t.Fatalf("rename prompt default = %q", promptDefault)
	}
	if service.renameReference != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		service.renameName != "Grace" {
		t.Fatalf(
			"rename = (%q, %q)",
			service.renameReference,
			service.renameName,
		)
	}
	if windowName != "Grace" {
		t.Fatalf("window name = %q", windowName)
	}
}

func TestViewerApplicationMenuExposesContainerActions(t *testing.T) {
	actions := newViewerActions(
		t.Context(),
		&fakeViewerService{},
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"Pantalk Ghost",
		ViewerOptions{},
	)
	applicationMenu := viewerApplicationMenu(actions)
	if len(applicationMenu.Items) != 4 {
		t.Fatalf("top-level menu count = %d", len(applicationMenu.Items))
	}
	fileMenu := applicationMenu.Items[1]
	if fileMenu.Label != "File" || fileMenu.SubMenu == nil {
		t.Fatalf("File menu = %#v", fileMenu)
	}
	var labels []string
	for _, item := range fileMenu.SubMenu.Items {
		if !item.IsSeparator() {
			labels = append(labels, item.Label)
		}
	}
	want := []string{
		"Open Agent Files in Finder",
		"Rename Agent…",
		"Stop Agent and Close Window",
		"Close Window",
	}
	if !slices.Equal(labels, want) {
		t.Fatalf("File menu labels = %#v, want %#v", labels, want)
	}
}

type fakeViewerService struct {
	filesPath       string
	filesReference  string
	stopReference   string
	stopErr         error
	renameReference string
	renameName      string
}

func (*fakeViewerService) Get(context.Context, string) (agent.View, error) {
	return agent.View{}, nil
}

func (service *fakeViewerService) Stop(
	_ context.Context,
	reference string,
) (domain.Instance, error) {
	service.stopReference = reference
	return domain.Instance{}, service.stopErr
}

func (service *fakeViewerService) AgentFiles(
	_ context.Context,
	reference string,
) (string, error) {
	service.filesReference = reference
	return service.filesPath, nil
}

func (service *fakeViewerService) Rename(
	_ context.Context,
	reference string,
	name string,
) (domain.Instance, error) {
	service.renameReference = reference
	service.renameName = name
	return domain.Instance{Name: name}, nil
}
