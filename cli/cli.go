package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/domain"
	"github.com/pdparchitect/launcher/internal/store"
)

type Service interface {
	Doctor(context.Context) (agent.DoctorReport, error)
	Catalog() []agent.CatalogEntry
	Create(context.Context, agent.CreateOptions) (domain.Instance, error)
	Duplicate(
		context.Context,
		string,
		agent.DuplicateOptions,
	) (domain.Instance, error)
	List(context.Context) ([]agent.View, error)
	Get(context.Context, string) (agent.View, error)
	Details(context.Context, string) (agent.Details, error)
	Start(context.Context, string) (domain.Instance, error)
	Stop(context.Context, string) (domain.Instance, error)
	Delete(context.Context, string) error
	CleanupImages(context.Context, time.Duration) (agent.ImageCleanupReport, error)
	Logs(context.Context, string, bool) error
	Exec(context.Context, string, agent.ExecOptions) error
}

type App struct {
	service      Service
	opener       Opener
	stdout       io.Writer
	stderr       io.Writer
	version      string
	input        io.Reader
	serve        ServeFunc
	desktop      DesktopFunc
	viewer       ViewerFunc
	viewerTarget ViewerTargetFunc
	refresh      CatalogRefreshFunc
	preview      previewDownloadOptions
	terminal     bool
}

type Option func(*App)

type ServeOptions struct {
	Listen string
	Open   bool
}

type ServeFunc func(context.Context, ServeOptions) error
type DesktopFunc func(context.Context) error
type ViewerFunc func(context.Context, string) error
type ViewerTargetFunc func(context.Context, string, string, string, string) error
type CatalogRefreshFunc func(context.Context) (bool, error)

func WithInput(input io.Reader) Option {
	return func(app *App) { app.input = input }
}

func WithServer(serve ServeFunc) Option {
	return func(app *App) { app.serve = serve }
}

func WithDesktop(desktop DesktopFunc) Option {
	return func(app *App) { app.desktop = desktop }
}

func WithViewer(viewer ViewerFunc) Option {
	return func(app *App) { app.viewer = viewer }
}

func WithViewerTarget(viewer ViewerTargetFunc) Option {
	return func(app *App) { app.viewerTarget = viewer }
}

func WithCatalogRefresh(refresh CatalogRefreshFunc) Option {
	return func(app *App) { app.refresh = refresh }
}

func WithTerminalAttached(attached bool) Option {
	return func(app *App) { app.terminal = attached }
}

func New(
	service Service,
	opener Opener,
	stdout io.Writer,
	stderr io.Writer,
	version string,
	options ...Option,
) *App {
	app := &App{
		service: service, opener: opener, stdout: stdout, stderr: stderr,
		version: version, preview: defaultPreviewDownloadOptions(),
	}
	for _, option := range options {
		option(app)
	}
	return app
}

func (app *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		if app.desktop == nil || app.terminal {
			app.printHelp()
			return 0
		}
		args = []string{"desktop"}
	}
	var err error
	switch args[0] {
	case "help", "-h", "--help":
		app.printHelp()
	case "version", "--version":
		fmt.Fprintf(app.stdout, "launcher %s\n", app.version)
	case "guide":
		err = app.guide(args[1:])
	case "doctor":
		err = app.doctor(ctx, args[1:])
	case "serve":
		err = app.serveUI(ctx, args[1:])
	case "desktop":
		err = app.desktopUI(ctx, args[1:])
	case "viewer":
		err = app.viewerUI(ctx, args[1:])
	case "catalog", "catalogue":
		err = app.catalog(ctx, args[1:])
	case "create":
		err = app.create(ctx, args[1:])
	case "duplicate", "clone":
		err = app.duplicate(ctx, args[1:])
	case "list", "library", "ls":
		err = app.list(ctx, args[1:])
	case "status":
		err = app.status(ctx, args[1:])
	case "start":
		err = app.start(ctx, args[1:])
	case "stop":
		err = app.stop(ctx, args[1:])
	case "open":
		err = app.open(ctx, args[1:])
	case "preview":
		err = app.savePreview(ctx, args[1:])
	case "logs":
		err = app.logs(ctx, args[1:])
	case "exec":
		err = app.exec(ctx, args[1:])
	case "delete", "rm":
		err = app.delete(ctx, args[1:])
	case "cleanup":
		err = app.cleanup(ctx, args[1:])
	default:
		err = fmt.Errorf("unknown command %q; run \"launcher help\"", args[0])
	}
	if err == nil {
		return 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if errors.Is(err, store.ErrNotFound) {
		fmt.Fprintln(app.stderr, "error: agent not found")
	} else {
		fmt.Fprintf(app.stderr, "error: %v\n", err)
	}
	return 1
}

func (app *App) desktopUI(ctx context.Context, args []string) error {
	flags := app.flags("desktop", "Open Launcher as a desktop application.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("desktop does not accept arguments")
	}
	if app.desktop == nil {
		return errors.New("desktop interface is not available in this build")
	}
	return app.desktop(ctx)
}

func (app *App) viewerUI(ctx context.Context, args []string) error {
	flags := app.flags("viewer", "Open an agent in a framed desktop window.")
	url := flags.String("url", "", "open an already-resolved agent URL")
	id := flags.String("id", "", "agent ID, with -url")
	name := flags.String("name", "", "window title, with -url")
	kind := flags.String("kind", "", "interface kind, with -url")
	if err := flags.Parse(args); err != nil {
		return err
	}
	// The launcher passes -url when it spawns a window, having already
	// resolved the agent. Skipping the runtime lookup here is what keeps the
	// window from taking seconds to appear.
	if strings.TrimSpace(*url) != "" {
		if flags.NArg() != 0 {
			return errors.New("usage: launcher viewer -url URL [-name NAME]")
		}
		if app.viewerTarget == nil {
			return errors.New("desktop agent viewer is not available in this build")
		}
		return app.viewerTarget(ctx, *id, *name, *url, *kind)
	}
	if flags.NArg() != 1 || strings.TrimSpace(flags.Arg(0)) == "" {
		return errors.New("usage: launcher viewer NAME")
	}
	if app.viewer == nil {
		return errors.New("desktop agent viewer is not available in this build")
	}
	return app.viewer(ctx, flags.Arg(0))
}

func (app *App) doctor(ctx context.Context, args []string) error {
	flags := app.flags("doctor", "Check the local container runtime.")
	noPrompt := flags.Bool("no-prompt", false, "do not offer runtime setup")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("doctor does not accept arguments")
	}
	report, err := app.service.Doctor(ctx)
	if err != nil {
		if *noPrompt || app.input == nil {
			return err
		}
		var starter runtimeServiceStarter
		if errors.As(err, &starter) {
			confirmed, confirmErr := app.confirm(
				fmt.Sprintf("Start %s service now?", starter.RuntimeName()),
			)
			if confirmErr != nil {
				return confirmErr
			}
			if !confirmed {
				return err
			}
			if startErr := starter.StartService(ctx); startErr != nil {
				return startErr
			}
			report, err = app.service.Doctor(ctx)
			if err != nil {
				return err
			}
		} else {
			var installer runtimeInstaller
			if !errors.As(err, &installer) {
				return err
			}
			confirmed, confirmErr := app.confirm(
				fmt.Sprintf(
					"Open the official %s installation page?",
					installer.RuntimeName(),
				),
			)
			if confirmErr != nil {
				return confirmErr
			}
			if confirmed {
				fmt.Fprintln(app.stdout, installer.InstallURL())
				if openErr := app.opener.Open(installer.InstallURL()); openErr != nil {
					return openErr
				}
			}
			fmt.Fprintln(app.stdout, installer.InstallGuidance())
			return err
		}
	}
	fmt.Fprintf(app.stdout, "Runtime:       %s %s\n", report.Runtime, report.Version)
	fmt.Fprintf(app.stdout, "Executable:    %s\n", report.Executable)
	fmt.Fprintf(app.stdout, "Data root:     %s\n", report.DataRoot)
	fmt.Fprintln(app.stdout, "Status:        ready")
	return nil
}

func (app *App) serveUI(ctx context.Context, args []string) error {
	flags := app.flags("serve", "Run the local Launcher interface.")
	listen := flags.String("listen", "127.0.0.1:16900", "local address to listen on")
	noOpen := flags.Bool("no-open", false, "do not open a browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("serve does not accept arguments")
	}
	if app.serve == nil {
		return errors.New("web interface is not available in this build")
	}
	return app.serve(ctx, ServeOptions{Listen: *listen, Open: !*noOpen})
}

func (app *App) catalog(ctx context.Context, args []string) error {
	flags := app.flags("catalog", "List available agent applications.")
	refresh := flags.Bool(
		"refresh",
		false,
		"refresh publisher feeds and application channels",
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("catalog does not accept arguments")
	}
	if *refresh {
		if app.refresh == nil {
			return errors.New("application registry refresh is not available")
		}
		changed, err := app.refresh(ctx)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintln(app.stdout, "Application registry refreshed.")
		} else {
			fmt.Fprintln(app.stdout, "Application registry is up to date.")
		}
	}
	table := tabwriter.NewWriter(app.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "SLUG\tAPPLICATION\tPUBLISHER\tIMAGE")
	for _, entry := range app.service.Catalog() {
		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\n",
			entry.Slug,
			entry.Name,
			entry.Publisher,
			entry.Image,
		)
	}
	return table.Flush()
}

func (app *App) create(ctx context.Context, args []string) error {
	flags := app.flags("create", "Create an agent application.")
	name := flags.String("name", "", "agent name")
	appID := flags.String(
		"app",
		"",
		"application slug or ID",
	)
	image := flags.String("image", "", "container image override")
	stopped := flags.Bool("stopped", false, "create without starting")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *name == "" && flags.NArg() == 1 {
		*name = flags.Arg(0)
	} else if flags.NArg() != 0 {
		return errors.New("create accepts one name, positional or with --name")
	}
	*appID = strings.TrimSpace(*appID)
	if *appID == "" {
		return errors.New("create requires --app with a catalogue slug or ID")
	}
	instance, err := app.service.Create(ctx, agent.CreateOptions{
		CatalogID: *appID, Name: *name, Image: *image,
		Start: !*stopped,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(app.stdout, "Created %s\n", instance.Name)
	if instance.DesiredState == domain.DesiredRunning {
		if target, exists := displayURL(instance); exists {
			fmt.Fprintf(app.stdout, "Open: %s\n", target)
		}
	} else {
		fmt.Fprintf(app.stdout, "Start with: launcher start %q\n", instance.Name)
	}
	return nil
}

func (app *App) duplicate(ctx context.Context, args []string) error {
	flags := app.flags("duplicate", "Duplicate an agent and its persistent files.")
	start := flags.Bool("start", false, "start the duplicate after copying")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 ||
		strings.TrimSpace(flags.Arg(0)) == "" ||
		strings.TrimSpace(flags.Arg(1)) == "" {
		return errors.New(
			"usage: launcher duplicate [--start] SOURCE NEW_NAME",
		)
	}
	instance, err := app.service.Duplicate(ctx, flags.Arg(0), agent.DuplicateOptions{
		Name:  flags.Arg(1),
		Start: *start,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(app.stdout, "Duplicated %s as %s\n", flags.Arg(0), instance.Name)
	if instance.DesiredState == domain.DesiredRunning {
		if target, exists := displayURL(instance); exists {
			fmt.Fprintf(app.stdout, "Open: %s\n", target)
		}
	} else {
		fmt.Fprintf(app.stdout, "Start with: launcher start %q\n", instance.Name)
	}
	return nil
}

func (app *App) list(ctx context.Context, args []string) error {
	flags := app.flags("list", "List installed agents.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("list does not accept arguments")
	}
	views, err := app.service.List(ctx)
	if err != nil {
		return err
	}
	if len(views) == 0 {
		fmt.Fprintln(
			app.stdout,
			"Library is empty. Run launcher catalog, then launcher create --app SLUG NAME.",
		)
		return nil
	}
	table := tabwriter.NewWriter(app.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tAPP\tSTATE\tENDPOINT\tIMAGE")
	for _, view := range views {
		application := view.CatalogSlug
		if application == "" {
			application = view.CatalogID
		}
		target, exists := displayURL(view.Instance)
		if !exists {
			target = "-"
		}
		fmt.Fprintf(
			table, "%s\t%s\t%s\t%s\t%s\n",
			view.Name, application, view.State, target, view.Image,
		)
	}
	return table.Flush()
}

func (app *App) status(ctx context.Context, args []string) error {
	reference, err := oneReference(app.flags("status", "Show an agent."), args)
	if err != nil {
		return err
	}
	details, err := app.service.Details(ctx, reference)
	if err != nil {
		return err
	}
	view := details.View
	fmt.Fprintf(app.stdout, "Name:          %s\n", view.Name)
	application := view.CatalogSlug
	if application == "" {
		application = view.CatalogID
	}
	fmt.Fprintf(app.stdout, "Application:   %s\n", application)
	fmt.Fprintf(app.stdout, "State:         %s\n", view.State)
	fmt.Fprintf(app.stdout, "Desired state: %s\n", view.DesiredState)
	if target, exists := displayURL(view.Instance); exists {
		fmt.Fprintf(app.stdout, "Open:          %s\n", target)
	}
	fmt.Fprintf(app.stdout, "Image:         %s\n", view.Image)
	fmt.Fprintf(app.stdout, "Container:     %s\n", view.ContainerName)
	fmt.Fprintf(app.stdout, "ID:            %s\n", view.ID)
	fmt.Fprintf(app.stdout, "Created:       %s\n", view.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(app.stdout, "Files:         %s\n", details.Files)
	for _, mount := range details.Mounts {
		source := mount.Source
		if source == "" {
			if mount.Storage == "volume" {
				source = "runtime volume"
			} else {
				source = "unavailable"
			}
		}
		fmt.Fprintf(
			app.stdout,
			"Mount %s: %s -> %s\n",
			mount.Name,
			source,
			mount.Target,
		)
	}
	interfaceIDs := make([]string, 0, len(view.Interfaces))
	for id := range view.Interfaces {
		interfaceIDs = append(interfaceIDs, id)
	}
	sort.Strings(interfaceIDs)
	for _, id := range interfaceIDs {
		resolved := view.Interfaces[id]
		fmt.Fprintf(
			app.stdout,
			"Interface %s: %s (%s)\n",
			id,
			resolved.URL(),
			resolved.Kind,
		)
	}
	fmt.Fprintf(app.stdout, "Network:       %s\n", details.Network.Name)
	fmt.Fprintf(app.stdout, "Attached:      %t\n", details.Network.Attached)
	for _, address := range details.Network.Addresses {
		fmt.Fprintf(app.stdout, "IP address:    %s\n", address)
	}
	if details.NetworkError != "" {
		fmt.Fprintf(app.stdout, "Network error: %s\n", details.NetworkError)
	}
	if view.Uptime > 0 {
		fmt.Fprintf(
			app.stdout,
			"Uptime:        %s\n",
			view.Uptime.Round(time.Second),
		)
	}
	if view.Metrics.CPUAvailable {
		fmt.Fprintf(app.stdout, "CPU:           %.2f%%\n", view.Metrics.CPUPercent)
	}
	if view.Metrics.MemoryAvailable {
		fmt.Fprintf(
			app.stdout,
			"Memory:        %.2f%%",
			view.Metrics.MemoryPercent,
		)
		if view.Metrics.MemoryUsageBytes > 0 || view.Metrics.MemoryLimitBytes > 0 {
			fmt.Fprintf(
				app.stdout,
				" (%s / %s)",
				formatBytes(view.Metrics.MemoryUsageBytes),
				formatBytes(view.Metrics.MemoryLimitBytes),
			)
		}
		fmt.Fprintln(app.stdout)
	}
	if view.MetricsError != "" {
		fmt.Fprintf(app.stdout, "Metrics error: %s\n", view.MetricsError)
	}
	return nil
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	scaled := float64(value)
	index := -1
	for scaled >= unit && index < len(units)-1 {
		scaled /= unit
		index++
	}
	return fmt.Sprintf("%.1f %s", scaled, units[index])
}

func (app *App) start(ctx context.Context, args []string) error {
	reference, err := oneReference(app.flags("start", "Start an agent."), args)
	if err != nil {
		return err
	}
	instance, err := app.service.Start(ctx, reference)
	if err != nil {
		return err
	}
	fmt.Fprintf(app.stdout, "Started %s\n", instance.Name)
	if target, exists := displayURL(instance); exists {
		fmt.Fprintf(app.stdout, "Open: %s\n", target)
	}
	return nil
}

func (app *App) stop(ctx context.Context, args []string) error {
	reference, err := oneReference(app.flags("stop", "Stop an agent."), args)
	if err != nil {
		return err
	}
	instance, err := app.service.Stop(ctx, reference)
	if err != nil {
		return err
	}
	fmt.Fprintf(app.stdout, "Stopped %s\n", instance.Name)
	return nil
}

func (app *App) open(ctx context.Context, args []string) error {
	reference, err := oneReference(app.flags("open", "Open an agent."), args)
	if err != nil {
		return err
	}
	view, err := app.service.Get(ctx, reference)
	if err != nil {
		return err
	}
	target, exists := displayURL(view.Instance)
	if !exists {
		return fmt.Errorf("%s has no display interface", view.Name)
	}
	fmt.Fprintln(app.stdout, target)
	return app.opener.Open(target)
}

func displayURL(instance domain.Instance) (string, bool) {
	_, resolved, exists := instance.DisplayInterface()
	if !exists {
		return "", false
	}
	return resolved.URL(), true
}

func (app *App) logs(ctx context.Context, args []string) error {
	flags := app.flags("logs", "Show agent logs.")
	follow := flags.Bool("follow", false, "follow log output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: launcher logs [--follow] NAME")
	}
	return app.service.Logs(ctx, flags.Arg(0), *follow)
}

func (app *App) exec(ctx context.Context, args []string) error {
	flags := app.flags("exec", "Execute a command inside a running agent.")
	var tty bool
	flags.BoolVar(&tty, "tty", false, "allocate a pseudo-terminal")
	flags.BoolVar(&tty, "t", false, "allocate a pseudo-terminal")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() < 2 ||
		strings.TrimSpace(flags.Arg(0)) == "" ||
		strings.TrimSpace(flags.Arg(1)) == "" {
		return errors.New("usage: launcher exec [--tty] NAME COMMAND [ARG...]")
	}
	return app.service.Exec(ctx, flags.Arg(0), agent.ExecOptions{
		Command: append([]string(nil), flags.Args()[1:]...),
		Stdin:   app.input,
		TTY:     tty,
	})
}

func (app *App) delete(ctx context.Context, args []string) error {
	flags := app.flags("delete", "Delete an agent and all of its local files.")
	force := flags.Bool("force", false, "confirm permanent deletion")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: launcher delete --force NAME")
	}
	if !*force {
		return errors.New("deletion is permanent; repeat with --force")
	}
	if err := app.service.Delete(ctx, flags.Arg(0)); err != nil {
		return err
	}
	fmt.Fprintf(app.stdout, "Deleted %s\n", flags.Arg(0))
	return nil
}

func (app *App) cleanup(ctx context.Context, args []string) error {
	flags := app.flags("cleanup", "Remove old images previously pulled by Launcher.")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("cleanup does not accept arguments")
	}
	report, err := app.service.CleanupImages(ctx, agent.DefaultImageRetention)
	fmt.Fprintf(
		app.stdout,
		"Removed %d unused image(s); %d protected, %d deferred, %d tracked.\n",
		report.Removed,
		report.Protected,
		report.Deferred,
		report.Tracked,
	)
	return err
}

type runtimeInstaller interface {
	error
	RuntimeName() string
	InstallURL() string
	InstallGuidance() string
}
type runtimeServiceStarter interface {
	error
	RuntimeName() string
	StartService(context.Context) error
}

func (app *App) confirm(question string) (bool, error) {
	fmt.Fprintf(app.stdout, "%s [y/N] ", question)
	answer, err := bufio.NewReader(app.input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func (app *App) flags(name string, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	flags.Usage = func() {
		fmt.Fprintln(app.stderr, description)
		fmt.Fprintln(app.stderr)
		fmt.Fprintf(app.stderr, "Usage: launcher %s [options]\n", name)
		flags.PrintDefaults()
	}
	return flags
}

func oneReference(flags *flag.FlagSet, args []string) (string, error) {
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() != 1 || strings.TrimSpace(flags.Arg(0)) == "" {
		return "", fmt.Errorf("usage: launcher %s NAME", flags.Name())
	}
	return flags.Arg(0), nil
}

func (app *App) printHelp() {
	fmt.Fprintln(app.stdout, `Launcher installs and runs friendly local agents.

Usage:
  launcher COMMAND [options]

Commands:
  desktop             Open the frameless desktop application
  viewer NAME         Open an agent in a framed desktop window
  serve               Open the local graphical interface
  catalog             Browse available applications
  create --app SLUG NAME
                       Create and start a catalogue application
  duplicate SOURCE NEW_NAME
                       Copy an agent and its persistent files
  list                Show the agent library
  status NAME         Show one agent
  start NAME          Start an agent
  stop NAME           Stop an agent
  open NAME           Open its desktop or dashboard
  preview --output PATH NAME
                       Save its current preview image
  logs NAME           Show its logs
  exec NAME COMMAND   Execute a command inside a running agent
  delete --force NAME Permanently delete an agent
  cleanup             Remove old Launcher images
  doctor              Check the local runtime
  guide               Print the built-in agent usage guide
  version             Print the version

Set PDPARCHITECT_LAUNCHER_HOME to override the data folder.
Set PDPARCHITECT_LAUNCHER_RUNTIME to auto, container, or docker.
Docker overrides are disabled on macOS.`)
}
