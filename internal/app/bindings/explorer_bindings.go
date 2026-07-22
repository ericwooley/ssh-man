package bindings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/remote"
)

const explorerConnectTimeout = 15 * time.Second

type ExplorerInitialState struct {
	Server serverdomain.Server `json:"server"`
}

type ExplorerBindings struct {
	app     *bootstrap.Application
	remote  *remote.Service
	window  *appwindow.Controller
	context context.Context
}

func NewExplorerBindings(app *bootstrap.Application, server serverdomain.Server, window *appwindow.Controller) (*ExplorerBindings, func(http.Handler) http.Handler) {
	if window == nil {
		window = appwindow.New()
	}
	remoteService := remote.NewService(server)
	return &ExplorerBindings{app: app, remote: remoteService, window: window}, remoteService.ContentMiddleware
}

func (b *ExplorerBindings) SetContext(ctx context.Context) {
	b.context = ctx
	b.window.SetContext(ctx)
}

func (b *ExplorerBindings) InitialState() ExplorerInitialState {
	return ExplorerInitialState{Server: b.remote.Server()}
}

func (b *ExplorerBindings) Connect(passphrase string) (remote.ConnectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), explorerConnectTimeout)
	defer cancel()
	return b.remote.Connect(ctx, passphrase)
}

func (b *ExplorerBindings) ListDirectory(remotePath string) (remote.Directory, error) {
	return b.remote.List(remotePath)
}

func (b *ExplorerBindings) PreviewFile(remotePath string) (remote.Preview, error) {
	return b.remote.Preview(remotePath)
}

func (b *ExplorerBindings) SaveFile(remotePath, content, expectedRevision string) (remote.Preview, error) {
	return b.remote.Save(remotePath, content, expectedRevision)
}

func (b *ExplorerBindings) Download(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return []string{}, nil
	}
	if b.context == nil {
		return nil, fmt.Errorf("explorer window is not ready")
	}
	destination, err := runtime.OpenDirectoryDialog(b.context, runtime.OpenDialogOptions{
		Title: "Choose where to download the selected remote items",
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(destination) == "" {
		return []string{}, nil
	}
	return b.remote.Download(context.Background(), paths, destination)
}

func (b *ExplorerBindings) Close() error {
	return b.window.Quit()
}

func (b *ExplorerBindings) Shutdown(ctx context.Context) error {
	return errors.Join(b.remote.Close(), b.app.Shutdown(ctx))
}
