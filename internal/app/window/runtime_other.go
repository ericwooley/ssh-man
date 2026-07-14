//go:build !darwin

package window

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type wailsRuntime struct{}

func newWailsRuntime() Runtime {
	return wailsRuntime{}
}

func (wailsRuntime) Hide(ctx context.Context) {
	wailsruntime.WindowHide(ctx)
}

func (wailsRuntime) Show(ctx context.Context) {
	wailsruntime.WindowShow(ctx)
}

func (wailsRuntime) Quit(ctx context.Context) {
	wailsruntime.Quit(ctx)
}
