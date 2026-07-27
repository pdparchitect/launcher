package desktop

import (
	"context"
	"io"

	"github.com/pdparchitect/launcher/internal/httpapi"
)

type Options struct {
	Stdout io.Writer
}

func Run(
	ctx context.Context,
	service httpapi.Service,
	options Options,
) error {
	return run(ctx, service, options)
}
