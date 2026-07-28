package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Objective-C only warns when a method is declared but never defined, so the
// build succeeds and the app dies at runtime with "unrecognized selector" the
// first time that code path runs. Editing these files by patch makes it easy
// to delete a definition while leaving its declaration behind, so every
// declaration in a class extension is checked for a matching definition.
func TestWailsPatchesDefineEveryMethodTheyDeclare(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate patch symbol test")
	}
	dir := filepath.Join(filepath.Dir(filename), "..", "..", "patches", "wails")
	entries, err := filepath.Glob(filepath.Join(dir, "*.patch"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no Wails patches found: %v", err)
	}

	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Base(entry), err)
		}
		// Added lines only, with the diff marker stripped.
		var added strings.Builder
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				added.WriteString(strings.TrimPrefix(line, "+"))
				added.WriteByte('\n')
			}
		}
		source := added.String()

		start := strings.Index(source, "@interface WailsSidebar ()")
		if start < 0 {
			continue
		}
		block := source[start:]
		block = block[:strings.Index(block, "@end")]

		for _, line := range strings.Split(block, "\n") {
			decl := strings.TrimSpace(line)
			if !strings.HasPrefix(decl, "- ") && !strings.HasPrefix(decl, "+ ") {
				continue
			}
			if !strings.HasSuffix(decl, ";") {
				continue
			}
			definition := strings.TrimSuffix(decl, ";") + " {"
			if !strings.Contains(source, definition) {
				t.Fatalf(
					"%s declares %q but never defines it — this crashes at runtime",
					filepath.Base(entry), decl,
				)
			}
		}
	}
}
