package agentskill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	launchercli "github.com/pdparchitect/launcher/cli"
)

const testGuide = `# Launcher agent guide

Run commands with Launcher.

` + "```sh" + `
launcher list
launcher exec NAME uname -a
` + "```" + `
`

func TestInstallSkipsUsersWithoutSupportedAgentTools(t *testing.T) {
	home := t.TempDir()

	err := Install(home, "/Applications/Launcher.app/launcher", testGuide)

	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, directory := range []string{".codex", ".claude"} {
		if _, err := os.Stat(filepath.Join(home, directory)); !os.IsNotExist(err) {
			t.Fatalf("%s directory error = %v", directory, err)
		}
	}
}

func TestInstallCreatesTheSameSkillForCodexAndClaudeCode(t *testing.T) {
	home := t.TempDir()
	for _, directory := range []string{".codex", ".claude"} {
		if err := os.Mkdir(filepath.Join(home, directory), 0o700); err != nil {
			t.Fatalf("Mkdir(%s) error = %v", directory, err)
		}
	}
	executable := "/Applications/Launcher App.app/Contents/MacOS/launcher"

	if err := Install(home, executable, testGuide); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	var installedContent string
	for _, directory := range []string{".codex", ".claude"} {
		target := skillPath(home, directory)
		content, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", target, err)
		}
		text := string(content)
		for _, expected := range []string{
			"---\nname: pdparchitect-launcher\n",
			"description: Manage local Launcher application containers",
			"Use when an agent needs to discover",
			"/Applications/Launcher App.app/Contents/MacOS/launcher",
			"'/Applications/Launcher App.app/Contents/MacOS/launcher' list",
			"'/Applications/Launcher App.app/Contents/MacOS/launcher' exec NAME uname -a",
		} {
			if !strings.Contains(text, expected) {
				t.Fatalf("%s skill missing %q:\n%s", directory, expected, text)
			}
		}
		if installedContent != "" && text != installedContent {
			t.Fatalf("%s skill differs from the other installation", directory)
		}
		installedContent = text
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", target, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s skill mode = %v", directory, info.Mode().Perm())
		}
	}
}

func TestInstallDoesNotCreateMissingAgentToolDirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	if err := Install(home, "/opt/launcher", testGuide); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := os.Stat(skillPath(home, ".claude")); err != nil {
		t.Fatalf("Claude Code skill error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("Codex directory error = %v", err)
	}
}

func TestInstallContinuesAfterOneAgentToolFails(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".codex"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	err := Install(home, "/opt/launcher", testGuide)

	if err == nil || !strings.Contains(err.Error(), "install Codex skill") {
		t.Fatalf("Install() error = %v", err)
	}
	if _, statErr := os.Stat(skillPath(home, ".claude")); statErr != nil {
		t.Fatalf("Claude Code skill error = %v", statErr)
	}
}

func TestInstallUpdatesChangedGuideAndLeavesCurrentSkillUntouched(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	executable := "/opt/launcher"
	if err := Install(home, executable, testGuide); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	target := skillPath(home, ".codex")
	first, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if err := Install(home, executable, testGuide); err != nil {
		t.Fatalf("second Install() error = %v", err)
	}
	second, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Fatalf("unchanged skill mtime = %v, want %v", second.ModTime(), first.ModTime())
	}
	updatedGuide := testGuide + "\nlauncher status NAME\n"
	if err := Install(home, executable, updatedGuide); err != nil {
		t.Fatalf("updated Install() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil || !strings.Contains(string(content), "'/opt/launcher' status NAME") {
		t.Fatalf("updated skill = %q, %v", content, err)
	}
}

func TestInstallRejectsRelativeExecutable(t *testing.T) {
	err := Install(t.TempDir(), "launcher", testGuide)

	if err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestRenderRewritesBuiltInGuideCommandsWithoutSelfReference(t *testing.T) {
	content := render(
		"/Applications/Launcher.app/Contents/MacOS/launcher",
		launchercli.AgentGuide(),
	)

	if strings.Contains(content, "\nlauncher ") {
		t.Fatalf("rendered skill retained a PATH-based command:\n%s", content)
	}
	if strings.Contains(content, ".codex/skills") ||
		strings.Contains(content, ".claude/skills") {
		t.Fatalf("rendered guide references its installed skill:\n%s", content)
	}
	for _, expected := range []string{
		"'/Applications/Launcher.app/Contents/MacOS/launcher' create --app SLUG_OR_ID NAME",
		"'/Applications/Launcher.app/Contents/MacOS/launcher' status NAME",
		"'/Applications/Launcher.app/Contents/MacOS/launcher' guide",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("rendered skill missing %q", expected)
		}
	}
}

func skillPath(home string, agentDirectory string) string {
	return filepath.Join(
		home,
		agentDirectory,
		"skills",
		"pdparchitect-launcher",
		"SKILL.md",
	)
}
