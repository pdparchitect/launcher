package desktop

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

type viewerActions struct {
	ctx       context.Context
	service   ViewerService
	reference string
	name      string
	options   ViewerOptions

	hostMutex sync.RWMutex
	close     func()
	report    func(string, error)
	update    func()
	prompt    func(string, string, string) (string, bool)
	renamed   func(string)
}

func newViewerActions(
	ctx context.Context,
	service ViewerService,
	reference string,
	name string,
	options ViewerOptions,
) *viewerActions {
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	return &viewerActions{
		ctx: ctx, service: service, reference: reference, name: name,
		options: options,
	}
}

func (actions *viewerActions) setRenameHost(
	prompt func(string, string, string) (string, bool),
	renamed func(string),
) {
	actions.hostMutex.Lock()
	defer actions.hostMutex.Unlock()
	actions.prompt = prompt
	actions.renamed = renamed
}

func (actions *viewerActions) setHost(
	closeWindow func(),
	reportError func(string, error),
	updateMenu func(),
) {
	actions.hostMutex.Lock()
	defer actions.hostMutex.Unlock()
	actions.close = closeWindow
	actions.report = reportError
	actions.update = updateMenu
}

func (actions *viewerActions) openFiles() error {
	path, err := actions.service.AgentFiles(actions.ctx, actions.reference)
	if err != nil {
		return fmt.Errorf("locate agent files: %w", err)
	}
	if actions.options.OpenPath == nil {
		return fmt.Errorf("opening local files is unavailable in this Launcher build")
	}
	if err := actions.options.OpenPath(path); err != nil {
		return fmt.Errorf("open agent files: %w", err)
	}
	return nil
}

func (actions *viewerActions) stop() error {
	if _, err := actions.service.Stop(actions.ctx, actions.reference); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	return nil
}

func (actions *viewerActions) rename() error {
	actions.hostMutex.RLock()
	prompt := actions.prompt
	currentName := actions.name
	actions.hostMutex.RUnlock()
	if prompt == nil {
		return fmt.Errorf("renaming agents is unavailable in this Launcher build")
	}

	name, accepted := prompt(
		"Rename Agent",
		fmt.Sprintf("Enter a new name for %s:", currentName),
		currentName,
	)
	if !accepted {
		return nil
	}
	instance, err := actions.service.Rename(
		actions.ctx,
		actions.reference,
		name,
	)
	if err != nil {
		return fmt.Errorf("rename agent: %w", err)
	}

	actions.hostMutex.Lock()
	actions.name = instance.Name
	renamed := actions.renamed
	actions.hostMutex.Unlock()
	if renamed != nil {
		renamed(instance.Name)
	}
	return nil
}

func (actions *viewerActions) closeWindow() {
	actions.hostMutex.RLock()
	closeWindow := actions.close
	actions.hostMutex.RUnlock()
	if closeWindow != nil {
		closeWindow()
	}
}

func (actions *viewerActions) perform(
	item *menu.MenuItem,
	title string,
	action func() error,
) bool {
	item.Disable()
	actions.updateMenu()
	if err := action(); err != nil {
		item.Enable()
		actions.updateMenu()
		actions.reportError(title, err)
		return false
	}
	item.Enable()
	actions.updateMenu()
	return true
}

func (actions *viewerActions) reportError(title string, err error) {
	actions.hostMutex.RLock()
	reportError := actions.report
	actions.hostMutex.RUnlock()
	if reportError != nil {
		reportError(title, err)
		return
	}
	fmt.Fprintf(actions.options.Stdout, "%s: %v\n", title, err)
}

func (actions *viewerActions) updateMenu() {
	actions.hostMutex.RLock()
	updateMenu := actions.update
	actions.hostMutex.RUnlock()
	if updateMenu != nil {
		updateMenu()
	}
}

func viewerApplicationMenu(actions *viewerActions) *menu.Menu {
	fileMenu := menu.NewMenu()
	fileMenu.AddText(
		"Open Agent Files in Finder",
		keys.Combo("o", keys.CmdOrCtrlKey, keys.ShiftKey),
		func(data *menu.CallbackData) {
			actions.perform(
				data.MenuItem,
				"Could Not Open Agent Files",
				actions.openFiles,
			)
		},
	)
	fileMenu.AddText(
		"Rename Agent…",
		nil,
		func(data *menu.CallbackData) {
			actions.perform(
				data.MenuItem,
				"Could Not Rename Agent",
				actions.rename,
			)
		},
	)
	fileMenu.AddSeparator()
	fileMenu.AddText(
		"Stop Agent and Close Window",
		nil,
		func(data *menu.CallbackData) {
			if actions.perform(
				data.MenuItem,
				"Could Not Stop Agent",
				actions.stop,
			) {
				actions.closeWindow()
			}
		},
	)
	fileMenu.AddText(
		"Close Window",
		keys.CmdOrCtrl("w"),
		func(*menu.CallbackData) { actions.closeWindow() },
	)

	return menu.NewMenuFromItems(
		menu.AppMenu(),
		menu.SubMenu("File", fileMenu),
		menu.EditMenu(),
		menu.WindowMenu(),
	)
}
