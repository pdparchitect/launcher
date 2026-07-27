package cli

import "testing"

func TestSystemOpenerTreatsMissingDesktopCommandAsFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := (SystemOpener{}).Open("http://127.0.0.1:16902"); err != nil {
		t.Fatalf("Open() error = %v, want printed-link fallback", err)
	}
}
