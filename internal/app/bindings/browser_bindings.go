package bindings

import (
	"context"
	"log"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *AppBindings) DiscoverBrowsers() (any, error) {
	return a.app.BrowserService.Discover(context.Background())
}

func (a *AppBindings) ChooseBrowserApplication() (string, error) {
	ctx, err := a.window.Context()
	if err != nil {
		return "", err
	}
	path, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Choose a browser application",
		Filters: []runtime.FileFilter{
			{DisplayName: "Browser applications", Pattern: "*.app"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}

func (a *AppBindings) PreviewBrowserLaunchThroughSocks(configurationID string, browserID string) (any, error) {
	return a.app.BrowserService.PreviewLaunchThroughSOCKS(context.Background(), configurationID, browserID)
}

func (a *AppBindings) LaunchBrowserThroughSocks(configurationID string, browserID string) error {
	return a.app.BrowserService.LaunchThroughSOCKS(context.Background(), configurationID, browserID)
}

func (a *AppBindings) ListRunningBrowsers() (any, error) {
	return a.app.BrowserService.ListRunning(context.Background())
}

func (a *AppBindings) ActivateRunningBrowser(targetID string) error {
	return a.app.BrowserService.ActivateRunning(context.Background(), targetID)
}

func (a *AppBindings) DefaultBrowserStatus() (any, error) {
	return a.app.DefaultBrowser.Status()
}

func (a *AppBindings) SetAsDefaultBrowser() (any, error) {
	if a.setDefaultBrowser != nil {
		return a.setDefaultBrowser()
	}
	status, err := a.app.DefaultBrowser.SetAsDefault()
	if err != nil {
		return status, err
	}
	go func() {
		if _, startErr := a.app.SessionService.StartManagedSOCKSProxies(context.Background()); startErr != nil {
			log.Printf("start URL routing browser proxies: %v", startErr)
		}
	}()
	return status, nil
}

func (a *AppBindings) PendingURLRoute() any {
	request, ok := a.app.URLRoutingService.Pending()
	if !ok {
		return nil
	}
	return request
}

func (a *AppBindings) ResolveURLRoute(requestID string, choiceID string) error {
	return a.app.URLRoutingService.ResolveChoice(context.Background(), requestID, choiceID)
}

func (a *AppBindings) DismissURLRoute(requestID string) {
	a.app.URLRoutingService.DismissChoice(requestID)
}
