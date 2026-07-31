package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
)

type Apple struct {
	command string
	runner  Runner
	stdout  io.Writer
	stderr  io.Writer
	now     func() time.Time
	samples map[string]appleCPUSample
	mu      sync.Mutex
}

type appleContainer struct {
	Configuration struct {
		Labels map[string]string `json:"labels"`
	} `json:"configuration"`
	Status struct {
		State     string    `json:"state"`
		StartedAt time.Time `json:"startedAt"`
	} `json:"status"`
}

type appleStats struct {
	CPUPercent       *float64 `json:"cpuPercent"`
	CPUUsageUsec     uint64   `json:"cpuUsageUsec"`
	MemoryUsageBytes uint64   `json:"memoryUsageBytes"`
	MemoryLimitBytes uint64   `json:"memoryLimitBytes"`
}

type appleCPUSample struct {
	usageUsec uint64
	at        time.Time
}

type appleVersion struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

func NewApple(
	command string,
	runner Runner,
	stdout io.Writer,
	stderr io.Writer,
) *Apple {
	return &Apple{
		command: command,
		runner:  runner,
		stdout:  stdout,
		stderr:  stderr,
		now:     time.Now,
		samples: make(map[string]appleCPUSample),
	}
}

func (apple *Apple) Doctor(ctx context.Context) (string, error) {
	result, err := apple.runner.Capture(
		ctx, apple.command, "system", "status", "--format", "json",
	)
	if err != nil {
		return "", &AppleServiceStoppedError{
			apple: apple,
			cause: commandError("check Apple container service", result, err),
		}
	}
	result, err = apple.runner.Capture(
		ctx, apple.command, "system", "version", "--format", "json",
	)
	if err != nil {
		return "", commandError("inspect Apple container version", result, err)
	}
	var versions []appleVersion
	if err := json.Unmarshal(result.Stdout, &versions); err != nil {
		return "", fmt.Errorf("decode Apple container version: %w", err)
	}
	for _, version := range versions {
		if version.AppName == "container" {
			return version.Version, nil
		}
	}
	return "unknown", nil
}

func (apple *Apple) Pull(
	ctx context.Context,
	image string,
	platform string,
) error {
	return apple.PullWithProgress(ctx, image, platform, nil)
}

func (apple *Apple) PullWithProgress(
	ctx context.Context,
	image string,
	platform string,
	progress func(string),
) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("image is required")
	}
	stdout := newProgressWriter(apple.stdout, progress)
	stderr := newProgressWriter(apple.stderr, progress)
	defer stdout.Flush()
	defer stderr.Flush()
	args := []string{"image", "pull"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, image)
	if err := apple.runner.Run(
		ctx,
		apple.command,
		args,
		nil,
		stdout,
		stderr,
	); err != nil {
		return fmt.Errorf("pull image %q: %w", image, err)
	}
	return nil
}

func (apple *Apple) Create(ctx context.Context, request CreateRequest) error {
	args, err := createArguments(request, true)
	if err != nil {
		return err
	}
	if err := apple.runner.Run(
		ctx, apple.command, args, nil, io.Discard, apple.stderr,
	); err != nil {
		return fmt.Errorf("create container %q: %w", request.ContainerName, err)
	}
	return nil
}

func (apple *Apple) Start(ctx context.Context, name string) error {
	return apple.simple(ctx, "start", name)
}

func (apple *Apple) Stop(ctx context.Context, name string) error {
	status, err := apple.Status(ctx, name)
	if err != nil {
		return err
	}
	if status == StatusMissing || status == StatusStopped {
		return nil
	}
	return apple.simple(ctx, "stop", name)
}

func (apple *Apple) Remove(
	ctx context.Context,
	name string,
	instanceID string,
) error {
	container, err := apple.inspect(ctx, name)
	if errors.Is(err, errAppleContainerMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	owner := container.Configuration.Labels[managedLabel]
	if owner != instanceID {
		return fmt.Errorf(
			"refusing to remove container %q: managed by %q, expected %q",
			name, owner, instanceID,
		)
	}
	if err := apple.runner.Run(
		ctx,
		apple.command,
		[]string{"delete", "--force", name},
		nil,
		io.Discard,
		apple.stderr,
	); err != nil {
		return fmt.Errorf("remove container %q: %w", name, err)
	}
	return nil
}

// DeleteMountData removes runtime-native volumes after the owning agent is
// deleted. Container replacement during an image update intentionally does
// not call this method, so the same deterministic volume is reattached.
func (apple *Apple) DeleteMountData(
	ctx context.Context,
	instanceID string,
	manifest catalog.Manifest,
) error {
	for _, mount := range manifest.Mounts {
		if mount.Storage != catalog.MountStorageVolume {
			continue
		}
		name := managedVolumeName(instanceID, mount.Name)
		result, err := apple.runner.Capture(
			ctx, apple.command, "volume", "delete", name,
		)
		if err != nil && !missingAppleVolume(result) {
			return commandError("delete Apple container volume "+name, result, err)
		}
	}
	return nil
}

func missingAppleVolume(result Result) bool {
	message := strings.ToLower(string(result.Stdout) + string(result.Stderr))
	return strings.Contains(message, "not found") ||
		strings.Contains(message, "no such volume") ||
		strings.Contains(message, "does not exist")
}

func (apple *Apple) Status(ctx context.Context, name string) (Status, error) {
	container, err := apple.inspect(ctx, name)
	if errors.Is(err, errAppleContainerMissing) {
		return StatusMissing, nil
	}
	if err != nil {
		return StatusMissing, err
	}
	return Status(container.Status.State), nil
}

func (apple *Apple) Stats(ctx context.Context, name string) (Metrics, error) {
	result, err := apple.runner.Capture(
		ctx,
		apple.command,
		"stats",
		"--format",
		"json",
		"--no-stream",
		name,
	)
	if err != nil {
		return Metrics{}, commandError("read Apple container statistics", result, err)
	}
	var records []appleStats
	if err := json.Unmarshal(result.Stdout, &records); err != nil {
		return Metrics{}, fmt.Errorf("decode Apple container statistics: %w", err)
	}
	if len(records) != 1 {
		return Metrics{}, fmt.Errorf(
			"Apple container statistics for %q returned %d records",
			name,
			len(records),
		)
	}
	record := records[0]
	metrics := Metrics{
		MemoryUsageBytes: record.MemoryUsageBytes,
		MemoryLimitBytes: record.MemoryLimitBytes,
	}
	if record.MemoryLimitBytes > 0 {
		metrics.MemoryPercent =
			float64(record.MemoryUsageBytes) / float64(record.MemoryLimitBytes) * 100
		metrics.MemoryAvailable = true
	}
	sampledAt := apple.now()
	apple.mu.Lock()
	previous, hasPrevious := apple.samples[name]
	apple.samples[name] = appleCPUSample{
		usageUsec: record.CPUUsageUsec,
		at:        sampledAt,
	}
	apple.mu.Unlock()
	if record.CPUPercent != nil {
		metrics.CPUPercent = *record.CPUPercent
		metrics.CPUAvailable = true
	} else if hasPrevious &&
		record.CPUUsageUsec >= previous.usageUsec &&
		sampledAt.After(previous.at) {
		elapsedUsec := sampledAt.Sub(previous.at).Microseconds()
		metrics.CPUPercent =
			float64(record.CPUUsageUsec-previous.usageUsec) /
				float64(elapsedUsec) * 100
		metrics.CPUAvailable = true
	}
	container, inspectErr := apple.inspect(ctx, name)
	if inspectErr == nil {
		metrics.StartedAt = container.Status.StartedAt
	}
	return metrics, nil
}

func (apple *Apple) Logs(
	ctx context.Context,
	name string,
	follow bool,
) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, name)
	if err := apple.runner.Run(
		ctx, apple.command, args, nil, apple.stdout, apple.stderr,
	); err != nil {
		return fmt.Errorf("read container logs for %q: %w", name, err)
	}
	return nil
}

func (apple *Apple) RecentLogs(
	ctx context.Context,
	name string,
	lines int,
) (string, error) {
	if lines < 1 {
		return "", errors.New("log line count must be positive")
	}
	result, err := apple.runner.Capture(
		ctx,
		apple.command,
		"logs",
		"-n",
		fmt.Sprint(lines),
		name,
	)
	if err != nil {
		return "", commandError("read recent Apple container logs", result, err)
	}
	return recentLogText(result), nil
}

func (apple *Apple) inspect(
	ctx context.Context,
	name string,
) (appleContainer, error) {
	result, err := apple.runner.Capture(ctx, apple.command, "inspect", name)
	if err != nil {
		message := strings.ToLower(string(result.Stderr) + string(result.Stdout))
		if strings.Contains(message, "not found") ||
			strings.Contains(message, "does not exist") {
			return appleContainer{}, errAppleContainerMissing
		}
		return appleContainer{}, commandError("inspect Apple container", result, err)
	}
	var containers []appleContainer
	if err := json.Unmarshal(result.Stdout, &containers); err != nil {
		return appleContainer{}, fmt.Errorf("decode Apple container inspection: %w", err)
	}
	if len(containers) != 1 {
		return appleContainer{}, fmt.Errorf(
			"inspect Apple container %q returned %d records",
			name, len(containers),
		)
	}
	return containers[0], nil
}

func (apple *Apple) simple(
	ctx context.Context,
	action string,
	name string,
) error {
	if err := apple.runner.Run(
		ctx,
		apple.command,
		[]string{action, name},
		nil,
		io.Discard,
		apple.stderr,
	); err != nil {
		return fmt.Errorf("%s container %q: %w", action, name, err)
	}
	return nil
}

var errAppleContainerMissing = errors.New("Apple container not found")

type AppleServiceStoppedError struct {
	apple *Apple
	cause error
}

func (runtimeError *AppleServiceStoppedError) Error() string {
	return fmt.Sprintf("Apple container service is not running: %v", runtimeError.cause)
}
func (*AppleServiceStoppedError) RuntimeName() string { return "Apple container" }
func (runtimeError *AppleServiceStoppedError) StartService(
	ctx context.Context,
) error {
	if err := runtimeError.apple.runner.Run(
		ctx,
		runtimeError.apple.command,
		[]string{"system", "start", "--enable-kernel-install"},
		nil,
		runtimeError.apple.stdout,
		runtimeError.apple.stderr,
	); err != nil {
		return fmt.Errorf("start Apple container service: %w", err)
	}
	return nil
}
