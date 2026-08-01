package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
)

const (
	defaultHealthTimeout = 30 * time.Second
	healthRetryDelay     = time.Second
	healthRequestTimeout = 3 * time.Second
)

func waitForHealth(ctx context.Context, target string) error {
	checkCtx, cancel := context.WithTimeout(ctx, defaultHealthTimeout)
	defer cancel()
	client := &http.Client{
		Timeout: healthRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var lastErr error
	for {
		request, err := http.NewRequestWithContext(checkCtx, http.MethodGet, target, nil)
		if err != nil {
			return fmt.Errorf("create health request: %w", err)
		}
		response, err := client.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("health endpoint returned %s", response.Status)
		} else {
			lastErr = err
		}

		timer := time.NewTimer(healthRetryDelay)
		select {
		case <-checkCtx.Done():
			timer.Stop()
			if lastErr == nil {
				lastErr = checkCtx.Err()
			}
			return fmt.Errorf("wait for %s: %w", target, lastErr)
		case <-timer.C:
		}
	}
}

func (service *Service) checkDeclaredHealth(
	ctx context.Context,
	instance domain.Instance,
	manifest catalog.Manifest,
) error {
	ids := make([]string, 0, len(manifest.Interfaces))
	for id, definition := range manifest.Interfaces {
		if definition.Kind == "health" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		resolved, exists := instance.Interfaces[id]
		if !exists {
			return fmt.Errorf("health interface %q is unresolved", id)
		}
		if err := service.options.HealthCheck(ctx, resolved.URL()); err != nil {
			return fmt.Errorf("health interface %q is not ready: %w", id, err)
		}
	}
	return nil
}

func joinedUpdateError(action string, cause error, recovery ...error) error {
	errorsToJoin := []error{fmt.Errorf("%s: %w", action, cause)}
	for _, err := range recovery {
		if err != nil {
			errorsToJoin = append(errorsToJoin, err)
		}
	}
	return errors.Join(errorsToJoin...)
}
