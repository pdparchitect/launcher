//go:build !darwin || !cgo

package nativehost

// Install is a no-op outside a cgo-enabled macOS build.
func Install() bool { return false }

// BadgeDockIcon is a no-op outside macOS.
func BadgeDockIcon() {}

// InstallViewerChrome is a no-op outside a cgo-enabled macOS build.
func InstallViewerChrome() bool { return false }

// ActivateProcess is a no-op outside a cgo-enabled macOS build.
func ActivateProcess(int) bool { return false }
