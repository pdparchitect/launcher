package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pdparchitect/launcher/internal/httpapi"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

type Options struct {
	Stdout   io.Writer
	OpenPath func(string) error
}

func Run(
	ctx context.Context,
	service httpapi.Service,
	options Options,
) error {
	return run(ctx, service, options)
}

// RunViewer resolves an agent through the container runtime before opening it.
// Used by "launcher viewer NAME" from a terminal, where nothing is resolved yet.
func RunViewer(
	ctx context.Context,
	service httpapi.Service,
	reference string,
) error {
	view, err := service.Get(ctx, reference)
	if err != nil {
		return err
	}
	if view.State != launchruntime.StatusRunning {
		return fmt.Errorf("start %s before opening it in a window", view.Name)
	}
	viewer := "web"
	for _, entry := range service.Catalog() {
		if entry.ID == view.CatalogID {
			viewer = entry.Viewer
			break
		}
	}
	return runViewer(ctx, view.Name, view.URL(), viewer)
}

// RunViewerTarget opens an already-resolved agent. The launcher process has
// just inspected the container to serve the request, so repeating that here
// would add seconds of container-runtime latency before the window appears.
func RunViewerTarget(ctx context.Context, target httpapi.ViewerTarget) error {
	if target.URL == "" {
		return errors.New("agent window needs a resolved agent URL")
	}
	viewer := target.Viewer
	if viewer == "" {
		viewer = "web"
	}
	return runViewer(ctx, target.Name, target.URL, viewer)
}

// SpawnViewer starts the viewer as a separate process, passing the resolved
// target so the child skips catalogue and container-runtime lookups entirely.
func SpawnViewer(target httpapi.ViewerTarget) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate Launcher executable: %w", err)
	}
	command := exec.Command(
		executable,
		"viewer",
		"--url", target.URL,
		"--name", target.Name,
		"--viewer", target.Viewer,
	)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start agent window: %w", err)
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
