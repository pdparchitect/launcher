package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
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
	focused := 0
	raisable := true
	windows := &viewerWindows{
		open: map[string]int{},
		focus: func(pid int) bool {
			focused = pid
			return raisable
		},
	}

	if windows.focusOrClaim("Pantalk Ghost") {
		t.Fatal("first open must start a process")
	}
	if !windows.focusOrClaim("Pantalk Ghost") {
		t.Fatal("a click while the window is opening must not start a second")
	}
	if focused != 0 {
		t.Fatalf("nothing to focus yet, focused pid = %d", focused)
	}

	windows.track("Pantalk Ghost", 4242)
	if !windows.focusOrClaim("Pantalk Ghost") {
		t.Fatal("an open window must be focused, not duplicated")
	}
	if focused != 4242 {
		t.Fatalf("focused pid = %d, want the tracked process", focused)
	}

	// A different agent is a different window.
	if windows.focusOrClaim("Buzznode") {
		t.Fatal("each agent gets its own window")
	}

	// Closing the window ends the process, which frees the name again.
	windows.release("Pantalk Ghost")
	if windows.focusOrClaim("Pantalk Ghost") {
		t.Fatal("a closed window must open again")
	}

	// A window that cannot be raised is worth no more than none at all.
	windows.track("Pantalk Ghost", 99)
	raisable = false
	if windows.focusOrClaim("Pantalk Ghost") {
		t.Fatal("an unfocusable window must fall back to opening one")
	}
}
