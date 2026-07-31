package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	sources map[string]notificationSource
}

func newNotificationPoller(
	service notificationAgentService,
	deliver func(notificationSource, bridgeNotification) error,
) *notificationPoller {
	return &notificationPoller{
		service: service,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		deliver: deliver,
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
	for _, view := range views {
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
			if previous, exists := poller.sources[view.ID]; exists && previous.url == source.url {
				source.cursor = previous.cursor
			}
			updated[view.ID] = source
			break
		}
	}
	poller.sources = updated
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
