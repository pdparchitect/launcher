package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const maxPreviewBytes = 4 * 1024 * 1024

type previewer interface {
	capture(context.Context) ([]byte, error)
}

type previewCache struct {
	mutex     sync.Mutex
	display   string
	directory string
	path      string
	maxAge    time.Duration
}

func newPreviewCache() *previewCache {
	directory := "/tmp/launcher-desktop-preview"
	return &previewCache{
		display:   getenv("DISPLAY", ":1"),
		directory: directory,
		path:      filepath.Join(directory, "preview.jpg"),
		maxAge:    previewCacheDuration(),
	}
}

func previewCacheDuration() time.Duration {
	value := getenv("DESKTOP_PREVIEW_CACHE_SECONDS", "2")
	duration, err := time.ParseDuration(value + "s")
	if err != nil || duration < 0 {
		return 2 * time.Second
	}
	return duration
}

func (preview *previewCache) capture(ctx context.Context) ([]byte, error) {
	preview.mutex.Lock()
	defer preview.mutex.Unlock()

	if info, err := os.Stat(preview.path); err == nil &&
		info.Size() > 0 &&
		time.Since(info.ModTime()) <= preview.maxAge {
		return os.ReadFile(preview.path)
	}
	if err := os.MkdirAll(preview.directory, 0o700); err != nil {
		return nil, err
	}
	temporary, err := os.CreateTemp(preview.directory, "preview-*.jpg")
	if err != nil {
		return nil, err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(temporaryPath)

	captureContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(
		captureContext,
		"scrot",
		"--silent",
		"--overwrite",
		"--quality",
		"65",
		temporaryPath,
	)
	command.Env = append(os.Environ(), "DISPLAY="+preview.display)
	output, err := command.CombinedOutput()
	if err != nil {
		if errors.Is(captureContext.Err(), context.DeadlineExceeded) {
			return nil, errors.New("screen capture timed out")
		}
		if message := string(output); message != "" {
			return nil, fmt.Errorf("screen capture failed: %s", message)
		}
		return nil, fmt.Errorf("screen capture failed: %w", err)
	}
	info, err := os.Stat(temporaryPath)
	if err != nil {
		return nil, err
	}
	if info.Size() < 1 || info.Size() > maxPreviewBytes {
		return nil, fmt.Errorf("snapshot size %d is outside allowed range", info.Size())
	}
	if err := os.Rename(temporaryPath, preview.path); err != nil {
		return nil, err
	}
	return os.ReadFile(preview.path)
}

type bridgeHandler struct {
	preview previewer
	queue   *notificationQueue
}

func newBridgeHandler(
	preview previewer,
	queue *notificationQueue,
) http.Handler {
	return &bridgeHandler{preview: preview, queue: queue}
}

func (handler *bridgeHandler) ServeHTTP(
	response http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch request.URL.Path {
	case "/healthz":
		handler.health(response)
	case "/notifications":
		handler.notifications(response, request)
	case "/preview.jpg":
		handler.previewImage(response, request)
	default:
		http.NotFound(response, request)
	}
}

func (handler *bridgeHandler) health(response http.ResponseWriter) {
	writeJSON(response, http.StatusOK, map[string]any{
		"status": "ready",
		"components": map[string]string{
			"bridge":        "ready",
			"notifications": "ready",
		},
	})
}

func (handler *bridgeHandler) notifications(
	response http.ResponseWriter,
	request *http.Request,
) {
	page, err := handler.queue.page(request.URL.Query().Get("cursor"))
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(response, http.StatusOK, page)
}

func (handler *bridgeHandler) previewImage(
	response http.ResponseWriter,
	request *http.Request,
) {
	snapshot, err := handler.preview.capture(request.Context())
	if err != nil {
		http.Error(
			response,
			"desktop preview unavailable: "+err.Error(),
			http.StatusServiceUnavailable,
		)
		return
	}
	response.Header().Set("Content-Type", "image/jpeg")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Length", fmt.Sprintf("%d", len(snapshot)))
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(snapshot)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
