package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/pdparchitect/launcher/internal/catalog"
	"github.com/pdparchitect/launcher/internal/domain"
	launchruntime "github.com/pdparchitect/launcher/internal/runtime"
)

type copiedDirectory struct {
	path    string
	mode    fs.FileMode
	modTime time.Time
}

func (service *Service) Duplicate(
	ctx context.Context,
	reference string,
	options DuplicateOptions,
) (duplicate domain.Instance, resultErr error) {
	source, err := service.store.Get(reference)
	if err != nil {
		return domain.Instance{}, err
	}
	manifest, exists := service.manifest(source.CatalogID)
	if !exists {
		return domain.Instance{}, fmt.Errorf(
			"catalogue entry %q is not built in",
			source.CatalogID,
		)
	}
	for _, mount := range manifest.Mounts {
		if mount.Storage == catalog.MountStorageVolume {
			return domain.Instance{}, fmt.Errorf(
				"duplicate agent: runtime-managed mount %q is not supported",
				mount.Name,
			)
		}
	}

	status, err := service.runtime.Status(ctx, source.ContainerName)
	if err != nil {
		return domain.Instance{}, fmt.Errorf(
			"inspect source agent before duplication: %w",
			err,
		)
	}

	duplicate, err = service.Create(ctx, CreateOptions{
		CatalogID: source.CatalogID,
		Name:      options.Name,
		Image:     source.Image,
		Start:     false,
	})
	if err != nil {
		return domain.Instance{}, fmt.Errorf("prepare duplicate agent: %w", err)
	}
	duplicateID := duplicate.ID
	cleanup := true
	defer func() {
		if !cleanup {
			return
		}
		if err := service.Delete(
			context.WithoutCancel(ctx),
			duplicateID,
		); err != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("remove incomplete duplicate: %w", err),
			)
		}
	}()

	active := status == launchruntime.StatusRunning ||
		status == launchruntime.StatusRestarting
	sourceStopped := false
	restartSource := func() error {
		if !sourceStopped {
			return nil
		}
		if err := service.runtime.Start(
			context.WithoutCancel(ctx),
			source.ContainerName,
		); err != nil {
			return err
		}
		sourceStopped = false
		return nil
	}
	if active {
		if err := service.runtime.Stop(ctx, source.ContainerName); err != nil {
			return domain.Instance{}, fmt.Errorf(
				"stop source agent for duplication: %w",
				err,
			)
		}
		sourceStopped = true
		defer func() {
			if err := restartSource(); err != nil {
				resultErr = errors.Join(
					resultErr,
					fmt.Errorf("restart source agent after duplication: %w", err),
				)
			}
		}()
	}

	sourcePaths := service.store.Paths(source.ID, manifest)
	duplicatePaths := service.store.Paths(duplicate.ID, manifest)
	for _, mount := range manifest.Mounts {
		sourcePath, sourceExists := sourcePaths.Mounts[mount.Name]
		duplicatePath, duplicateExists := duplicatePaths.Mounts[mount.Name]
		if !sourceExists || !duplicateExists {
			return domain.Instance{}, fmt.Errorf(
				"duplicate mount %q: managed path is missing",
				mount.Name,
			)
		}
		if err := service.options.CopyFiles(sourcePath, duplicatePath); err != nil {
			return domain.Instance{}, fmt.Errorf(
				"duplicate mount %q: %w",
				mount.Name,
				err,
			)
		}
	}
	if err := restartSource(); err != nil {
		return domain.Instance{}, fmt.Errorf(
			"restart source agent after duplication: %w",
			err,
		)
	}

	cleanup = false
	if !options.Start {
		return duplicate, nil
	}
	started, err := service.Start(ctx, duplicate.ID)
	if err != nil {
		return duplicate, fmt.Errorf("start duplicate agent: %w", err)
	}
	return started, nil
}

func copyDirectory(source string, destination string) error {
	directories := make([]copiedDirectory, 0)
	err := filepath.WalkDir(source, func(
		path string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}

		switch {
		case entry.IsDir():
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			directories = append(directories, copiedDirectory{
				path: target, mode: info.Mode().Perm(),
				modTime: info.ModTime(),
			})
			return nil
		case entry.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := removeCopyTarget(target); err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyRegularFile(path, target, info)
		default:
			return fmt.Errorf("unsupported file type %s", info.Mode().Type())
		}
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		directory := directories[index]
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return err
		}
		if err := os.Chtimes(
			directory.path,
			directory.modTime,
			directory.modTime,
		); err != nil {
			return err
		}
	}
	return nil
}

func copyRegularFile(source string, target string, info fs.FileInfo) error {
	if err := removeCopyTarget(target); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = input.Close()
		return err
	}
	_, copyErr := io.Copy(output, input)
	inputErr := input.Close()
	outputErr := output.Close()
	if err := errors.Join(copyErr, inputErr, outputErr); err != nil {
		return err
	}
	if err := os.Chmod(target, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(target, info.ModTime(), info.ModTime())
}

func removeCopyTarget(target string) error {
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("destination %q is a directory", target)
	}
	return os.Remove(target)
}
