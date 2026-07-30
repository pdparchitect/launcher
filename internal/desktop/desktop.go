package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"sync"

	"github.com/pdparchitect/launcher/internal/httpapi"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

type Options struct {
	Stdout        io.Writer
	OpenPath      func(string) error
	CatalogAssets fs.FS
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

// One window per agent. Both windows would be the same view of the same
// container, so a second one is only ever in the way: opening an agent that is
// already open brings its window forward instead.
type viewerWindows struct {
	mutex sync.Mutex
	// Agent name to process identifier. Zero means a viewer that has been
	// claimed but has not started yet.
	open  map[string]int
	focus func(pid int) bool
}

var viewers = &viewerWindows{
	open:  map[string]int{},
	focus: focusViewer,
}

// focusOrClaim reports whether the agent's window has been dealt with. False
// means the caller now owns the claim and must either track a process against
// it or release it.
func (windows *viewerWindows) focusOrClaim(name string) bool {
	windows.mutex.Lock()
	defer windows.mutex.Unlock()

	if pid, opening := windows.open[name]; opening {
		/*
		 A window still on its way is as good as focused - the click that
		 started it is what is being repeated. A window that cannot be brought
		 forward is worth no more than none at all, so the claim is left in
		 place and a fresh process opens over it.
		*/
		return pid == 0 || windows.focus(pid)
	}
	windows.open[name] = 0

	return false
}

func (windows *viewerWindows) track(name string, pid int) {
	windows.mutex.Lock()
	defer windows.mutex.Unlock()
	windows.open[name] = pid
}

func (windows *viewerWindows) release(name string) {
	windows.mutex.Lock()
	defer windows.mutex.Unlock()
	delete(windows.open, name)
}

// SpawnViewer starts the viewer as a separate process, passing the resolved
// target so the child skips catalogue and container-runtime lookups entirely.
// An agent already showing a window is focused rather than opened twice.
func SpawnViewer(target httpapi.ViewerTarget) error {
	if viewers.focusOrClaim(target.Name) {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		viewers.release(target.Name)
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
		viewers.release(target.Name)
		return fmt.Errorf("start agent window: %w", err)
	}
	viewers.track(target.Name, command.Process.Pid)
	go func() {
		_ = command.Wait()
		// Closing the window ends the process, which frees the name for the
		// next time the agent is opened.
		viewers.release(target.Name)
	}()
	return nil
}
