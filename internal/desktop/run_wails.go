//go:build desktop

package desktop

import (
	"context"
	"fmt"
	"runtime"

	"github.com/pdparchitect/launcher/internal/desktop/nativehost"
	"github.com/pdparchitect/launcher/internal/httpapi"
	"github.com/pdparchitect/launcher/internal/webapp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	windowWidth          = 1440
	windowHeight         = 900
	windowMinWidth       = 960
	windowMinHeight      = 640
	macOSWindowWidth     = 1180
	macOSWindowHeight    = 760
	macOSWindowMinWidth  = 820
	macOSWindowMinHeight = 560
)

func Available() bool {
	return true
}

type windowChrome struct {
	frameless         bool
	hideWindowOnClose bool
	mac               *mac.Options
}

type windowGeometry struct {
	width     int
	height    int
	minWidth  int
	minHeight int
}

func mainWindowGeometry() windowGeometry {
	if runtime.GOOS == "darwin" {
		return windowGeometry{
			width:     macOSWindowWidth,
			height:    macOSWindowHeight,
			minWidth:  macOSWindowMinWidth,
			minHeight: macOSWindowMinHeight,
		}
	}
	return windowGeometry{
		width:     windowWidth,
		height:    windowHeight,
		minWidth:  windowMinWidth,
		minHeight: windowMinHeight,
	}
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
			// SwiftUI owns the unified toolbar after its hosting controller is
			// installed. Starting with Wails' toolbar-free full-size title bar
			// avoids competing with NavigationSplitView's toolbar integration.
			TitleBar: mac.TitleBarHidden(),
		},
	}
}

// viewerWindowChrome gives macOS the real title bar rather than the frameless
// HTML controls. The viewer navigates to the agent's own interface, which is
// cross-origin, so there is no page of ours left to draw window chrome into.
// The title bar is then hidden until the pointer nears the top edge, which is
// what InstallViewerChrome sets up. Other platforms keep their normal frame.
func viewerWindowChrome() *mac.Options {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return &mac.Options{
		TitleBar: mac.TitleBarHidden(),
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
	geometry := mainWindowGeometry()
	window := mainWindowChrome()
	err = wails.Run(&options.App{
		Title:                    "Agent Launcher",
		Width:                    geometry.width,
		Height:                   geometry.height,
		MinWidth:                 geometry.minWidth,
		MinHeight:                geometry.minHeight,
		Frameless:                window.frameless,
		Mac:                      window.mac,
		StartHidden:              runtime.GOOS == "darwin",
		HideWindowOnClose:        window.hideWindowOnClose,
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
			nativehost.Install()
			go func() {
				select {
				case <-ctx.Done():
					wailsruntime.Quit(wailsContext)
				case <-finished:
				}
			}()
		},
		OnDomReady: func(context.Context) {
			// OnStartup normally installs before navigation begins. Retrying
			// after the DOM loads closes the small race with Wails creating and
			// exposing its native window.
			nativehost.Install()
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
		Title:                    fmt.Sprintf("%s - Agent Launcher", name),
		Width:                    1280,
		Height:                   800,
		MinWidth:                 720,
		MinHeight:                480,
		Frameless:                false,
		Mac:                      viewerWindowChrome(),
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
			nativehost.BadgeDockIcon()
			nativehost.InstallViewerChrome()
			go func() {
				select {
				case <-ctx.Done():
					wailsruntime.Quit(wailsContext)
				case <-finished:
				}
			}()
		},
		OnDomReady: func(context.Context) {
			// Same race as the launcher's shell: OnStartup can run before
			// Wails has created and exposed the native window.
			nativehost.InstallViewerChrome()
		},
	})
	if err != nil {
		return fmt.Errorf("run agent viewer window: %w", err)
	}
	return nil
}
