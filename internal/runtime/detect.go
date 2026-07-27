package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
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
	switch requested {
	case "", "auto":
	case "container", "apple":
		if options.GOOS != "darwin" {
			return Selection{}, errors.New(
				"Apple container runtime is only supported on macOS",
			)
		}
		if options.GOARCH != "" && options.GOARCH != "arm64" {
			return Selection{}, errors.New(
				"Apple container runtime requires Apple silicon",
			)
		}
		return detectKind(KindApple, options), nil
	case "docker":
		return detectKind(KindDocker, options), nil
	default:
		return Selection{}, fmt.Errorf(
			"unknown runtime %q; use auto, container, or docker",
			options.Requested,
		)
	}
	if options.GOOS == "darwin" &&
		(options.GOARCH == "" || options.GOARCH == "arm64") {
		if selection, found := findKind(KindApple, options); found {
			return selection, nil
		}
		if selection, found := findKind(KindDocker, options); found {
			return selection, nil
		}
		return missingSelection(KindApple, options.GOOS), nil
	}
	if selection, found := findKind(KindDocker, options); found {
		return selection, nil
	}
	return missingSelection(KindDocker, options.GOOS), nil
}

func detectKind(kind Kind, options DetectOptions) Selection {
	if selection, found := findKind(kind, options); found {
		return selection
	}
	return missingSelection(kind, options.GOOS)
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

func missingSelection(kind Kind, goos string) Selection {
	return Selection{
		Name: kind,
		Runtime: &Missing{
			kind: kind, goos: goos,
			searched: commandCandidates(kind, goos),
		},
	}
}

type Missing struct {
	kind     Kind
	goos     string
	searched []string
}

func (missing *Missing) runtimeError() error {
	return &MissingRuntimeError{
		kind: missing.kind, goos: missing.goos,
		searched: append([]string(nil), missing.searched...),
	}
}
func (missing *Missing) Doctor(context.Context) (string, error) {
	return "", missing.runtimeError()
}
func (missing *Missing) Pull(context.Context, string) error {
	return missing.runtimeError()
}
func (missing *Missing) Create(context.Context, CreateRequest) error {
	return missing.runtimeError()
}
func (missing *Missing) Start(context.Context, string) error {
	return missing.runtimeError()
}
func (missing *Missing) Stop(context.Context, string) error {
	return missing.runtimeError()
}
func (missing *Missing) Remove(context.Context, string, string) error {
	return missing.runtimeError()
}
func (missing *Missing) Status(context.Context, string) (Status, error) {
	return StatusMissing, missing.runtimeError()
}
func (missing *Missing) Stats(context.Context, string) (Metrics, error) {
	return Metrics{}, missing.runtimeError()
}
func (missing *Missing) RecentLogs(context.Context, string, int) (string, error) {
	return "", missing.runtimeError()
}
func (missing *Missing) Logs(context.Context, string, bool) error {
	return missing.runtimeError()
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
		return "On an Apple silicon Mac with macOS 26 or later, download and run the signed installer package, then run launcher doctor again."
	}
	if runtimeError.goos == "darwin" {
		return "Install Docker Desktop, start it, then run launcher doctor again."
	}
	return "Install and start Docker, then run launcher doctor again."
}
