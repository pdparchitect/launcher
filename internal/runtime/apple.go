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
	Networks []struct {
		Network string `json:"network"`
		Address string `json:"address"`
	} `json:"networks"`
	Configuration struct {
		Labels map[string]string `json:"labels"`
	} `json:"configuration"`
	Status appleContainerStatus `json:"status"`
}

type appleContainerStatus struct {
	State     string
	StartedAt time.Time
}

func (status *appleContainerStatus) UnmarshalJSON(data []byte) error {
	var state string
	if err := json.Unmarshal(data, &state); err == nil {
		status.State = state
		return nil
	}
	var value struct {
		State     string    `json:"state"`
		StartedAt time.Time `json:"startedAt"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	status.State = value.State
	status.StartedAt = value.StartedAt
	return nil
}

type appleNetwork struct {
	ID            string `json:"id"`
	Configuration struct {
		Labels map[string]string `json:"labels"`
	} `json:"configuration"`
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

type appleImage struct {
	ID string `json:"id"`
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

func (apple *Apple) ResolveImage(
	ctx context.Context,
	reference string,
) (LocalImage, error) {
	if strings.TrimSpace(reference) == "" {
		return LocalImage{}, errors.New("image reference is required")
	}
	result, err := apple.runner.Capture(
		ctx,
		apple.command,
		"image",
		"inspect",
		reference,
	)
	if err != nil {
		return LocalImage{}, commandError(
			"inspect image "+reference,
			result,
			err,
		)
	}
	var images []appleImage
	if err := json.Unmarshal(result.Stdout, &images); err != nil {
		return LocalImage{}, fmt.Errorf("decode Apple container image inspection: %w", err)
	}
	if len(images) != 1 || strings.TrimSpace(images[0].ID) == "" {
		return LocalImage{}, fmt.Errorf(
			"inspect image %q returned %d invalid records",
			reference,
			len(images),
		)
	}
	return LocalImage{ID: strings.TrimSpace(images[0].ID)}, nil
}

func (apple *Apple) DeleteImage(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("image ID is required")
	}
	result, err := apple.runner.Capture(
		ctx,
		apple.command,
		"image",
		"delete",
		id,
	)
	if err != nil {
		if missingAppleImage(result) {
			return nil
		}
		return commandError("delete image "+id, result, err)
	}
	return nil
}

func missingAppleImage(result Result) bool {
	message := strings.ToLower(string(result.Stdout) + string(result.Stderr))
	return strings.Contains(message, "image not found") ||
		strings.Contains(message, "not found") ||
		strings.Contains(message, "does not exist")
}

func (apple *Apple) EnsureNetwork(
	ctx context.Context,
	instanceID string,
) error {
	name := ManagedNetworkName(instanceID)
	owner, exists, err := apple.networkOwner(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		if owner != instanceID {
			return fmt.Errorf(
				"refusing to use network %q: managed by %q, expected %q",
				name,
				owner,
				instanceID,
			)
		}
		return nil
	}
	if err := apple.runner.Run(
		ctx,
		apple.command,
		[]string{
			"network", "create",
			"--label", managedLabel + "=" + instanceID,
			name,
		},
		nil,
		io.Discard,
		apple.stderr,
	); err != nil {
		return fmt.Errorf("create agent network %q: %w", name, err)
	}
	return nil
}

func (apple *Apple) DeleteNetwork(
	ctx context.Context,
	instanceID string,
) error {
	name := ManagedNetworkName(instanceID)
	owner, exists, err := apple.networkOwner(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if owner != instanceID {
		return fmt.Errorf(
			"refusing to remove network %q: managed by %q, expected %q",
			name,
			owner,
			instanceID,
		)
	}
	if err := apple.runner.Run(
		ctx,
		apple.command,
		[]string{"network", "delete", name},
		nil,
		io.Discard,
		apple.stderr,
	); err != nil {
		return fmt.Errorf("remove agent network %q: %w", name, err)
	}
	return nil
}

func (apple *Apple) NetworkAttached(
	ctx context.Context,
	containerName string,
	instanceID string,
) (bool, error) {
	info, err := apple.NetworkInfo(ctx, containerName, instanceID)
	return info.Attached, err
}

func (apple *Apple) NetworkInfo(
	ctx context.Context,
	containerName string,
	instanceID string,
) (NetworkInfo, error) {
	wanted := ManagedNetworkName(instanceID)
	container, err := apple.inspect(ctx, containerName)
	if err != nil {
		return NetworkInfo{Name: wanted}, err
	}
	for _, network := range container.Networks {
		if network.Network == wanted {
			info := NetworkInfo{Name: wanted, Attached: true}
			if address := strings.TrimSpace(network.Address); address != "" {
				info.Addresses = append(info.Addresses, address)
			}
			return info, nil
		}
	}
	return NetworkInfo{Name: wanted}, nil
}

func (apple *Apple) networkOwner(
	ctx context.Context,
	name string,
) (string, bool, error) {
	result, err := apple.runner.Capture(
		ctx,
		apple.command,
		"network",
		"inspect",
		name,
	)
	if err != nil {
		if missingAppleNetwork(result) {
			return "", false, nil
		}
		return "", false, commandError("inspect agent network "+name, result, err)
	}
	var networks []appleNetwork
	if err := json.Unmarshal(result.Stdout, &networks); err != nil {
		return "", false, fmt.Errorf("decode agent network %q: %w", name, err)
	}
	if len(networks) != 1 || networks[0].ID != name {
		return "", false, fmt.Errorf(
			"inspect agent network %q returned %d unexpected records",
			name,
			len(networks),
		)
	}
	return networks[0].Configuration.Labels[managedLabel], true, nil
}

func missingAppleNetwork(result Result) bool {
	message := strings.ToLower(string(result.Stdout) + string(result.Stderr))
	return strings.Contains(message, "network not found") ||
		strings.Contains(message, "not found") ||
		strings.Contains(message, "does not exist")
}

func (apple *Apple) Create(ctx context.Context, request CreateRequest) error {
	args, err := createArguments(request, true)
	if err != nil {
		return err
	}
	return runWithCapturedError(
		ctx,
		apple.runner,
		apple.command,
		args,
		nil,
		io.Discard,
		apple.stderr,
		"create container "+request.ContainerName,
	)
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

func (apple *Apple) Exec(
	ctx context.Context,
	name string,
	options ExecOptions,
) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("container name is required")
	}
	if len(options.Command) == 0 || strings.TrimSpace(options.Command[0]) == "" {
		return errors.New("container command is required")
	}
	args := []string{"exec"}
	if options.Stdin != nil {
		args = append(args, "--interactive")
	}
	if options.TTY {
		args = append(args, "--tty")
	}
	args = append(args, name)
	args = append(args, options.Command...)
	if err := apple.runner.Run(
		ctx,
		apple.command,
		args,
		options.Stdin,
		apple.stdout,
		apple.stderr,
	); err != nil {
		return fmt.Errorf("execute command in Apple container %q: %w", name, err)
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
	return runWithCapturedError(
		ctx,
		apple.runner,
		apple.command,
		[]string{action, name},
		nil,
		io.Discard,
		apple.stderr,
		action+" container "+name,
	)
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
