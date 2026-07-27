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
