package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
)

var errExit = errors.New("exit status 1")

func TestDockerCreateBuildsConstrainedCommand(t *testing.T) {
	runner := &fakeRunner{}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	if err := docker.Create(t.Context(), testCreateRequest("linux/amd64")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	want := []string{
		"create",
		"--name", "launcher-ghost-aaaaaaaaaaaa",
		"--platform", "linux/amd64",
		"--label", managedLabel + "=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--network", ManagedNetworkName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		"--shm-size", "1g",
		"--publish", "127.0.0.1:16902:6901",
		"--env", "PANTALK_AUTOSTART=true",
		"--volume", "/data/agents/a/workspace:/workspace",
		"pantalk/ghost:test",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("Create() args = %#v\nwant %#v", runner.runArgs, want)
	}
}

func TestAppleCreateBuildsSupportedCommand(t *testing.T) {
	runner := &fakeRunner{}
	apple := NewApple("container", runner, io.Discard, io.Discard)
	if err := apple.Create(t.Context(), testCreateRequest("linux/arm64")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	want := []string{
		"create",
		"--name", "launcher-ghost-aaaaaaaaaaaa",
		"--platform", "linux/arm64",
		"--label", managedLabel + "=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--network", ManagedNetworkName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		"--memory", "4g",
		"--shm-size", "1g",
		"--publish", "127.0.0.1:16902:6901",
		"--env", "PANTALK_AUTOSTART=true",
		"--volume", "/data/agents/a/workspace:/workspace",
		"pantalk/ghost:test",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("Create() args = %#v\nwant %#v", runner.runArgs, want)
	}
}

func TestDockerExecStreamsCommandThroughProvider(t *testing.T) {
	runner := &fakeRunner{runStdout: "output", runStderr: "warning"}
	var stdout, stderr bytes.Buffer
	input := strings.NewReader("input")
	docker := NewDocker("docker", runner, &stdout, &stderr)

	err := docker.Exec(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		ExecOptions{
			Command: []string{"sh", "-c", "printf '%s' \"$HOME\""},
			Stdin:   input,
			TTY:     true,
		},
	)

	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	want := []string{
		"exec", "--interactive", "--tty",
		"launcher-ghost-aaaaaaaaaaaa",
		"sh", "-c", "printf '%s' \"$HOME\"",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("Exec() args = %#v, want %#v", runner.runArgs, want)
	}
	if runner.runStdin != input {
		t.Fatal("Exec() did not attach standard input")
	}
	if stdout.String() != "output" || stderr.String() != "warning" {
		t.Fatalf("Exec() output = (%q, %q)", stdout.String(), stderr.String())
	}
}

func TestAppleExecStreamsCommandThroughProvider(t *testing.T) {
	runner := &fakeRunner{runStdout: "output", runStderr: "warning"}
	var stdout, stderr bytes.Buffer
	input := strings.NewReader("input")
	apple := NewApple("container", runner, &stdout, &stderr)

	err := apple.Exec(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		ExecOptions{
			Command: []string{"uname", "-a"},
			Stdin:   input,
		},
	)

	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	want := []string{
		"exec", "--interactive",
		"launcher-ghost-aaaaaaaaaaaa",
		"uname", "-a",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("Exec() args = %#v, want %#v", runner.runArgs, want)
	}
	if runner.runStdin != input {
		t.Fatal("Exec() did not attach standard input")
	}
	if stdout.String() != "output" || stderr.String() != "warning" {
		t.Fatalf("Exec() output = (%q, %q)", stdout.String(), stderr.String())
	}
}

func TestDockerEnsureNetworkCreatesOwnedNetwork(t *testing.T) {
	runner := &fakeRunner{
		captureResult: Result{Stderr: []byte("network not found")},
		captureErr:    errExit,
	}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if err := docker.EnsureNetwork(t.Context(), instanceID); err != nil {
		t.Fatalf("EnsureNetwork() error = %v", err)
	}
	want := []string{
		"network", "create",
		"--label", managedLabel + "=" + instanceID,
		ManagedNetworkName(instanceID),
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("EnsureNetwork() args = %#v, want %#v", runner.runArgs, want)
	}
}

func TestAppleEnsureNetworkCreatesOwnedNetwork(t *testing.T) {
	runner := &fakeRunner{
		captureResult: Result{Stderr: []byte("network not found")},
		captureErr:    errExit,
	}
	apple := NewApple("container", runner, io.Discard, io.Discard)
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if err := apple.EnsureNetwork(t.Context(), instanceID); err != nil {
		t.Fatalf("EnsureNetwork() error = %v", err)
	}
	want := []string{
		"network", "create",
		"--label", managedLabel + "=" + instanceID,
		ManagedNetworkName(instanceID),
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("EnsureNetwork() args = %#v, want %#v", runner.runArgs, want)
	}
}

func TestDockerEnsureNetworkReusesOnlyOwnedNetwork(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(instanceID + "\n")}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)

	if err := docker.EnsureNetwork(t.Context(), instanceID); err != nil {
		t.Fatalf("EnsureNetwork() error = %v", err)
	}
	if runner.runCalled {
		t.Fatal("EnsureNetwork() created an existing owned network")
	}

	runner.captureResult.Stdout = []byte("someone-else\n")
	if err := docker.EnsureNetwork(t.Context(), instanceID); err == nil ||
		!strings.Contains(err.Error(), "refusing to use network") {
		t.Fatalf("EnsureNetwork() error = %v, want ownership error", err)
	}
}

func TestAppleDeleteNetworkRemovesOnlyOwnedNetwork(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	name := ManagedNetworkName(instanceID)
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(`[
  {
    "id": "` + name + `",
    "configuration": {
      "labels": {"` + managedLabel + `": "` + instanceID + `"}
    }
  }
]`)}}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	if err := apple.DeleteNetwork(t.Context(), instanceID); err != nil {
		t.Fatalf("DeleteNetwork() error = %v", err)
	}
	want := []string{"network", "delete", name}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("DeleteNetwork() args = %#v, want %#v", runner.runArgs, want)
	}
}

func TestDockerNetworkAttachedFindsManagedNetwork(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	name := ManagedNetworkName(instanceID)
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(
		`{"bridge":{},"` + name + `":{}}`,
	)}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)

	attached, err := docker.NetworkAttached(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		instanceID,
	)
	if err != nil || !attached {
		t.Fatalf("NetworkAttached() = %t, %v", attached, err)
	}
}

func TestDockerNetworkInfoReadsManagedNetworkAddresses(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	name := ManagedNetworkName(instanceID)
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(
		`{"` + name + `":{"IPAddress":"172.20.0.2","GlobalIPv6Address":"fd00::2"}}`,
	)}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)

	info, err := docker.NetworkInfo(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		instanceID,
	)

	if err != nil || !info.Attached || info.Name != name ||
		!reflect.DeepEqual(info.Addresses, []string{"172.20.0.2", "fd00::2"}) {
		t.Fatalf("NetworkInfo() = %#v, %v", info, err)
	}
}

func TestAppleNetworkAttachedFindsManagedNetwork(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	name := ManagedNetworkName(instanceID)
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(`[
  {
    "networks": [{"network": "` + name + `"}],
    "configuration": {"labels": {}},
    "status": {"state": "stopped"}
  }
]`)}}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	attached, err := apple.NetworkAttached(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		instanceID,
	)
	if err != nil || !attached {
		t.Fatalf("NetworkAttached() = %t, %v", attached, err)
	}
}

func TestAppleNetworkInfoReadsManagedNetworkAddress(t *testing.T) {
	instanceID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	name := ManagedNetworkName(instanceID)
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(`[
  {
    "networks": [{"network": "` + name + `", "address": "192.168.64.3/24"}],
    "configuration": {"labels": {}},
    "status": "running"
  }
]`)}}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	info, err := apple.NetworkInfo(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		instanceID,
	)

	if err != nil || !info.Attached || info.Name != name ||
		!reflect.DeepEqual(info.Addresses, []string{"192.168.64.3/24"}) {
		t.Fatalf("NetworkInfo() = %#v, %v", info, err)
	}
}

func TestAppleCreateUsesNativeVolumeForVolumeStorage(t *testing.T) {
	runner := &fakeRunner{}
	apple := NewApple("container", runner, io.Discard, io.Discard)
	request := testCreateRequest("linux/arm64")
	request.Manifest.Mounts = append(request.Manifest.Mounts, catalog.Mount{
		Name: "private/services", Target: "/var/lib/services",
		Storage: catalog.MountStorageVolume,
	})
	request.Paths["private/services"] = "/data/agents/a/private/services"

	if err := apple.Create(t.Context(), request); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	wantVolume := managedVolumeName(request.InstanceID, "private/services")
	want := []string{
		"create",
		"--name", "launcher-ghost-aaaaaaaaaaaa",
		"--platform", "linux/arm64",
		"--label", managedLabel + "=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"--network", ManagedNetworkName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		"--memory", "4g",
		"--shm-size", "1g",
		"--publish", "127.0.0.1:16902:6901",
		"--env", "PANTALK_AUTOSTART=true",
		"--volume", "/data/agents/a/workspace:/workspace",
		"--volume", wantVolume + ":/var/lib/services",
		"pantalk/ghost:test",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("Create() args = %#v\nwant %#v", runner.runArgs, want)
	}
}

func TestAppleCreatePreservesUnderlyingRuntimeError(t *testing.T) {
	runner := &fakeRunner{
		runErr:    errExit,
		runStderr: "invalid storage-device attachment",
	}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	err := apple.Create(t.Context(), testCreateRequest("linux/arm64"))

	if err == nil ||
		!strings.Contains(err.Error(), "invalid storage-device attachment") ||
		!strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestCreateArgumentsRejectsOneSourceForMultipleTargets(t *testing.T) {
	request := testCreateRequest("linux/arm64")
	request.Manifest.Mounts = []catalog.Mount{
		{Name: "first", Target: "/first"},
		{Name: "second", Target: "/second"},
	}
	request.Paths = map[string]string{
		"first":  "/host/shared",
		"second": "/host/shared",
	}

	_, err := createArguments(request, true)

	if err == nil || !strings.Contains(
		err.Error(),
		"cannot be attached to both",
	) {
		t.Fatalf("createArguments() error = %v", err)
	}
}

func TestDockerCreateUsesNamedVolumeForVolumeStorage(t *testing.T) {
	runner := &fakeRunner{}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	request := testCreateRequest("linux/amd64")
	request.Manifest.Mounts = append(request.Manifest.Mounts, catalog.Mount{
		Name: "private/services", Target: "/var/lib/services",
		Storage: catalog.MountStorageVolume,
	})
	request.Paths["private/services"] = "/data/agents/a/private/services"

	if err := docker.Create(t.Context(), request); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	wantMount := managedVolumeName(
		request.InstanceID, "private/services",
	) + ":/var/lib/services"
	if !containsString(runner.runArgs, wantMount) {
		t.Fatalf("Create() args = %#v, missing %q", runner.runArgs, wantMount)
	}
}

func TestAppleDeleteMountDataRemovesRuntimeVolumes(t *testing.T) {
	runner := &fakeRunner{}
	apple := NewApple("container", runner, io.Discard, io.Discard)
	manifest := testCreateRequest("linux/arm64").Manifest
	manifest.Mounts = append(manifest.Mounts, catalog.Mount{
		Name: "private/services", Target: "/var/lib/services",
		Storage: catalog.MountStorageVolume,
	})

	err := apple.DeleteMountData(
		t.Context(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", manifest,
	)
	if err != nil {
		t.Fatalf("DeleteMountData() error = %v", err)
	}
	want := []string{
		"volume", "delete",
		managedVolumeName(
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "private/services",
		),
	}
	if len(runner.captureArgs) != 1 ||
		!reflect.DeepEqual(runner.captureArgs[0], want) {
		t.Fatalf("DeleteMountData() args = %#v, want %#v", runner.captureArgs, want)
	}
}

func TestDockerDeleteMountDataRemovesRuntimeVolumes(t *testing.T) {
	runner := &fakeRunner{}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	manifest := testCreateRequest("linux/amd64").Manifest
	manifest.Mounts = append(manifest.Mounts, catalog.Mount{
		Name: "private/services", Target: "/var/lib/services",
		Storage: catalog.MountStorageVolume,
	})

	err := docker.DeleteMountData(
		t.Context(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", manifest,
	)
	if err != nil {
		t.Fatalf("DeleteMountData() error = %v", err)
	}
	want := []string{
		"volume", "rm",
		managedVolumeName(
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "private/services",
		),
	}
	if len(runner.captureArgs) != 1 ||
		!reflect.DeepEqual(runner.captureArgs[0], want) {
		t.Fatalf("DeleteMountData() args = %#v, want %#v", runner.captureArgs, want)
	}
}

func TestDockerPullStreamsCommandOutput(t *testing.T) {
	runner := &fakeRunner{
		runStdout: "first layer complete\nsecond layer downloading\r",
		runStderr: "verifying image\n",
	}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	var progress []string

	err := docker.PullWithProgress(
		t.Context(),
		"pantalk/ghost:test",
		"linux/amd64",
		func(message string) {
			progress = append(progress, message)
		},
	)

	if err != nil {
		t.Fatalf("PullWithProgress() error = %v", err)
	}
	want := []string{
		"pull", "--platform", "linux/amd64", "pantalk/ghost:test",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("PullWithProgress() args = %#v, want %#v", runner.runArgs, want)
	}
	for _, expected := range []string{
		"first layer complete",
		"second layer downloading",
		"verifying image",
	} {
		if !containsString(progress, expected) {
			t.Fatalf("progress = %#v, missing %q", progress, expected)
		}
	}
}

func TestApplePullLimitsDownloadToSelectedPlatform(t *testing.T) {
	runner := &fakeRunner{}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	err := apple.PullWithProgress(
		t.Context(),
		"pantalk/ghost:test",
		"linux/arm64",
		nil,
	)

	if err != nil {
		t.Fatalf("PullWithProgress() error = %v", err)
	}
	want := []string{
		"image", "pull", "--platform", "linux/arm64", "pantalk/ghost:test",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("PullWithProgress() args = %#v, want %#v", runner.runArgs, want)
	}
}

func TestDockerResolvesAndDeletesExactImageID(t *testing.T) {
	runner := &fakeRunner{captureResults: []Result{
		{Stdout: []byte("sha256:abc123\n")},
		{},
	}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)

	image, err := docker.ResolveImage(t.Context(), "example/app@sha256:release")
	if err != nil {
		t.Fatalf("ResolveImage() error = %v", err)
	}
	if image.ID != "sha256:abc123" {
		t.Fatalf("ResolveImage() = %#v", image)
	}
	if err := docker.DeleteImage(t.Context(), image.ID); err != nil {
		t.Fatalf("DeleteImage() error = %v", err)
	}
	want := [][]string{
		{"image", "inspect", "--format", "{{.Id}}", "example/app@sha256:release"},
		{"image", "rm", "sha256:abc123"},
	}
	if !reflect.DeepEqual(runner.captureArgs, want) {
		t.Fatalf("image commands = %#v, want %#v", runner.captureArgs, want)
	}
}

func TestAppleResolvesAndDeletesExactImageID(t *testing.T) {
	runner := &fakeRunner{captureResults: []Result{
		{Stdout: []byte(`[{"id":"abc123"}]`)},
		{},
	}}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	image, err := apple.ResolveImage(t.Context(), "example/app@sha256:release")
	if err != nil {
		t.Fatalf("ResolveImage() error = %v", err)
	}
	if image.ID != "abc123" {
		t.Fatalf("ResolveImage() = %#v", image)
	}
	if err := apple.DeleteImage(t.Context(), image.ID); err != nil {
		t.Fatalf("DeleteImage() error = %v", err)
	}
	want := [][]string{
		{"image", "inspect", "example/app@sha256:release"},
		{"image", "delete", "abc123"},
	}
	if !reflect.DeepEqual(runner.captureArgs, want) {
		t.Fatalf("image commands = %#v, want %#v", runner.captureArgs, want)
	}
}

func TestAppleStatusReadsInspectJSON(t *testing.T) {
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(`[
		{
			"id":"launcher-ghost-aaaaaaaaaaaa",
			"configuration":{"labels":{"dev.pdparchitect.launcher.instance":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			"status":{"state":"running"}
		}
	]`)}}
	apple := NewApple("container", runner, io.Discard, io.Discard)
	status, err := apple.Status(t.Context(), "launcher-ghost-aaaaaaaaaaaa")
	if err != nil || status != StatusRunning {
		t.Fatalf("Status() = %q, %v", status, err)
	}
}

func TestAppleStatusReadsCurrentInspectJSON(t *testing.T) {
	runner := &fakeRunner{captureResult: Result{Stdout: []byte(`[
		{
			"id":"launcher-ghost-aaaaaaaaaaaa",
			"configuration":{"labels":{}},
			"status":"running"
		}
	]`)}}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	status, err := apple.Status(t.Context(), "launcher-ghost-aaaaaaaaaaaa")

	if err != nil || status != StatusRunning {
		t.Fatalf("Status() = %q, %v", status, err)
	}
}

func TestDockerStatsReadsLiveUsageAndStartTime(t *testing.T) {
	runner := &fakeRunner{captureResults: []Result{
		{Stdout: []byte(
			`{"CPUPerc":"0.22%","MemPerc":"0.34%","MemUsage":"161.9MiB / 47.07GiB"}`,
		)},
		{Stdout: []byte("2026-07-27T19:55:42.830571888Z\n")},
	}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)

	metrics, err := docker.Stats(t.Context(), "launcher-ghost-aaaaaaaaaaaa")

	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if metrics.CPUPercent != 0.22 || !metrics.CPUAvailable ||
		metrics.MemoryPercent != 0.34 || !metrics.MemoryAvailable {
		t.Fatalf("Stats() percentages = %#v", metrics)
	}
	if metrics.MemoryUsageBytes == 0 || metrics.MemoryLimitBytes == 0 ||
		metrics.StartedAt.IsZero() {
		t.Fatalf("Stats() details = %#v", metrics)
	}
}

func TestAppleStatsCalculatesCPUAcrossSnapshots(t *testing.T) {
	startedAt := "2026-07-27T12:00:00Z"
	inspection := []byte(
		`[{"status":{"state":"running","startedAt":"` + startedAt + `"}}]`,
	)
	runner := &fakeRunner{captureResults: []Result{
		{Stdout: []byte(
			`[{"cpuUsageUsec":1000000,"memoryUsageBytes":536870912,"memoryLimitBytes":1073741824}]`,
		)},
		{Stdout: inspection},
		{Stdout: []byte(
			`[{"cpuUsageUsec":2000000,"memoryUsageBytes":536870912,"memoryLimitBytes":1073741824}]`,
		)},
		{Stdout: inspection},
	}}
	apple := NewApple("container", runner, io.Discard, io.Discard)
	times := []time.Time{
		time.Date(2026, 7, 27, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 7, 27, 12, 1, 5, 0, time.UTC),
	}
	apple.now = func() time.Time {
		now := times[0]
		times = times[1:]
		return now
	}

	first, err := apple.Stats(t.Context(), "launcher-ghost-aaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("first Stats() error = %v", err)
	}
	second, err := apple.Stats(t.Context(), "launcher-ghost-aaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("second Stats() error = %v", err)
	}

	if first.CPUAvailable {
		t.Fatalf("first CPU metric = %#v, want initial sample", first)
	}
	if !second.CPUAvailable || second.CPUPercent != 20 ||
		!second.MemoryAvailable || second.MemoryPercent != 50 {
		t.Fatalf("second Stats() = %#v", second)
	}
	if second.StartedAt.Format(time.RFC3339) != startedAt {
		t.Fatalf("StartedAt = %v", second.StartedAt)
	}
}

func TestDockerRecentLogsReturnsBoundedOutput(t *testing.T) {
	runner := &fakeRunner{captureResult: Result{Stdout: []byte("ready\n")}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)

	logs, err := docker.RecentLogs(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		200,
	)

	if err != nil || logs != "ready\n" {
		t.Fatalf("RecentLogs() = %q, %v", logs, err)
	}
	want := []string{
		"logs", "--tail", "200", "launcher-ghost-aaaaaaaaaaaa",
	}
	if !reflect.DeepEqual(runner.captureArgs[0], want) {
		t.Fatalf("RecentLogs() args = %#v, want %#v", runner.captureArgs[0], want)
	}
}

func TestAppleRecentLogsReturnsBoundedOutput(t *testing.T) {
	runner := &fakeRunner{captureResult: Result{Stdout: []byte("ready\n")}}
	apple := NewApple("container", runner, io.Discard, io.Discard)

	logs, err := apple.RecentLogs(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		200,
	)

	if err != nil || logs != "ready\n" {
		t.Fatalf("RecentLogs() = %q, %v", logs, err)
	}
	want := []string{
		"logs", "-n", "200", "launcher-ghost-aaaaaaaaaaaa",
	}
	if !reflect.DeepEqual(runner.captureArgs[0], want) {
		t.Fatalf("RecentLogs() args = %#v, want %#v", runner.captureArgs[0], want)
	}
}

func TestRuntimeRemovalChecksOwnership(t *testing.T) {
	runner := &fakeRunner{captureResult: Result{Stdout: []byte("other\n")}}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	if err := docker.Remove(
		t.Context(),
		"launcher-ghost-aaaaaaaaaaaa",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	); err == nil {
		t.Fatal("Remove() error = nil, want ownership error")
	}
	if runner.runCalled {
		t.Fatal("Remove() deleted an unowned container")
	}
}

func TestDetectPrefersAppleContainerOnAppleSilicon(t *testing.T) {
	selection, err := Detect(DetectOptions{
		GOOS:   "darwin",
		GOARCH: "arm64",
		LookPath: func(name string) (string, error) {
			return "/usr/local/bin/" + name, nil
		},
		Runner: &fakeRunner{},
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil || selection.Name != KindApple {
		t.Fatalf("Detect() = %#v, %v", selection, err)
	}
}

func TestDetectFindsAppleContainerOutsideFinderPath(t *testing.T) {
	var searched []string
	selection, err := Detect(DetectOptions{
		GOOS:   "darwin",
		GOARCH: "arm64",
		LookPath: func(name string) (string, error) {
			searched = append(searched, name)
			if name == "/usr/local/bin/container" {
				return name, nil
			}
			return "", errExit
		},
		Runner: &fakeRunner{},
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if selection.Name != KindApple ||
		selection.Path != "/usr/local/bin/container" {
		t.Fatalf("Detect() = %#v, want Apple container installer path", selection)
	}
	if len(searched) < 2 ||
		searched[0] != "container" ||
		searched[1] != "/usr/local/bin/container" {
		t.Fatalf("searched paths = %#v", searched)
	}
}

func TestDetectDoesNotFallBackToDockerOnMacOS(t *testing.T) {
	var searched []string
	selection, err := Detect(DetectOptions{
		GOOS:   "darwin",
		GOARCH: "arm64",
		LookPath: func(name string) (string, error) {
			searched = append(searched, name)
			if name == "docker" {
				return "/usr/local/bin/docker", nil
			}
			return "", errExit
		},
		Runner: &fakeRunner{},
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil || selection.Name != KindApple {
		t.Fatalf("Detect() = %#v, %v", selection, err)
	}
	for _, candidate := range searched {
		if strings.Contains(candidate, "docker") {
			t.Fatalf("Detect() searched Docker candidate %q on macOS", candidate)
		}
	}
	_, doctorErr := selection.Runtime.Doctor(t.Context())
	var missing *MissingRuntimeError
	if !errors.As(doctorErr, &missing) || missing.kind != KindApple {
		t.Fatalf("Doctor() error = %v, want missing Apple container", doctorErr)
	}
}

func TestDetectRejectsExplicitDockerOnMacOS(t *testing.T) {
	_, err := Detect(DetectOptions{
		GOOS:      "darwin",
		GOARCH:    "arm64",
		Requested: "docker",
	})
	if err == nil || !strings.Contains(err.Error(), "requires Apple container") {
		t.Fatalf("Detect() error = %v", err)
	}
}

func TestDetectRejectsIntelMacInsteadOfSelectingDocker(t *testing.T) {
	_, err := Detect(DetectOptions{
		GOOS:   "darwin",
		GOARCH: "amd64",
	})
	if err == nil || !strings.Contains(err.Error(), "requires Apple silicon") {
		t.Fatalf("Detect() error = %v", err)
	}
}

func TestDetectReturnsInstallableMissingRuntime(t *testing.T) {
	selection, err := Detect(DetectOptions{
		GOOS:      "darwin",
		GOARCH:    "arm64",
		Requested: "container",
		LookPath: func(string) (string, error) {
			return "", errExit
		},
		Runner: &fakeRunner{},
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	_, doctorErr := selection.Runtime.Doctor(t.Context())
	var missing *MissingRuntimeError
	if !errors.As(doctorErr, &missing) {
		t.Fatalf("Doctor() error = %v, want MissingRuntimeError", doctorErr)
	}
	for _, expected := range []string{
		"not installed or is not visible",
		"/usr/local/bin/container",
		"/opt/homebrew/bin/container",
	} {
		if !strings.Contains(doctorErr.Error(), expected) {
			t.Fatalf("Doctor() error = %q, missing %q", doctorErr, expected)
		}
	}
}

func TestMissingRuntimeRecoversAfterOfficialInstallerCompletes(t *testing.T) {
	installed := false
	runner := &fakeRunner{captureResults: []Result{
		{Stdout: []byte(`{"status":"running"}`)},
		{Stdout: []byte(`[{"appName":"container","version":"1.0.0"}]`)},
	}}
	selection, err := Detect(DetectOptions{
		GOOS:      "darwin",
		GOARCH:    "arm64",
		Requested: "container",
		LookPath: func(name string) (string, error) {
			if installed && name == "/usr/local/bin/container" {
				return name, nil
			}
			return "", errExit
		},
		Runner: runner,
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if _, err := selection.Runtime.Doctor(t.Context()); err == nil {
		t.Fatal("Doctor() error = nil before installation")
	}

	installed = true
	version, err := selection.Runtime.Doctor(t.Context())

	if err != nil || version != "1.0.0" {
		t.Fatalf("Doctor() = %q, %v after installation", version, err)
	}
	recovered, ok := selection.Runtime.(*Missing)
	if !ok || recovered.RuntimePath() != "/usr/local/bin/container" {
		t.Fatalf("recovered runtime = %#v", selection.Runtime)
	}
}

func TestOSRunnerCapturesStandardError(t *testing.T) {
	result, err := (OSRunner{}).Capture(
		context.Background(),
		"sh",
		"-c",
		"printf output; printf problem >&2; exit 7",
	)
	if err == nil || !bytes.Equal(result.Stdout, []byte("output")) ||
		!bytes.Equal(result.Stderr, []byte("problem")) {
		t.Fatalf("Capture() = %#v, %v", result, err)
	}
}

func testCreateRequest(platform string) CreateRequest {
	return CreateRequest{
		InstanceID:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ContainerName: "launcher-ghost-aaaaaaaaaaaa",
		Network:       ManagedNetworkName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		Image:         "pantalk/ghost:test",
		Ports:         map[int]int{6901: 16902},
		Platform:      platform,
		Paths: map[string]string{
			"workspace": "/data/agents/a/workspace",
		},
		Manifest: catalog.Manifest{
			ID:        "370a2228-322d-4089-846b-62fb8c15d154",
			Slug:      "pantalk-ghost",
			Name:      "Pantalk Ghost",
			Publisher: "Pantalk",
			Interfaces: map[string]catalog.Interface{
				"desktop": {Kind: "kasmweb", Port: 6901, Path: "/"},
			},
			Memory:       "4g",
			SharedMemory: "1g",
			Environment:  map[string]string{"PANTALK_AUTOSTART": "true"},
			Mounts:       []catalog.Mount{{Name: "workspace", Target: "/workspace"}},
		},
	}
}

type fakeRunner struct {
	captureResult  Result
	captureResults []Result
	captureErr     error
	captureIndex   int
	captureArgs    [][]string
	runArgs        []string
	runCalled      bool
	runStdout      string
	runStderr      string
	runErr         error
	runStdin       io.Reader
}

func (runner *fakeRunner) Capture(
	_ context.Context,
	_ string,
	args ...string,
) (Result, error) {
	runner.captureArgs = append(
		runner.captureArgs,
		append([]string(nil), args...),
	)
	if runner.captureIndex < len(runner.captureResults) {
		result := runner.captureResults[runner.captureIndex]
		runner.captureIndex++
		return result, runner.captureErr
	}
	return runner.captureResult, runner.captureErr
}

func (runner *fakeRunner) Run(
	_ context.Context,
	_ string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	runner.runCalled = true
	runner.runArgs = append([]string(nil), args...)
	runner.runStdin = stdin
	_, _ = io.WriteString(stdout, runner.runStdout)
	_, _ = io.WriteString(stderr, runner.runStderr)
	return runner.runErr
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
