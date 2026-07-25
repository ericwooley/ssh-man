package bindings

import (
	"context"
	"errors"
	"net/http"

	"ssh-man/internal/app/bootstrap"
	appwindow "ssh-man/internal/app/window"
	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/remote"
)

type PreviewInitialState struct {
	Server     serverdomain.Server `json:"server"`
	RemotePath string              `json:"remotePath"`
}

type PreviewBindings struct {
	app        *bootstrap.Application
	remote     *remote.Service
	remotePath string
	window     *appwindow.Controller
}

func NewPreviewBindings(app *bootstrap.Application, server serverdomain.Server, remotePath string, window *appwindow.Controller) (*PreviewBindings, func(http.Handler) http.Handler) {
	if window == nil {
		window = appwindow.New()
	}
	remoteService := remote.NewService(server)
	return &PreviewBindings{
		app:        app,
		remote:     remoteService,
		remotePath: remotePath,
		window:     window,
	}, remoteService.ContentMiddleware
}

func (b *PreviewBindings) SetContext(ctx context.Context) {
	b.window.SetContext(ctx)
}

func (b *PreviewBindings) InitialState() PreviewInitialState {
	return PreviewInitialState{
		Server:     b.remote.Server(),
		RemotePath: b.remotePath,
	}
}

func (b *PreviewBindings) Connect(passphrase string) (remote.ConnectResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), explorerConnectTimeout)
	defer cancel()
	return b.remote.Connect(ctx, passphrase)
}

func (b *PreviewBindings) PreviewFile() (remote.Preview, error) {
	return b.remote.Preview(b.remotePath)
}

func (b *PreviewBindings) Shutdown(ctx context.Context) error {
	return errors.Join(b.remote.Close(), b.app.Shutdown(ctx))
}
