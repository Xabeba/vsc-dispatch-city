package api

import (
	"context"

	"github.com/teko/food-delivery/internal/model"
	"github.com/teko/food-delivery/internal/simulation"
)

// Commands controls the simulation without coupling HTTP to its transport.
type Commands interface {
	Start(context.Context) error
	Pause(context.Context) error
	Reset(context.Context) error
	CreateOrder(context.Context) (model.Order, error)
}

// LocalCommands directly controls the standalone simulation.
type LocalCommands struct {
	engine *simulation.Engine
}

// NewLocalCommands returns standalone command handlers.
func NewLocalCommands(engine *simulation.Engine) *LocalCommands {
	return &LocalCommands{engine: engine}
}

func (c *LocalCommands) Start(context.Context) error {
	c.engine.SetRunning(true)
	return nil
}

func (c *LocalCommands) Pause(context.Context) error {
	c.engine.SetRunning(false)
	return nil
}

func (c *LocalCommands) Reset(context.Context) error {
	c.engine.Reset()
	return nil
}

func (c *LocalCommands) CreateOrder(context.Context) (model.Order, error) {
	return c.engine.CreateOrder(), nil
}
