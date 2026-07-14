package window

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrNotReady           = errors.New("the application window is not ready yet")
	ErrRuntimeUnavailable = errors.New("the application window runtime is unavailable")
)

// Runtime contains the side-effecting Wails window operations used by Controller.
// Keeping this boundary explicit makes the lifecycle decisions independently testable.
type Runtime interface {
	Hide(context.Context)
	Show(context.Context)
	Quit(context.Context)
}

type Controller struct {
	mu             sync.RWMutex
	ctx            context.Context
	runtime        Runtime
	showPending    bool
	quitRequested  bool
	quitDispatched bool
}

func New() *Controller {
	return NewWithRuntime(newWailsRuntime())
}

func NewWithRuntime(runtime Runtime) *Controller {
	return &Controller{runtime: runtime}
}

func (c *Controller) SetContext(ctx context.Context) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.ctx = ctx
	quitPending := ctx != nil && c.quitRequested && !c.quitDispatched
	if quitPending {
		c.quitDispatched = true
		c.showPending = false
	}
	showPending := ctx != nil && c.showPending && !c.quitRequested
	if showPending {
		c.showPending = false
	}
	c.mu.Unlock()

	if quitPending {
		c.runtime.Quit(ctx)
	} else if showPending {
		c.runtime.Show(ctx)
	}
}

func (c *Controller) Context() (context.Context, error) {
	if c == nil {
		return nil, ErrNotReady
	}

	c.mu.RLock()
	ctx := c.ctx
	c.mu.RUnlock()
	if ctx == nil {
		return nil, ErrNotReady
	}
	return ctx, nil
}

func (c *Controller) Hide() error {
	ctx, err := c.Context()
	if err != nil {
		return err
	}
	c.runtime.Hide(ctx)
	return nil
}

func (c *Controller) Show() error {
	ctx, err := c.Context()
	if err != nil {
		return err
	}
	c.runtime.Show(ctx)
	return nil
}

// ShowWhenReady shows immediately once Wails has supplied its lifecycle
// context, or remembers the request when a second app launch arrives early.
func (c *Controller) ShowWhenReady() {
	if c == nil {
		return
	}

	c.mu.Lock()
	ctx := c.ctx
	if c.quitRequested {
		c.showPending = false
		c.mu.Unlock()
		return
	}
	if ctx == nil {
		c.showPending = true
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	c.runtime.Show(ctx)
}

func (c *Controller) Quit() error {
	if c == nil {
		return ErrNotReady
	}

	c.mu.Lock()
	if c.runtime == nil {
		c.mu.Unlock()
		return ErrRuntimeUnavailable
	}
	c.quitRequested = true
	ctx := c.ctx
	if ctx == nil || c.quitDispatched {
		c.mu.Unlock()
		return nil
	}
	c.quitDispatched = true
	c.showPending = false
	c.mu.Unlock()

	c.runtime.Quit(ctx)
	return nil
}
