//go:build desktop

package desktop

import (
	"context"
	"fmt"

	"github.com/pdparchitect/launcher/internal/httpapi"
	"github.com/pdparchitect/launcher/internal/webapp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	windowWidth     = 1440
	windowHeight    = 900
	windowMinWidth  = 960
	windowMinHeight = 640
)

func Available() bool {
	return true
}

func run(
	ctx context.Context,
	service httpapi.Service,
	runOptions Options,
) error {
	token, err := webapp.SessionToken()
	if err != nil {
		return err
	}

	finished := make(chan struct{})
	defer close(finished)

	handler := httpapi.New(
		service,
		token,
		httpapi.WithLogger(runOptions.Stdout),
	)
	err = wails.Run(&options.App{
		Title:         "Agent Launcher",
		Width:         windowWidth,
		Height:        windowHeight,
		MinWidth:      windowMinWidth,
		MinHeight:     windowMinHeight,
		Frameless:     true,
		DisableResize: false,
		BackgroundColour: options.NewRGB(
			5,
			6,
			4,
		),
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		OnStartup: func(wailsContext context.Context) {
			go func() {
				select {
				case <-ctx.Done():
					wailsruntime.Quit(wailsContext)
				case <-finished:
				}
			}()
		},
	})
	if err != nil {
		return fmt.Errorf("run Launcher desktop application: %w", err)
	}
	return nil
}
