//go:build !desktop

package desktop

import (
	"context"
	"errors"

	"github.com/pdparchitect/launcher/internal/httpapi"
)

func Available() bool {
	return false
}

func run(context.Context, httpapi.Service, Options) error {
	return errors.New("desktop interface is not available in this build")
}

func runViewer(context.Context, string, string, string) error {
	return errors.New("desktop agent viewer is not available in this build")
}

// focusViewer cannot raise a window a build without a desktop never opened.
func focusViewer(int) bool {
	return false
}
