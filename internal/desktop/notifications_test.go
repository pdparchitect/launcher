package desktop

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
)

type notificationAgentServiceStub struct {
	views []agent.View
	err   error
}

func (service notificationAgentServiceStub) ListWithIssues(
	context.Context,
) ([]agent.View, []store.Issue, error) {
	return service.views, nil, service.err
}

func TestNotificationPollerDiscoversAndAdvancesCursor(t *testing.T) {
	var mutex sync.Mutex
	requests := make([]string, 0, 2)
	bridge := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		mutex.Lock()
		requests = append(requests, request.URL.Query().Get("cursor"))
		mutex.Unlock()
		response.Header().Set("Content-Type", "application/json")
		page := bridgeNotificationPage{NextCursor: "generation.1"}
		if request.URL.Query().Get("cursor") == "" {
			page.Notifications = []bridgeNotification{
				{
					ID:    7,
					App:   "Hermes",
					Title: "Task complete",
					Body:  "Finished",
				},
			}
		}
		_ = json.NewEncoder(response).Encode(page)
	}))
	defer bridge.Close()

	port := serverPort(t, bridge.URL)
	service := notificationAgentServiceStub{views: []agent.View{
		{
			Instance: domain.Instance{
				ID:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Name: "Research",
				Interfaces: map[string]domain.Interface{
					"notifications": {
						Kind: "notifications",
						Port: port,
						Path: "/notifications",
					},
				},
			},
			State: launchruntime.StatusRunning,
		},
	}}
	delivered := make([]bridgeNotification, 0, 1)
	poller := newNotificationPoller(
		t.TempDir(),
		service,
		func(_ notificationSource, item bridgeNotification) error {
			delivered = append(delivered, item)
			return nil
		},
	)
	if err := poller.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	poller.poll(context.Background())
	poller.poll(context.Background())

	if len(delivered) != 1 || delivered[0].ID != 7 {
		t.Fatalf("delivered = %#v", delivered)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(requests) != 2 ||
		requests[0] != "" ||
		requests[1] != "generation.1" {
		t.Fatalf("notification cursors = %#v", requests)
	}
}

func TestNotificationPollerIgnoresStoppedAgents(t *testing.T) {
	service := notificationAgentServiceStub{views: []agent.View{
		{
			Instance: domain.Instance{
				ID:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Name: "Stopped",
				Interfaces: map[string]domain.Interface{
					"notifications": {
						Kind: "notifications",
						Port: 6902,
						Path: "/notifications",
					},
				},
			},
			State: launchruntime.StatusStopped,
		},
	}}
	poller := newNotificationPoller(
		t.TempDir(),
		service,
		func(notificationSource, bridgeNotification) error { return nil },
	)
	if err := poller.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(poller.sources) != 0 {
		t.Fatalf("sources = %#v", poller.sources)
	}
}

// A bridge outlives the launcher, so a restarted launcher that asks without a
// cursor is handed the whole retained queue again. Cursors kept on disk are
// what stops the user seeing every one of those notifications twice.
func TestNotificationPollerResumesCursorAfterRestart(t *testing.T) {
	bridge := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		after := uint64(0)
		if cursor := request.URL.Query().Get("cursor"); cursor != "" {
			_, sequenceText, _ := strings.Cut(cursor, ".")
			after, _ = strconv.ParseUint(sequenceText, 10, 64)
		}
		page := bridgeNotificationPage{NextCursor: "generation.2"}
		for sequence := after + 1; sequence <= 2; sequence++ {
			page.Notifications = append(page.Notifications, bridgeNotification{
				ID:    uint32(sequence),
				Title: "Task complete",
			})
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(page)
	}))
	defer bridge.Close()

	service := notificationAgentServiceStub{views: []agent.View{
		{
			Instance: domain.Instance{
				ID:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Name: "Research",
				Interfaces: map[string]domain.Interface{
					"notifications": {
						Kind: "notifications",
						Port: serverPort(t, bridge.URL),
						Path: "/notifications",
					},
				},
			},
			State: launchruntime.StatusRunning,
		},
	}}
	root := t.TempDir()
	delivered := make([]bridgeNotification, 0, 2)
	collect := func(_ notificationSource, item bridgeNotification) error {
		delivered = append(delivered, item)
		return nil
	}

	first := newNotificationPoller(root, service, collect)
	if err := first.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	first.poll(context.Background())
	if len(delivered) != 2 {
		t.Fatalf("first run delivered = %#v", delivered)
	}

	second := newNotificationPoller(root, service, collect)
	if err := second.refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	second.poll(context.Background())
	if len(delivered) != 2 {
		t.Fatalf("restart redelivered = %#v", delivered)
	}
}

// An agent republished on another address is a different bridge, whose
// sequences have nothing to do with the cursor held for the old one.
func TestNotificationCursorStoreIgnoresCursorFromAnotherAddress(t *testing.T) {
	root := t.TempDir()
	saved := newNotificationCursorStore(root)
	saved.save(notificationSource{
		agentID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		url:     "http://127.0.0.1:6902/notifications",
		cursor:  "generation.4",
	})

	restored := newNotificationCursorStore(root)
	if cursor := restored.lookup(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"http://127.0.0.1:6902/notifications",
	); cursor != "generation.4" {
		t.Fatalf("restored cursor = %q", cursor)
	}
	if cursor := restored.lookup(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"http://127.0.0.1:7100/notifications",
	); cursor != "" {
		t.Fatalf("cursor from another address = %q", cursor)
	}

	restored.retain(map[string]struct{}{})
	if cursor := newNotificationCursorStore(root).lookup(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"http://127.0.0.1:6902/notifications",
	); cursor != "" {
		t.Fatalf("cursor for a deleted agent = %q", cursor)
	}
}

func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	_, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return port
}
