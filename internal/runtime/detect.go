package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type Kind string

const (
	KindApple  Kind = "container"
	KindDocker Kind = "docker"
)

type DetectOptions struct {
	GOOS      string
	GOARCH    string
	Requested string
	LookPath  func(string) (string, error)
	Runner    Runner
	Stdout    io.Writer
	Stderr    io.Writer
}

type Selection struct {
	Name    Kind
	Path    string
	Runtime Lifecycle
}

func Detect(options DetectOptions) (Selection, error) {
	if options.LookPath == nil {
		options.LookPath = exec.LookPath
	}
	if options.Runner == nil {
		options.Runner = OSRunner{}
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}
	requested := strings.ToLower(strings.TrimSpace(options.Requested))
	if options.GOOS == "darwin" {
		switch requested {
		case "", "auto", "container", "apple":
		case "docker", "podman":
			return Selection{}, fmt.Errorf(
				"%s runtime is disabled on macOS; Launcher requires Apple container",
				requested,
			)
		default:
			return Selection{}, fmt.Errorf(
				"unknown runtime %q; macOS uses Apple container",
				options.Requested,
			)
		}
		if options.GOARCH != "" && options.GOARCH != "arm64" {
			return Selection{}, errors.New(
				"Apple container runtime requires Apple silicon",
			)
		}
		return detectKind(KindApple, options), nil
	}
	switch requested {
	case "", "auto":
	case "container", "apple":
		return Selection{}, errors.New(
			"Apple container runtime is only supported on macOS",
		)
	case "docker":
		return detectKind(KindDocker, options), nil
	default:
		return Selection{}, fmt.Errorf(
			"unknown runtime %q; use auto, container, or docker",
			options.Requested,
		)
	}
	if selection, found := findKind(KindDocker, options); found {
		return selection, nil
	}
	return missingSelection(KindDocker, options), nil
}

func detectKind(kind Kind, options DetectOptions) Selection {
	if selection, found := findKind(kind, options); found {
		return selection
	}
	return missingSelection(kind, options)
}

func findKind(kind Kind, options DetectOptions) (Selection, bool) {
	for _, candidate := range commandCandidates(kind, options.GOOS) {
		path, err := options.LookPath(candidate)
		if err != nil {
			continue
		}
		if kind == KindApple {
			return Selection{
				Name: kind,
				Path: path,
				Runtime: NewApple(
					path, options.Runner, options.Stdout, options.Stderr,
				),
			}, true
		}
		return Selection{
			Name: kind,
			Path: path,
			Runtime: NewDocker(
				path, options.Runner, options.Stdout, options.Stderr,
			),
		}, true
	}
	return Selection{}, false
}

func commandCandidates(kind Kind, goos string) []string {
	candidates := []string{string(kind)}
	if goos != "darwin" {
		return candidates
	}
	switch kind {
	case KindApple:
		return append(
			candidates,
			"/usr/local/bin/container",
			"/opt/homebrew/bin/container",
		)
	case KindDocker:
		return append(
			candidates,
			"/usr/local/bin/docker",
			"/opt/homebrew/bin/docker",
			"/Applications/Docker.app/Contents/Resources/bin/docker",
		)
	default:
		return candidates
	}
}

func missingSelection(kind Kind, options DetectOptions) Selection {
	return Selection{
		Name: kind,
		Runtime: &Missing{
			kind: kind, goos: options.GOOS,
			searched: commandCandidates(kind, options.GOOS),
			options:  options,
		},
	}
}

type Missing struct {
	kind     Kind
	goos     string
	searched []string
	options  DetectOptions
	mu       sync.RWMutex
	resolved Lifecycle
	path     string
}

func (missing *Missing) runtimeError() error {
	return &MissingRuntimeError{
		kind: missing.kind, goos: missing.goos,
		searched: append([]string(nil), missing.searched...),
	}
}
func (missing *Missing) resolve() (Lifecycle, error) {
	missing.mu.RLock()
	resolved := missing.resolved
	missing.mu.RUnlock()
	if resolved != nil {
		return resolved, nil
	}

	missing.mu.Lock()
	defer missing.mu.Unlock()
	if missing.resolved != nil {
		return missing.resolved, nil
	}
	selection, found := findKind(missing.kind, missing.options)
	if !found {
		return nil, missing.runtimeError()
	}
	missing.resolved = selection.Runtime
	missing.path = selection.Path
	return missing.resolved, nil
}
func (missing *Missing) RuntimePath() string {
	missing.mu.RLock()
	defer missing.mu.RUnlock()
	return missing.path
}
func (missing *Missing) Doctor(ctx context.Context) (string, error) {
	resolved, err := missing.resolve()
	if err != nil {
		return "", err
	}
	return resolved.Doctor(ctx)
}
func (missing *Missing) Pull(ctx context.Context, image string) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	return resolved.Pull(ctx, image)
}
func (missing *Missing) PullWithProgress(
	ctx context.Context,
	image string,
	progress func(string),
) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	if progressRuntime, ok := resolved.(interface {
		PullWithProgress(context.Context, string, func(string)) error
	}); ok {
		return progressRuntime.PullWithProgress(ctx, image, progress)
	}
	return resolved.Pull(ctx, image)
}
func (missing *Missing) Create(ctx context.Context, request CreateRequest) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	return resolved.Create(ctx, request)
}
func (missing *Missing) Start(ctx context.Context, name string) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	return resolved.Start(ctx, name)
}
func (missing *Missing) Stop(ctx context.Context, name string) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	return resolved.Stop(ctx, name)
}
func (missing *Missing) Remove(
	ctx context.Context,
	name string,
	instanceID string,
) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	return resolved.Remove(ctx, name, instanceID)
}
func (missing *Missing) Status(ctx context.Context, name string) (Status, error) {
	resolved, err := missing.resolve()
	if err != nil {
		return StatusMissing, err
	}
	return resolved.Status(ctx, name)
}
func (missing *Missing) Stats(ctx context.Context, name string) (Metrics, error) {
	resolved, err := missing.resolve()
	if err != nil {
		return Metrics{}, err
	}
	return resolved.Stats(ctx, name)
}
func (missing *Missing) RecentLogs(
	ctx context.Context,
	name string,
	lines int,
) (string, error) {
	resolved, err := missing.resolve()
	if err != nil {
		return "", err
	}
	return resolved.RecentLogs(ctx, name, lines)
}
func (missing *Missing) Logs(
	ctx context.Context,
	name string,
	follow bool,
) error {
	resolved, err := missing.resolve()
	if err != nil {
		return err
	}
	return resolved.Logs(ctx, name, follow)
}

type MissingRuntimeError struct {
	kind     Kind
	goos     string
	searched []string
}

func (runtimeError *MissingRuntimeError) Error() string {
	message := fmt.Sprintf(
		"%s runtime is not installed or is not visible to Launcher",
		runtimeError.RuntimeName(),
	)
	if len(runtimeError.searched) > 0 {
		message += "; searched " + strings.Join(runtimeError.searched, ", ")
	}
	return message
}
func (runtimeError *MissingRuntimeError) RuntimeName() string {
	if runtimeError.kind == KindApple {
		return "Apple container"
	}
	return "Docker"
}
func (runtimeError *MissingRuntimeError) InstallURL() string {
	if runtimeError.kind == KindApple {
		return "https://github.com/apple/container/releases/latest"
	}
	return "https://docs.docker.com/get-started/get-docker/"
}
func (runtimeError *MissingRuntimeError) InstallGuidance() string {
	if runtimeError.kind == KindApple {
		return "On an Apple silicon Mac with macOS 26 or later, download and run the signed installer package, then return to Launcher and check the runtime again."
	}
	if runtimeError.goos == "darwin" {
		return "Install Docker Desktop, start it, then return to Launcher and check the runtime again."
	}
	return "Install and start Docker, then return to Launcher and check the runtime again."
}
