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
	"github.com/wailsapp/wails/v2/pkg/menu"
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

// focusViewer brings an already-open agent window to the front. Each viewer is
// its own process, so this activates that application rather than raising a
// window this one owns.
func focusViewer(pid int) bool {
	return nativehost.ActivateProcess(pid)
}

type windowChrome struct {
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

// mainWindowChrome leaves window controls to the platform on every operating
// system. Linux and Windows use Wails' decorated native window, while macOS
// adds a full-size SwiftUI content view beneath its native traffic lights.
//
// Closing the window hides it on macOS rather than quitting, because agent
// viewers run as separate processes: quitting the main process would strand
// them with no way to bring the launcher back. Wails already answers NO to
// applicationShouldTerminateAfterLastWindowClosed, so hiding leaves the app
// running and a Dock click restores it. Cmd+Q still quits.
func mainWindowChrome() windowChrome {
	if runtime.GOOS != "darwin" {
		return windowChrome{}
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

// viewerWindowChrome gives macOS a transparent, full-size native title bar.
// InstallViewerChrome keeps the WKWebView below that bar while it is revealed,
// then lets the view fill the frame after the chrome disappears. The viewer
// navigates cross-origin, so this separation has to happen in AppKit rather
// than in the agent's page. Other platforms keep their normal frame.
func viewerWindowChrome() *mac.Options {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return &mac.Options{
		TitleBar: &mac.TitleBar{
			TitlebarAppearsTransparent: true,
			HideTitle:                  true,
			HideTitleBar:               false,
			FullSizeContent:            true,
			UseToolbar:                 false,
			HideToolbarSeparator:       true,
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
	notificationContext, stopNotifications := context.WithCancel(ctx)
	defer stopNotifications()

	serverOptions := []httpapi.Option{
		httpapi.WithLogger(runOptions.Stdout),
		httpapi.WithViewerOpener(SpawnViewer),
	}
	if runOptions.CatalogAssets != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithCatalogAssets(runOptions.CatalogAssets),
		)
	}
	if runOptions.UpdateStatus != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithUpdateStatus(runOptions.UpdateStatus),
		)
	}
	if runOptions.UpdateRefresh != nil {
		serverOptions = append(
			serverOptions,
			httpapi.WithUpdateRefresh(runOptions.UpdateRefresh),
		)
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
		Frameless:                false,
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
			go runNativeNotifications(
				notificationContext,
				wailsContext,
				runOptions.DataRoot,
				service,
				runOptions.Stdout,
			)
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

func runViewer(
	ctx context.Context,
	service ViewerService,
	reference string,
	name string,
	target string,
	kind string,
	viewerOptions ViewerOptions,
) error {
	actions := newViewerActions(ctx, service, reference, name, viewerOptions)
	finished := make(chan struct{})
	defer close(finished)
	var applicationMenu *menu.Menu
	if runtime.GOOS == "darwin" && reference != "" {
		applicationMenu = viewerApplicationMenu(actions)
	}

	err := wails.Run(&options.App{
		Title:                    fmt.Sprintf("%s - Agent Launcher", name),
		Width:                    1280,
		Height:                   800,
		MinWidth:                 720,
		MinHeight:                480,
		Frameless:                false,
		Mac:                      viewerWindowChrome(),
		Menu:                     applicationMenu,
		DisableResize:            false,
		EnableDefaultContextMenu: false,
		BackgroundColour: options.NewRGB(
			5,
			6,
			4,
		),
		AssetServer: &assetserver.Options{
			Handler: viewerHandler(name, target, kind),
		},
		OnStartup: func(wailsContext context.Context) {
			actions.setRenameHost(
				nativehost.PromptText,
				func(name string) {
					wailsruntime.WindowSetTitle(
						wailsContext,
						fmt.Sprintf("%s - Agent Launcher", name),
					)
				},
			)
			actions.setHost(
				func() { wailsruntime.Quit(wailsContext) },
				func(title string, actionErr error) {
					_, dialogErr := wailsruntime.MessageDialog(
						wailsContext,
						wailsruntime.MessageDialogOptions{
							Type:  wailsruntime.ErrorDialog,
							Title: title, Message: actionErr.Error(),
						},
					)
					if dialogErr != nil && viewerOptions.Stdout != nil {
						fmt.Fprintf(
							viewerOptions.Stdout,
							"%s: %v (show dialog: %v)\n",
							title,
							actionErr,
							dialogErr,
						)
					}
				},
				func() {
					wailsruntime.MenuUpdateApplicationMenu(wailsContext)
				},
			)
			nativehost.BadgeDockIcon()
			// Before Wails navigates: the viewer page replaces itself with the
			// agent's interface immediately, and the fix has to be installed
			// ahead of that load.
			nativehost.InstallArrowKeyFix()
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
			nativehost.InstallArrowKeyFix()
			nativehost.InstallViewerChrome()
		},
	})
	if err != nil {
		return fmt.Errorf("run agent viewer window: %w", err)
	}
	return nil
}
