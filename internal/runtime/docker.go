package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
)

const managedLabel = "dev.pdparchitect.launcher.instance"

type Docker struct {
	command string
	runner  Runner
	stdout  io.Writer
	stderr  io.Writer
}

func NewDocker(
	command string,
	runner Runner,
	stdout io.Writer,
	stderr io.Writer,
) *Docker {
	return &Docker{command: command, runner: runner, stdout: stdout, stderr: stderr}
}

func (docker *Docker) Doctor(ctx context.Context) (string, error) {
	result, err := docker.runner.Capture(
		ctx, docker.command, "version", "--format", "{{.Server.Version}}",
	)
	if err != nil {
		return "", commandError("inspect Docker server", result, err)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func (docker *Docker) Pull(
	ctx context.Context,
	image string,
	platform string,
) error {
	return docker.PullWithProgress(ctx, image, platform, nil)
}

func (docker *Docker) PullWithProgress(
	ctx context.Context,
	image string,
	platform string,
	progress func(string),
) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("image is required")
	}
	stdout := newProgressWriter(docker.stdout, progress)
	stderr := newProgressWriter(docker.stderr, progress)
	defer stdout.Flush()
	defer stderr.Flush()
	args := []string{"pull"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, image)
	if err := docker.runner.Run(
		ctx,
		docker.command,
		args,
		nil,
		stdout,
		stderr,
	); err != nil {
		return fmt.Errorf("pull image %q: %w", image, err)
	}
	return nil
}

func (docker *Docker) Create(ctx context.Context, request CreateRequest) error {
	args, err := createArguments(request, false)
	if err != nil {
		return err
	}
	if err := docker.runner.Run(
		ctx, docker.command, args, nil, io.Discard, docker.stderr,
	); err != nil {
		return fmt.Errorf("create container %q: %w", request.ContainerName, err)
	}
	return nil
}

func (docker *Docker) Start(ctx context.Context, name string) error {
	return docker.simple(ctx, "start", name)
}

func (docker *Docker) Stop(ctx context.Context, name string) error {
	status, err := docker.Status(ctx, name)
	if err != nil {
		return err
	}
	if status == StatusMissing || status == StatusStopped {
		return nil
	}
	return docker.simple(ctx, "stop", name)
}

func (docker *Docker) Remove(
	ctx context.Context,
	name string,
	instanceID string,
) error {
	result, err := docker.runner.Capture(
		ctx,
		docker.command,
		"container",
		"inspect",
		"--format",
		"{{ index .Config.Labels \""+managedLabel+"\" }}",
		name,
	)
	if err != nil {
		if missingContainer(result) {
			return nil
		}
		return commandError("inspect container ownership", result, err)
	}
	owner := strings.TrimSpace(string(result.Stdout))
	if owner != instanceID {
		return fmt.Errorf(
			"refusing to remove container %q: managed by %q, expected %q",
			name, owner, instanceID,
		)
	}
	if err := docker.runner.Run(
		ctx,
		docker.command,
		[]string{"rm", "--force", name},
		nil,
		io.Discard,
		docker.stderr,
	); err != nil {
		return fmt.Errorf("remove container %q: %w", name, err)
	}
	return nil
}

// DeleteMountData removes Docker-managed volumes after their owning agent is
// deleted. Image updates only replace the container and retain these volumes.
func (docker *Docker) DeleteMountData(
	ctx context.Context,
	instanceID string,
	manifest catalog.Manifest,
) error {
	for _, mount := range manifest.Mounts {
		if mount.Storage != catalog.MountStorageVolume {
			continue
		}
		name := managedVolumeName(instanceID, mount.Name)
		result, err := docker.runner.Capture(
			ctx, docker.command, "volume", "rm", name,
		)
		if err != nil && !missingDockerVolume(result) {
			return commandError("delete Docker volume "+name, result, err)
		}
	}
	return nil
}

func missingDockerVolume(result Result) bool {
	message := strings.ToLower(string(result.Stdout) + string(result.Stderr))
	return strings.Contains(message, "no such volume") ||
		strings.Contains(message, "not found")
}

func (docker *Docker) Status(ctx context.Context, name string) (Status, error) {
	result, err := docker.runner.Capture(
		ctx,
		docker.command,
		"container",
		"inspect",
		"--format",
		"{{.State.Status}}",
		name,
	)
	if err != nil {
		if missingContainer(result) {
			return StatusMissing, nil
		}
		return StatusMissing, commandError("inspect container status", result, err)
	}
	value := Status(strings.TrimSpace(string(result.Stdout)))
	if value == "exited" {
		return StatusStopped, nil
	}
	return value, nil
}

func (docker *Docker) Stats(ctx context.Context, name string) (Metrics, error) {
	result, err := docker.runner.Capture(
		ctx,
		docker.command,
		"stats",
		"--no-stream",
		"--format",
		"{{json .}}",
		name,
	)
	if err != nil {
		return Metrics{}, commandError("read container statistics", result, err)
	}
	var stats struct {
		CPUPercent    string `json:"CPUPerc"`
		MemoryPercent string `json:"MemPerc"`
		MemoryUsage   string `json:"MemUsage"`
	}
	if err := json.Unmarshal(result.Stdout, &stats); err != nil {
		return Metrics{}, fmt.Errorf("decode Docker container statistics: %w", err)
	}
	cpu, err := parsePercent(stats.CPUPercent)
	if err != nil {
		return Metrics{}, fmt.Errorf("decode Docker CPU percentage: %w", err)
	}
	memory, err := parsePercent(stats.MemoryPercent)
	if err != nil {
		return Metrics{}, fmt.Errorf("decode Docker memory percentage: %w", err)
	}
	usage, limit, err := parseMemoryUsage(stats.MemoryUsage)
	if err != nil {
		return Metrics{}, fmt.Errorf("decode Docker memory usage: %w", err)
	}
	result, err = docker.runner.Capture(
		ctx,
		docker.command,
		"container",
		"inspect",
		"--format",
		"{{.State.StartedAt}}",
		name,
	)
	if err != nil {
		return Metrics{}, commandError("inspect container start time", result, err)
	}
	startedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(result.Stdout)))
	if err != nil {
		return Metrics{}, fmt.Errorf("decode Docker container start time: %w", err)
	}
	return Metrics{
		CPUPercent:       cpu,
		CPUAvailable:     true,
		MemoryPercent:    memory,
		MemoryAvailable:  true,
		MemoryUsageBytes: usage,
		MemoryLimitBytes: limit,
		StartedAt:        startedAt,
	}, nil
}

func (docker *Docker) Logs(
	ctx context.Context,
	name string,
	follow bool,
) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, name)
	if err := docker.runner.Run(
		ctx, docker.command, args, nil, docker.stdout, docker.stderr,
	); err != nil {
		return fmt.Errorf("read container logs for %q: %w", name, err)
	}
	return nil
}

func (docker *Docker) RecentLogs(
	ctx context.Context,
	name string,
	lines int,
) (string, error) {
	if lines < 1 {
		return "", errors.New("log line count must be positive")
	}
	result, err := docker.runner.Capture(
		ctx,
		docker.command,
		"logs",
		"--tail",
		strconv.Itoa(lines),
		name,
	)
	if err != nil {
		return "", commandError("read recent container logs", result, err)
	}
	return recentLogText(result), nil
}

func parsePercent(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")), 64)
}

func parseMemoryUsage(value string) (uint64, uint64, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected usage and limit, got %q", value)
	}
	usage, err := parseBytes(parts[0])
	if err != nil {
		return 0, 0, err
	}
	limit, err := parseBytes(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return usage, limit, nil
}

func parseBytes(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	index := 0
	for index < len(value) &&
		((value[index] >= '0' && value[index] <= '9') || value[index] == '.') {
		index++
	}
	number, err := strconv.ParseFloat(value[:index], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte count %q", value)
	}
	units := map[string]float64{
		"B":  1,
		"kB": 1_000, "KB": 1_000, "KiB": 1 << 10,
		"MB": 1_000_000, "MiB": 1 << 20,
		"GB": 1_000_000_000, "GiB": 1 << 30,
		"TB": 1_000_000_000_000, "TiB": 1 << 40,
	}
	multiplier, exists := units[strings.TrimSpace(value[index:])]
	if !exists {
		return 0, fmt.Errorf("unsupported byte unit in %q", value)
	}
	return uint64(number * multiplier), nil
}

func (docker *Docker) simple(
	ctx context.Context,
	action string,
	name string,
) error {
	if err := docker.runner.Run(
		ctx,
		docker.command,
		[]string{action, name},
		nil,
		io.Discard,
		docker.stderr,
	); err != nil {
		return fmt.Errorf("%s container %q: %w", action, name, err)
	}
	return nil
}

func createArguments(request CreateRequest, apple bool) ([]string, error) {
	args := []string{"create", "--name", request.ContainerName}
	if request.Platform != "" {
		args = append(args, "--platform", request.Platform)
	}
	args = append(args, "--label", managedLabel+"="+request.InstanceID)
	if apple && request.Manifest.Memory != "" {
		args = append(args, "--memory", request.Manifest.Memory)
	}
	args = append(args, "--shm-size", request.Manifest.SharedMemory)
	containerPorts := make([]int, 0, len(request.Ports))
	for containerPort := range request.Ports {
		containerPorts = append(containerPorts, containerPort)
	}
	sort.Ints(containerPorts)
	for _, containerPort := range containerPorts {
		hostPort := request.Ports[containerPort]
		if hostPort < 1 || hostPort > 65535 ||
			containerPort < 1 || containerPort > 65535 {
			return nil, errors.New("published ports must be between 1 and 65535")
		}
		args = append(
			args,
			"--publish",
			fmt.Sprintf("127.0.0.1:%d:%d", hostPort, containerPort),
		)
	}
	environment := make(map[string]string, len(request.Manifest.Environment))
	for key, value := range request.Manifest.Environment {
		environment[key] = value
	}
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--env", key+"="+environment[key])
	}
	for _, mount := range request.Manifest.Mounts {
		var source string
		if mount.Storage == catalog.MountStorageVolume {
			source = managedVolumeName(request.InstanceID, mount.Name)
		} else {
			var exists bool
			source, exists = request.Paths[mount.Name]
			if !exists {
				return nil, fmt.Errorf("mount path %q is missing", mount.Name)
			}
		}
		args = append(args, "--volume", source+":"+mount.Target)
	}
	return append(args, request.Image), nil
}

func managedVolumeName(instanceID string, mountName string) string {
	digest := sha256.Sum256([]byte(mountName))
	return fmt.Sprintf("launcher-%s-%x", instanceID, digest[:6])
}

func commandError(action string, result Result, err error) error {
	message := strings.TrimSpace(string(result.Stderr))
	if message == "" {
		message = strings.TrimSpace(string(result.Stdout))
	}
	if message == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %s: %w", action, message, err)
}

func missingContainer(result Result) bool {
	message := string(result.Stderr) + string(result.Stdout)
	return strings.Contains(message, "No such object") ||
		strings.Contains(message, "No such container")
}
