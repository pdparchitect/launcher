package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

const maxPreviewSize = 32 << 20

type previewDownloadOptions struct {
	client     *http.Client
	attempts   int
	retryDelay time.Duration
	maxBytes   int64
}

func defaultPreviewDownloadOptions() previewDownloadOptions {
	return previewDownloadOptions{
		client:     &http.Client{Timeout: 15 * time.Second},
		attempts:   20,
		retryDelay: time.Second,
		maxBytes:   maxPreviewSize,
	}
}

func (app *App) savePreview(ctx context.Context, args []string) error {
	flags := app.flags("preview", "Save an agent's current preview image.")
	output := flags.String("output", "", "destination image path")
	force := flags.Bool("force", false, "replace an existing destination")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 ||
		strings.TrimSpace(flags.Arg(0)) == "" ||
		strings.TrimSpace(*output) == "" {
		return errors.New(
			"usage: launcher preview --output PATH [--force] NAME",
		)
	}
	destination := filepath.Clean(strings.TrimSpace(*output))
	if err := validatePreviewDestination(destination, *force); err != nil {
		return err
	}

	view, err := app.service.Get(ctx, flags.Arg(0))
	if err != nil {
		return err
	}
	if view.State != launchruntime.StatusRunning {
		return fmt.Errorf("%s must be running to capture its preview", view.Name)
	}
	var previewURL string
	for _, resolved := range view.Interfaces {
		if resolved.Kind != "preview" {
			continue
		}
		if previewURL != "" {
			return fmt.Errorf("%s exposes multiple preview interfaces", view.Name)
		}
		previewURL = resolved.URL()
	}
	if previewURL == "" {
		return fmt.Errorf("%s does not expose a preview interface", view.Name)
	}

	response, err := app.fetchPreview(ctx, previewURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := writePreviewAtomically(
		destination,
		response.Body,
		*force,
		app.preview.maxBytes,
	); err != nil {
		return fmt.Errorf("save preview: %w", err)
	}
	fmt.Fprintf(app.stdout, "Saved preview for %s to %s\n", view.Name, destination)
	return nil
}

func validatePreviewDestination(destination string, force bool) error {
	info, err := os.Lstat(destination)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect preview destination: %w", err)
	}
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("preview destination %q is a directory", destination)
		}
		if !force {
			return fmt.Errorf(
				"preview destination %q already exists; use --force to replace it",
				destination,
			)
		}
	}
	parent := filepath.Dir(destination)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("inspect preview destination directory: %w", err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("preview destination directory %q is not a directory", parent)
	}
	return nil
}

func (app *App) fetchPreview(
	ctx context.Context,
	previewURL string,
) (*http.Response, error) {
	attempts := app.preview.attempts
	if attempts < 1 {
		attempts = 1
	}
	client := app.preview.client
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			previewURL,
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("create preview request: %w", err)
		}
		response, requestErr := client.Do(request)
		if requestErr == nil && response.StatusCode != http.StatusServiceUnavailable {
			if response.StatusCode != http.StatusOK {
				_ = response.Body.Close()
				return nil, fmt.Errorf(
					"download preview: server returned %s",
					response.Status,
				)
			}
			return response, nil
		}
		if response != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			_ = response.Body.Close()
			lastErr = fmt.Errorf("server returned %s", response.Status)
		} else {
			lastErr = requestErr
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt < attempts {
			if err := waitForPreviewRetry(ctx, app.preview.retryDelay); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf(
		"download preview after %d attempts: %w",
		attempts,
		lastErr,
	)
}

func waitForPreviewRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writePreviewAtomically(
	destination string,
	source io.Reader,
	force bool,
	maxBytes int64,
) (resultErr error) {
	if maxBytes < 1 {
		maxBytes = maxPreviewSize
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".launcher-preview-*")
	if err != nil {
		return fmt.Errorf("create temporary image: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()

	buffered := bufio.NewReader(source)
	header, peekErr := buffered.Peek(512)
	if peekErr != nil && !errors.Is(peekErr, io.EOF) && !errors.Is(peekErr, bufio.ErrBufferFull) {
		return fmt.Errorf("read image header: %w", peekErr)
	}
	mediaType := http.DetectContentType(header)
	if !strings.HasPrefix(mediaType, "image/") {
		return fmt.Errorf("preview returned %s instead of an image", mediaType)
	}
	written, err := io.Copy(temporary, io.LimitReader(buffered, maxBytes+1))
	if err != nil {
		return fmt.Errorf("write temporary image: %w", err)
	}
	if written > maxBytes {
		return fmt.Errorf("preview exceeds the %d-byte size limit", maxBytes)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary image: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary image: %w", err)
	}
	if force {
		if err := os.Rename(temporaryPath, destination); err != nil {
			return fmt.Errorf("replace destination: %w", err)
		}
		return nil
	}
	if err := os.Link(temporaryPath, destination); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf(
				"destination %q appeared while saving; use --force to replace it",
				destination,
			)
		}
		return fmt.Errorf("publish destination: %w", err)
	}
	return nil
}
