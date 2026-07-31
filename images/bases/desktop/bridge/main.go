package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	notificationBusName   = "org.freedesktop.Notifications"
	notificationInterface = "org.freedesktop.Notifications"
	notificationPath      = dbus.ObjectPath("/org/freedesktop/Notifications")
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("[desktop-bridge] %v", err)
	}
}

func run() error {
	queue, err := newNotificationQueue(250)
	if err != nil {
		return fmt.Errorf("create notification queue: %w", err)
	}

	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect to desktop session bus: %w", err)
	}
	defer connection.Close()

	notifications := newNotificationServer(connection, queue)
	if err := connection.Export(
		notifications,
		notificationPath,
		notificationInterface,
	); err != nil {
		return fmt.Errorf("export notification service: %w", err)
	}
	introspection := &introspect.Node{
		Name: string(notificationPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{
				Name:    notificationInterface,
				Methods: introspect.Methods(notifications),
				Signals: []introspect.Signal{
					{
						Name: "NotificationClosed",
						Args: []introspect.Arg{
							{Name: "id", Type: "u"},
							{Name: "reason", Type: "u"},
						},
					},
				},
			},
		},
	}
	if err := connection.Export(
		introspect.NewIntrospectable(introspection),
		notificationPath,
		introspect.IntrospectData.Name,
	); err != nil {
		return fmt.Errorf("export notification introspection: %w", err)
	}
	reply, err := connection.RequestName(
		notificationBusName,
		dbus.NameFlagDoNotQueue,
	)
	if err != nil {
		return fmt.Errorf("register notification service: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return errors.New("another desktop notification service is already running")
	}

	handler := newBridgeHandler(newPreviewCache(), queue)
	server := &http.Server{
		Addr:              ":6902",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		log.Printf("[desktop-bridge] serving preview, health, and notifications on port 6902")
		errorsChannel <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()
		return server.Shutdown(shutdownContext)
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve desktop bridge: %w", err)
	}
}

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
