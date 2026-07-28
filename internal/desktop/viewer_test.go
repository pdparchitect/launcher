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
			Port: 16902,
		},
		State: launchruntime.StatusRunning,
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	viewerHandler(view, "kasmvnc").ServeHTTP(response, request)

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
