//go:build !desktop

package desktop

import (
	"context"
	"errors"

	"github.com/pdparchitect/launcher/internal/agent"
	"github.com/pdparchitect/launcher/internal/httpapi"
)

func Available() bool {
	return false
}

func run(context.Context, httpapi.Service, Options) error {
	return errors.New("desktop interface is not available in this build")
}

func runViewer(context.Context, agent.View, string) error {
	return errors.New("desktop agent viewer is not available in this build")
}
