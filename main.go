package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/pdparchitect/launcher/cli"
	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/agentskill"
	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/config"
	"github.com/pdparchitect/launcher/internal/desktop"
	"github.com/pdparchitect/launcher/internal/httpapi"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
	"github.com/pdparchitect/launcher/internal/store"
	"github.com/pdparchitect/launcher/internal/updatecheck"
	"github.com/pdparchitect/launcher/internal/webapp"
)

var (
	version          = "dev"
	swiftArchiveHash = "none"
)

func main() {
	installAgentSkills()
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
	if len(catalogue.List()) == 0 {
		registryContext, cancelRegistry := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		_, registryErr := catalogue.Refresh(registryContext, false)
		cancelRegistry()
		if registryErr != nil {
			fmt.Fprintf(
				os.Stderr,
				"warning: application registry is unavailable: %v\n",
				registryErr,
			)
		}
	}
	updates := updatecheck.NewManager(root, version, updatecheck.Options{})
	refreshLauncherUpdate := func(
		ctx context.Context,
	) (updatecheck.Status, error) {
		_, refreshErr := updates.Refresh(ctx, true)
		return updates.Status(), refreshErr
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
		cli.WithTerminalAttached(
			isatty.IsTerminal(os.Stdin.Fd()) ||
				isatty.IsTerminal(os.Stdout.Fd()) ||
				isatty.IsTerminal(os.Stderr.Fd()),
		),
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
				UpdateStatus:  updates.Status,
				UpdateRefresh: refreshLauncherUpdate,
			})
		}),
	}
	if desktop.Available() {
		appOptions = append(
			appOptions,
			cli.WithDesktop(func(ctx context.Context) error {
				return desktop.Run(ctx, service, desktop.Options{
					DataRoot:      root,
					Stdout:        os.Stdout,
					OpenPath:      systemOpener.OpenPath,
					CatalogAssets: catalogue,
					UpdateStatus:  updates.Status,
					UpdateRefresh: refreshLauncherUpdate,
				})
			}),
			cli.WithViewer(func(ctx context.Context, reference string) error {
				return desktop.RunViewer(
					ctx,
					service,
					reference,
					desktop.ViewerOptions{
						Stdout: os.Stdout, OpenPath: systemOpener.OpenPath,
					},
				)
			}),
			cli.WithViewerTarget(func(
				ctx context.Context,
				id string,
				name string,
				url string,
				kind string,
			) error {
				return desktop.RunViewerTarget(
					ctx,
					service,
					httpapi.ViewerTarget{
						ID: id, Name: name, URL: url, Kind: kind,
					},
					desktop.ViewerOptions{
						Stdout: os.Stdout, OpenPath: systemOpener.OpenPath,
					},
				)
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
		func(ctx context.Context, force bool) {
			refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			_, _ = refreshCatalogue(refreshCtx, force)
		},
	)
	go runRefreshLoop(
		ctx,
		updatecheck.DefaultRefreshInterval,
		func(ctx context.Context) {
			refreshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			_, _ = updates.Refresh(refreshCtx, false)
		},
	)
	go runRefreshLoop(
		ctx,
		agent.DefaultImageCleanupInterval,
		func(ctx context.Context) {
			cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			_, _ = service.CleanupImages(
				cleanupCtx,
				agent.DefaultImageRetention,
			)
		},
	)
	code := app.Run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func installAgentSkills() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: locate home for agent integration: %v\n", err)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: locate Launcher for agent integration: %v\n", err)
		return
	}
	if err := agentskill.Install(home, executable, cli.AgentGuide()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: install agent skills: %v\n", err)
	}
}

func runRefreshLoop(
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

func runCatalogueRefreshLoop(
	ctx context.Context,
	interval time.Duration,
	refresh func(context.Context, bool),
) {
	force := true
	runRefreshLoop(ctx, interval, func(ctx context.Context) {
		refresh(ctx, force)
		force = false
	})
}
