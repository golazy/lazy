package controllers

import (
	"context"

	"golazy.dev/lazycontroller"
)

type BaseController struct {
	lazycontroller.Base
}

func NewBaseController(ctx context.Context) (BaseController, error) {
	base, err := lazycontroller.NewBase(ctx)
	if err != nil {
		return BaseController{}, err
	}
	return BaseController{Base: base}, nil
}
