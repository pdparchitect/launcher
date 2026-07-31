//go:build desktop

package desktop

import (
	"context"
	"fmt"
	"io"

	"github.com/pdparchitect/launcher/internal/httpapi"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func runNativeNotifications(
	ctx context.Context,
	wailsContext context.Context,
	service httpapi.Service,
	output io.Writer,
) {
	if err := wailsruntime.InitializeNotifications(wailsContext); err != nil {
		notificationError(output, "initialize native notifications", err)
		return
	}
	defer wailsruntime.CleanupNotifications(wailsContext)

	authorized, err := wailsruntime.CheckNotificationAuthorization(wailsContext)
	if err != nil {
		notificationError(output, "check notification authorization", err)
		return
	}
	if !authorized {
		authorized, err = wailsruntime.RequestNotificationAuthorization(wailsContext)
		if err != nil {
			notificationError(output, "request notification authorization", err)
			return
		}
	}
	if !authorized {
		return
	}

	poller := newNotificationPoller(
		service,
		func(
			source notificationSource,
			item bridgeNotification,
		) error {
			subtitle := source.agentName
			if item.App != "" && item.App != source.agentName {
				subtitle += " · " + item.App
			}
			return wailsruntime.SendNotification(
				wailsContext,
				wailsruntime.NotificationOptions{
					ID: fmt.Sprintf(
						"%s:%d",
						source.agentID,
						item.ID,
					),
					Title:    item.Title,
					Subtitle: subtitle,
					Body:     item.Body,
				},
			)
		},
	)
	poller.run(ctx)
}

func notificationError(output io.Writer, operation string, err error) {
	if output == nil {
		return
	}
	_, _ = fmt.Fprintf(
		output,
		"[launcher] %s: %v\n",
		operation,
		err,
	)
}
