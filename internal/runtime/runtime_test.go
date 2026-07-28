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
		"--shm-size", "1g",
		"--publish", "127.0.0.1:16902:6901",
		"--env", "GHOST_RESOLUTION=1600x900",
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
		"--memory", "4g",
		"--shm-size", "1g",
		"--publish", "127.0.0.1:16902:6901",
		"--env", "GHOST_RESOLUTION=1600x900",
		"--env", "PANTALK_AUTOSTART=true",
		"--volume", "/data/agents/a/workspace:/workspace",
		"pantalk/ghost:test",
	}
	if !reflect.DeepEqual(runner.runArgs, want) {
		t.Fatalf("Create() args = %#v\nwant %#v", runner.runArgs, want)
	}
}

func TestDockerPullStreamsCommandOutput(t *testing.T) {
	runner := &fakeRunner{
		captureErr: errExit,
		runStdout:  "first layer complete\nsecond layer downloading\r",
		runStderr:  "verifying image\n",
	}
	docker := NewDocker("docker", runner, io.Discard, io.Discard)
	var progress []string

	err := docker.PullWithProgress(
		t.Context(),
		"pantalk/ghost:test",
		func(message string) {
			progress = append(progress, message)
		},
	)

	if err != nil {
		t.Fatalf("PullWithProgress() error = %v", err)
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

func TestDetectFallsBackToDocker(t *testing.T) {
	selection, err := Detect(DetectOptions{
		GOOS:   "darwin",
		GOARCH: "arm64",
		LookPath: func(name string) (string, error) {
			if name == "docker" {
				return "/usr/local/bin/docker", nil
			}
			return "", errExit
		},
		Runner: &fakeRunner{},
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil || selection.Name != KindDocker {
		t.Fatalf("Detect() = %#v, %v", selection, err)
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
		Image:         "pantalk/ghost:test",
		Port:          16902,
		Platform:      platform,
		Paths: map[string]string{
			"workspace": "/data/agents/a/workspace",
		},
		Manifest: catalog.Manifest{
			ID:                    "370a2228-322d-4089-846b-62fb8c15d154",
			Slug:                  "pantalk-ghost",
			Name:                  "Pantalk Ghost",
			Publisher:             "Pantalk",
			ContainerPort:         6901,
			Memory:                "4g",
			SharedMemory:          "1g",
			ResolutionEnvironment: "GHOST_RESOLUTION",
			Resolution:            "1600x900",
			Environment:           map[string]string{"PANTALK_AUTOSTART": "true"},
			Mounts:                []catalog.Mount{{Name: "workspace", Target: "/workspace"}},
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
	_ io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	runner.runCalled = true
	runner.runArgs = append([]string(nil), args...)
	_, _ = io.WriteString(stdout, runner.runStdout)
	_, _ = io.WriteString(stderr, runner.runStderr)
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
