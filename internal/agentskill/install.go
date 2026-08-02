package agentskill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	skillName = "pdparchitect-launcher"
	skillFile = "SKILL.md"
)

type target struct {
	name      string
	directory string
}

var targets = []target{
	{name: "Codex", directory: ".codex"},
	{name: "Claude Code", directory: ".claude"},
}

// Install writes Launcher's built-in guide for each supported agent tool that
// already has a home directory. Missing agent tools are left untouched.
func Install(home string, executable string, guide string) error {
	home = strings.TrimSpace(home)
	if home == "" {
		return errors.New("home directory is required")
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return errors.New("launcher executable path is required")
	}
	if !filepath.IsAbs(executable) {
		return errors.New("launcher executable path must be absolute")
	}
	if strings.TrimSpace(guide) == "" {
		return errors.New("launcher agent guide is empty")
	}

	content := []byte(render(executable, guide))
	var installErrors []error
	for _, target := range targets {
		if err := installTarget(home, target, content); err != nil {
			installErrors = append(installErrors, fmt.Errorf(
				"install %s skill: %w",
				target.name,
				err,
			))
		}
	}
	return errors.Join(installErrors...)
}

func installTarget(home string, target target, content []byte) error {
	agentRoot := filepath.Join(home, target.directory)
	info, err := os.Stat(agentRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s directory: %w", target.name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", agentRoot)
	}

	directory := filepath.Join(agentRoot, "skills", skillName)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}
	targetFile := filepath.Join(directory, skillFile)
	current, readErr := os.ReadFile(targetFile)
	if readErr == nil && string(current) == string(content) {
		return nil
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read existing skill: %w", readErr)
	}
	return writeAtomic(targetFile, content)
}

func render(executable string, guide string) string {
	command := shellQuote(executable)
	guide = strings.ReplaceAll(guide, "`launcher", "`"+command)
	lines := strings.Split(guide, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "launcher ") {
			lines[index] = command + strings.TrimPrefix(line, "launcher")
		}
	}
	guide = strings.Join(lines, "\n")
	guide = strings.TrimSpace(guide)
	guide = strings.TrimPrefix(guide, "# Launcher agent guide")
	guide = strings.TrimSpace(guide)
	return fmt.Sprintf(`---
name: pdparchitect-launcher
description: Manage local Launcher application containers and their persistent data. Use when an agent needs to discover, create, inspect, start, stop, open, duplicate, update, troubleshoot, execute commands in, preview, or delete an application managed by Launcher.
---

# Launcher

Always use the exact Launcher executable installed with the application:

%s

Do not substitute a different `+"`launcher`"+` executable from `+"`PATH`"+`. The commands
below already use this installation-specific path.

%s
`, fencedText(executable), guide)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func fencedText(value string) string {
	return "```text\n" + value + "\n```"
}

func writeAtomic(target string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(target), ".SKILL.md-*")
	if err != nil {
		return fmt.Errorf("create temporary skill: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary skill: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary skill: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary skill: %w", err)
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return fmt.Errorf("replace skill: %w", err)
	}
	return nil
}
