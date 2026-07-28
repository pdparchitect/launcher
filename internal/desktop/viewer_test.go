package desktop

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

func TestViewerPageRedirectsToRunningAgentURL(t *testing.T) {
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

	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", response.Code)
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if location.Scheme != "http" || location.Host != "127.0.0.1:16902" ||
		location.Query().Get("resize") != "remote" ||
		location.Query().Get("show_control_bar") != "true" ||
		location.Query().Get("enable_threading") != "false" {
		t.Fatalf("Location = %q", location)
	}
}
