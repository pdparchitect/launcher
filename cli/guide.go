package cli

import (
	_ "embed"
	"errors"
	"fmt"
)

// agentGuide is kept as a source document so command changes can update the
// embedded instructions in the same release.
//
//go:embed agent-guide.md
var agentGuide string

func (app *App) guide(args []string) error {
	if len(args) != 0 {
		return errors.New("guide does not accept arguments")
	}
	_, err := fmt.Fprint(app.stdout, agentGuide)
	return err
}
