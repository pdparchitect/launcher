package desktop

import (
	"context"
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
	return runViewer(ctx, view)
}

func SpawnViewer(reference string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate Launcher executable: %w", err)
	}
	command := exec.Command(executable, "viewer", reference)
	if err := command.Start(); err != nil {
		return fmt.Errorf("start agent window: %w", err)
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
