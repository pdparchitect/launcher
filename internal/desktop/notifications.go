package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pdparchitect/launcher/internal/agent"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
)

const (
	notificationDiscoveryInterval = 10 * time.Second
	notificationPollInterval      = 2 * time.Second
	maxNotificationPageBytes      = 1 << 20
)

type notificationAgentService interface {
	ListWithIssues(context.Context) ([]agent.View, []store.Issue, error)
}

type bridgeNotification struct {
	ID        uint32    `json:"id"`
	App       string    `json:"app"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Urgency   string    `json:"urgency"`
	CreatedAt time.Time `json:"createdAt"`
}

type bridgeNotificationPage struct {
	Notifications []bridgeNotification `json:"notifications"`
	NextCursor    string               `json:"nextCursor"`
}

type notificationSource struct {
	agentID   string
	agentName string
	url       string
	cursor    string
}

type notificationPoller struct {
	service notificationAgentService
	client  *http.Client
	deliver func(notificationSource, bridgeNotification) error
	cursors *notificationCursorStore
	sources map[string]notificationSource
}

// newNotificationPoller keeps its cursors under root so a launcher restart
// resumes where the previous run stopped. Without that, every bridge is asked
// for its whole retained queue again and the desktop repeats notifications the
// user has already seen.
func newNotificationPoller(
	root string,
	service notificationAgentService,
	deliver func(notificationSource, bridgeNotification) error,
) *notificationPoller {
	return &notificationPoller{
		service: service,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		deliver: deliver,
		cursors: newNotificationCursorStore(root),
		sources: make(map[string]notificationSource),
	}
}

func (poller *notificationPoller) run(ctx context.Context) {
	_ = poller.refresh(ctx)
	poller.poll(ctx)

	discoveryTicker := time.NewTicker(notificationDiscoveryInterval)
	pollTicker := time.NewTicker(notificationPollInterval)
	defer discoveryTicker.Stop()
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-discoveryTicker.C:
			_ = poller.refresh(ctx)
		case <-pollTicker.C:
			poller.poll(ctx)
		}
	}
}

func (poller *notificationPoller) refresh(ctx context.Context) error {
	views, _, err := poller.service.ListWithIssues(ctx)
	if err != nil {
		return err
	}
	updated := make(map[string]notificationSource)
	known := make(map[string]struct{}, len(views))
	for _, view := range views {
		known[view.ID] = struct{}{}
		if view.State != launchruntime.StatusRunning {
			continue
		}
		for _, resolved := range view.Interfaces {
			if resolved.Kind != "notifications" {
				continue
			}
			source := notificationSource{
				agentID:   view.ID,
				agentName: view.Name,
				url:       resolved.URL(),
			}
			// The cursor is only meaningful for the bridge that issued it, so
			// an agent republished on a different address starts over. A bridge
			// that restarted behind the same address rejects the stale cursor
			// itself, because its generation no longer matches.
			source.cursor = poller.cursors.lookup(view.ID, source.url)
			updated[view.ID] = source
			break
		}
	}
	poller.sources = updated
	poller.cursors.retain(known)
	return nil
}

func (poller *notificationPoller) poll(ctx context.Context) {
	for id, source := range poller.sources {
		page, err := poller.fetch(ctx, source)
		if err != nil {
			continue
		}
		for _, item := range page.Notifications {
			if item.ID == 0 || strings.TrimSpace(item.Title) == "" {
				continue
			}
			// Delivery failures must not turn one notification into a retry
			// storm every two seconds. The bridge has accepted the event and
			// advancing the cursor keeps polling bounded and quiet.
			_ = poller.deliver(source, item)
		}
		source.cursor = page.NextCursor
		poller.sources[id] = source
		poller.cursors.save(source)
	}
}

// notificationCursorStore remembers how far each agent's notification bridge
// has been read. Cursors survive a launcher restart, which is what keeps
// already-delivered notifications from being announced a second time.
type notificationCursorStore struct {
	path    string
	mutex   sync.Mutex
	cursors map[string]notificationCursorRecord
}

type notificationCursorRecord struct {
	URL    string `json:"url"`
	Cursor string `json:"cursor"`
}

// An empty root keeps the cursors in memory only, which is all a build without
// a data folder can offer.
func newNotificationCursorStore(root string) *notificationCursorStore {
	cursorStore := &notificationCursorStore{
		cursors: make(map[string]notificationCursorRecord),
	}
	if root == "" {
		return cursorStore
	}
	cursorStore.path = filepath.Join(root, "notifications", "cursors.json")
	cursorStore.restore()
	return cursorStore
}

func (cursorStore *notificationCursorStore) restore() {
	data, err := os.ReadFile(cursorStore.path)
	if err != nil {
		return
	}
	var records map[string]notificationCursorRecord
	if json.Unmarshal(data, &records) != nil {
		return
	}
	for id, record := range records {
		if record.URL == "" || record.Cursor == "" {
			continue
		}
		cursorStore.cursors[id] = record
	}
}

func (cursorStore *notificationCursorStore) lookup(
	agentID string,
	target string,
) string {
	cursorStore.mutex.Lock()
	defer cursorStore.mutex.Unlock()

	record, exists := cursorStore.cursors[agentID]
	if !exists || record.URL != target {
		return ""
	}
	return record.Cursor
}

func (cursorStore *notificationCursorStore) save(source notificationSource) {
	cursorStore.mutex.Lock()
	defer cursorStore.mutex.Unlock()

	record := notificationCursorRecord{
		URL:    source.url,
		Cursor: source.cursor,
	}
	if cursorStore.cursors[source.agentID] == record {
		return
	}
	cursorStore.cursors[source.agentID] = record
	cursorStore.persist()
}

// retain drops cursors for agents that no longer exist so a long-lived
// installation does not accumulate them forever.
func (cursorStore *notificationCursorStore) retain(
	agentIDs map[string]struct{},
) {
	cursorStore.mutex.Lock()
	defer cursorStore.mutex.Unlock()

	changed := false
	for id := range cursorStore.cursors {
		if _, keep := agentIDs[id]; keep {
			continue
		}
		delete(cursorStore.cursors, id)
		changed = true
	}
	if changed {
		cursorStore.persist()
	}
}

// persist runs under the store mutex. A cursor that fails to reach disk only
// costs a repeat of the current backlog after the next restart, so the write
// stays quiet rather than interrupting delivery.
func (cursorStore *notificationCursorStore) persist() {
	if cursorStore.path == "" {
		return
	}
	data, err := json.MarshalIndent(cursorStore.cursors, "", "  ")
	if err != nil {
		return
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(cursorStore.path), 0o700); err != nil {
		return
	}
	temporary := cursorStore.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return
	}
	if err := os.Rename(temporary, cursorStore.path); err != nil {
		_ = os.Remove(temporary)
	}
}

func (poller *notificationPoller) fetch(
	ctx context.Context,
	source notificationSource,
) (bridgeNotificationPage, error) {
	target, err := url.Parse(source.url)
	if err != nil {
		return bridgeNotificationPage{}, err
	}
	query := target.Query()
	if source.cursor != "" {
		query.Set("cursor", source.cursor)
	}
	target.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		target.String(),
		nil,
	)
	if err != nil {
		return bridgeNotificationPage{}, err
	}
	response, err := poller.client.Do(request)
	if err != nil {
		return bridgeNotificationPage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return bridgeNotificationPage{}, fmt.Errorf(
			"notification bridge returned %s",
			response.Status,
		)
	}
	body, err := io.ReadAll(io.LimitReader(
		response.Body,
		maxNotificationPageBytes+1,
	))
	if err != nil {
		return bridgeNotificationPage{}, err
	}
	if len(body) > maxNotificationPageBytes {
		return bridgeNotificationPage{}, errors.New(
			"notification bridge response is too large",
		)
	}
	var page bridgeNotificationPage
	if err := json.Unmarshal(body, &page); err != nil {
		return bridgeNotificationPage{}, err
	}
	if page.NextCursor == "" {
		return bridgeNotificationPage{}, errors.New(
			"notification bridge response has no cursor",
		)
	}
	return page, nil
}
