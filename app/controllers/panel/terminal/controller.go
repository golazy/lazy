package terminal

import (
	"context"
	"fmt"
	"net/http"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/terminalservice"
)

type TerminalController struct {
	panel.Base
	Terminal *terminalservice.Manager
}

func New(ctx context.Context) (*TerminalController, error) {
	base, err := panel.NewBase(ctx)
	if err != nil {
		return nil, err
	}
	manager, ok := terminalservice.ManagerFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("dev panel terminal manager is missing")
	}
	return &TerminalController{Base: base, Terminal: manager}, nil
}

func (c *TerminalController) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.Set("state", c.Snapshot())
	c.Set("terminal", c.Terminal.Snapshot())
	return nil
}
