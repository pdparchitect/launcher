package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"

	"github.com/pdparchitect/launcher/cli"
	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/config"
	"github.com/pdparchitect/launcher/internal/desktop"
	"github.com/pdparchitect/launcher/internal/httpapi"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
	"github.com/pdparchitect/launcher/internal/webapp"
)

var (
	version          = "dev"
	swiftArchiveHash = "none"
)

func main() {
	root, err := config.DataRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: locate data folder: %v\n", err)
		os.Exit(1)
	}
	manifests, err := catalog.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load catalogue: %v\n", err)
		os.Exit(1)
	}
	selection, err := launchruntime.Detect(launchruntime.DetectOptions{
		GOOS:      goruntime.GOOS,
		GOARCH:    goruntime.GOARCH,
		Requested: os.Getenv("PDPARCHITECT_LAUNCHER_RUNTIME"),
		Runner:    launchruntime.OSRunner{},
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: select container runtime: %v\n", err)
		os.Exit(1)
	}
	service := agent.New(
		store.New(root),
		selection.Runtime,
		manifests,
		agent.Options{
			RuntimeName: string(selection.Name),
			RuntimePath: selection.Path,
		},
	)
	systemOpener := cli.SystemOpener{}
	appOptions := []cli.Option{
		cli.WithInput(os.Stdin),
		cli.WithServer(func(
			ctx context.Context,
			options cli.ServeOptions,
		) error {
			return webapp.Run(ctx, service, systemOpener, webapp.Options{
				Listen: options.Listen,
				Open:   options.Open,
				Stdout: os.Stdout,
			})
		}),
	}
	if desktop.Available() {
		appOptions = append(
			appOptions,
			cli.WithDesktop(func(ctx context.Context) error {
				return desktop.Run(ctx, service, desktop.Options{
					Stdout:   os.Stdout,
					OpenPath: systemOpener.OpenPath,
				})
			}),
			cli.WithViewer(func(ctx context.Context, reference string) error {
				return desktop.RunViewer(ctx, service, reference)
			}),
			cli.WithViewerTarget(func(
				ctx context.Context,
				name string,
				url string,
				viewer string,
			) error {
				return desktop.RunViewerTarget(ctx, httpapi.ViewerTarget{
					Name:   name,
					URL:    url,
					Viewer: viewer,
				})
			}),
		)
	}
	app := cli.New(
		service,
		systemOpener,
		os.Stdout,
		os.Stderr,
		version,
		appOptions...,
	)
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	code := app.Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}
