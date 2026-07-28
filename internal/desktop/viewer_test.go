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

func TestViewerPageEmbedsRunningAgentURL(t *testing.T) {
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

	viewerHandler(view).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{
		"<title>Pantalk Ghost — Agent Launcher</title>",
		`src="http://127.0.0.1:16902"`,
		`allow="clipboard-read; clipboard-write; fullscreen"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("viewer page missing %q", expected)
		}
	}
}
