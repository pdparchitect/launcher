//go:build !darwin || !cgo

package nativehost

// Install is a no-op outside a cgo-enabled macOS build.
func Install() bool { return false }

// BadgeDockIcon is a no-op outside macOS.
func BadgeDockIcon() {}

// InstallViewerChrome is a no-op outside a cgo-enabled macOS build.
func InstallViewerChrome() bool { return false }

// InstallArrowKeyFix is a no-op outside a cgo-enabled macOS build. The
// numeric-pad flag it compensates for is a macOS keyboard quirk.
func InstallArrowKeyFix() bool { return false }

// ActivateProcess is a no-op outside a cgo-enabled macOS build.
func ActivateProcess(int) bool { return false }
