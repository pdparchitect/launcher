package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"
	"time"

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
	catalogue, err := catalog.NewManager(root, catalog.ManagerOptions{})
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
		catalogue.List(),
		agent.Options{
			RuntimeName: string(selection.Name),
			RuntimePath: selection.Path,
		},
	)
	systemOpener := cli.SystemOpener{}
	refreshCatalogue := func(
		ctx context.Context,
		force bool,
	) (bool, error) {
		changed, refreshErr := catalogue.Refresh(ctx, force)
		if refreshErr != nil {
			return false, refreshErr
		}
		if changed {
			service.ReplaceCatalog(catalogue.List())
		}
		return changed, nil
	}
	appOptions := []cli.Option{
		cli.WithInput(os.Stdin),
		cli.WithCatalogRefresh(func(ctx context.Context) (bool, error) {
			return refreshCatalogue(ctx, true)
		}),
		cli.WithServer(func(
			ctx context.Context,
			options cli.ServeOptions,
		) error {
			return webapp.Run(ctx, service, systemOpener, webapp.Options{
				Listen:        options.Listen,
				Open:          options.Open,
				Stdout:        os.Stdout,
				CatalogAssets: catalogue,
			})
		}),
	}
	if desktop.Available() {
		appOptions = append(
			appOptions,
			cli.WithDesktop(func(ctx context.Context) error {
				return desktop.Run(ctx, service, desktop.Options{
					Stdout:        os.Stdout,
					OpenPath:      systemOpener.OpenPath,
					CatalogAssets: catalogue,
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
	go runCatalogueRefreshLoop(
		ctx,
		catalog.DefaultRefreshInterval,
		func(ctx context.Context) {
			refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			_, _ = refreshCatalogue(refreshCtx, false)
		},
	)
	code := app.Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func runCatalogueRefreshLoop(
	ctx context.Context,
	interval time.Duration,
	refresh func(context.Context),
) {
	refresh(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh(ctx)
		}
	}
}
