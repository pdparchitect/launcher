package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	DefaultImageCleanupInterval = 24 * time.Hour
	DefaultImageRetention       = 7 * 24 * time.Hour
)

type ImageCleanupReport struct {
	Tracked   int
	Protected int
	Deferred  int
	Removed   int
}

func (service *Service) trackImage(ctx context.Context, reference string) error {
	image, err := service.runtime.ResolveImage(ctx, reference)
	if err != nil {
		return fmt.Errorf("resolve image %q: %w", reference, err)
	}
	if err := service.imageCache.Record(
		image.ID,
		reference,
		service.options.Now().UTC(),
	); err != nil {
		return fmt.Errorf("record image %q: %w", reference, err)
	}
	return nil
}

func (service *Service) CleanupImages(
	ctx context.Context,
	minimumAge time.Duration,
) (ImageCleanupReport, error) {
	if minimumAge < 0 {
		return ImageCleanupReport{}, errors.New("image retention cannot be negative")
	}
	service.cleanupMutex.Lock()
	defer service.cleanupMutex.Unlock()
	if err := ctx.Err(); err != nil {
		return ImageCleanupReport{}, err
	}

	instances, err := service.store.List()
	if err != nil {
		return ImageCleanupReport{}, err
	}
	now := service.options.Now().UTC()
	protected := make(map[string]struct{}, len(instances))
	references := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		references[instance.Image] = struct{}{}
	}
	orderedReferences := make([]string, 0, len(references))
	for reference := range references {
		orderedReferences = append(orderedReferences, reference)
	}
	sort.Strings(orderedReferences)
	for _, reference := range orderedReferences {
		image, resolveErr := service.runtime.ResolveImage(ctx, reference)
		if resolveErr != nil {
			return ImageCleanupReport{}, fmt.Errorf(
				"resolve active image %q: %w",
				reference,
				resolveErr,
			)
		}
		protected[image.ID] = struct{}{}
		if recordErr := service.imageCache.Record(image.ID, reference, now); recordErr != nil {
			return ImageCleanupReport{}, fmt.Errorf(
				"record active image %q: %w",
				reference,
				recordErr,
			)
		}
	}

	entries, err := service.imageCache.Entries()
	if err != nil {
		return ImageCleanupReport{}, err
	}
	report := ImageCleanupReport{Tracked: len(entries)}
	cutoff := now.Add(-minimumAge)
	var cleanupErrors []error
	for _, entry := range entries {
		if _, exists := protected[entry.ID]; exists {
			report.Protected++
			continue
		}
		if entry.LastUsedAt.After(cutoff) {
			report.Deferred++
			continue
		}
		if err := service.runtime.DeleteImage(ctx, entry.ID); err != nil {
			cleanupErrors = append(
				cleanupErrors,
				fmt.Errorf("delete cached image %q: %w", entry.ID, err),
			)
			continue
		}
		if err := service.imageCache.Forget(entry.ID); err != nil {
			cleanupErrors = append(
				cleanupErrors,
				fmt.Errorf("forget deleted image %q: %w", entry.ID, err),
			)
			continue
		}
		report.Removed++
	}
	return report, errors.Join(cleanupErrors...)
}
