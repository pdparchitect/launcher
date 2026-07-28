//go:build desktop

package desktop

import (
	"context"
	"fmt"
	"runtime"

	"github.com/pdparchitect/launcher/internal/httpapi"
	"github.com/pdparchitect/launcher/internal/webapp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
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

type windowChrome struct {
	frameless         bool
	hideWindowOnClose bool
	mac               *mac.Options
}

// mainWindowChrome keeps macOS on the real window chrome: native traffic
// lights, rounded corners and a full-size content view the webview draws
// underneath. Every other platform stays frameless with the HTML controls.
//
// Closing the window hides it on macOS rather than quitting, because agent
// viewers run as separate processes: quitting the main process would strand
// them with no way to bring the launcher back. Wails already answers NO to
// applicationShouldTerminateAfterLastWindowClosed, so hiding leaves the app
// running and a Dock click restores it. Cmd+Q still quits.
func mainWindowChrome() windowChrome {
	if runtime.GOOS != "darwin" {
		return windowChrome{frameless: true}
	}
	return windowChrome{
		hideWindowOnClose: true,
		mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	}
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

	serverOptions := []httpapi.Option{
		httpapi.WithLogger(runOptions.Stdout),
		httpapi.WithViewerOpener(SpawnViewer),
	}
	if runOptions.OpenPath != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithPathOpener(runOptions.OpenPath),
		)
	}
	handler := httpapi.New(service, token, serverOptions...)
	chrome := mainWindowChrome()
	err = wails.Run(&options.App{
		Title:                    "Agent Launcher",
		Width:                    windowWidth,
		Height:                   windowHeight,
		MinWidth:                 windowMinWidth,
		MinHeight:                windowMinHeight,
		Frameless:                chrome.frameless,
		Mac:                      chrome.mac,
		HideWindowOnClose:        chrome.hideWindowOnClose,
		DisableResize:            false,
		EnableDefaultContextMenu: false,
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

func runViewer(ctx context.Context, name string, target string, viewer string) error {
	finished := make(chan struct{})
	defer close(finished)

	err := wails.Run(&options.App{
		Title:                    fmt.Sprintf("%s — Agent Launcher", name),
		Width:                    1280,
		Height:                   800,
		MinWidth:                 720,
		MinHeight:                480,
		Frameless:                false,
		DisableResize:            false,
		EnableDefaultContextMenu: false,
		BackgroundColour: options.NewRGB(
			5,
			6,
			4,
		),
		AssetServer: &assetserver.Options{
			Handler: viewerHandler(name, target, viewer),
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
		return fmt.Errorf("run agent viewer window: %w", err)
	}
	return nil
}
