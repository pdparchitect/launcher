package runtime

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
)

type Status string

const (
	StatusCreated    Status = "created"
	StatusRunning    Status = "running"
	StatusStopped    Status = "stopped"
	StatusRestarting Status = "restarting"
	StatusPaused     Status = "paused"
	StatusDead       Status = "dead"
	StatusMissing    Status = "missing"
)

type CreateRequest struct {
	InstanceID    string
	ContainerName string
	Network       string
	Image         string
	Ports         map[int]int
	Platform      string
	Paths         map[string]string
	Manifest      catalog.Manifest
}

type Metrics struct {
	CPUPercent       float64
	CPUAvailable     bool
	MemoryPercent    float64
	MemoryAvailable  bool
	MemoryUsageBytes uint64
	MemoryLimitBytes uint64
	StartedAt        time.Time
}

type NetworkInfo struct {
	Name      string
	Attached  bool
	Addresses []string
}

type LocalImage struct {
	ID string
}

type ExecOptions struct {
	Command []string
	Stdin   io.Reader
	TTY     bool
}

type Lifecycle interface {
	Doctor(context.Context) (string, error)
	Pull(context.Context, string, string) error
	ResolveImage(context.Context, string) (LocalImage, error)
	DeleteImage(context.Context, string) error
	EnsureNetwork(context.Context, string) error
	DeleteNetwork(context.Context, string) error
	NetworkAttached(context.Context, string, string) (bool, error)
	NetworkInfo(context.Context, string, string) (NetworkInfo, error)
	Create(context.Context, CreateRequest) error
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Remove(context.Context, string, string) error
	Status(context.Context, string) (Status, error)
	Stats(context.Context, string) (Metrics, error)
	RecentLogs(context.Context, string, int) (string, error)
	Logs(context.Context, string, bool) error
	Exec(context.Context, string, ExecOptions) error
}

func ManagedNetworkName(instanceID string) string {
	return "launcher-agent-" + instanceID
}

type Result struct {
	Stdout []byte
	Stderr []byte
}

type Runner interface {
	Capture(context.Context, string, ...string) (Result, error)
	Run(
		context.Context,
		string,
		[]string,
		io.Reader,
		io.Writer,
		io.Writer,
	) error
}

type OSRunner struct{}

func runWithCapturedError(
	ctx context.Context,
	runner Runner,
	command string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	action string,
) error {
	var capturedOutput bytes.Buffer
	var capturedError bytes.Buffer
	err := runner.Run(
		ctx,
		command,
		args,
		stdin,
		io.MultiWriter(stdout, &capturedOutput),
		io.MultiWriter(stderr, &capturedError),
	)
	if err == nil {
		return nil
	}
	return commandError(action, Result{
		Stdout: capturedOutput.Bytes(),
		Stderr: capturedError.Bytes(),
	}, err)
}

func recentLogText(result Result) string {
	stdout := string(result.Stdout)
	stderr := string(result.Stderr)
	if stdout == "" {
		return stderr
	}
	if stderr == "" {
		return stdout
	}
	if stdout[len(stdout)-1] != '\n' {
		stdout += "\n"
	}
	return stdout + stderr
}

func (OSRunner) Capture(
	ctx context.Context,
	name string,
	args ...string,
) (Result, error) {
	command := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, err
}

func (OSRunner) Run(
	ctx context.Context,
	name string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}
