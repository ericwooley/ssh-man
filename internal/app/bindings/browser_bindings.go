package bindings

import "context"

func (a *AppBindings) DiscoverBrowsers() (any, error) {
	return a.app.BrowserService.Discover(context.Background())
}

func (a *AppBindings) PreviewBrowserLaunchThroughSocks(configurationID string, browserID string) (any, error) {
	return a.app.BrowserService.PreviewLaunchThroughSOCKS(context.Background(), configurationID, browserID)
}

func (a *AppBindings) LaunchBrowserThroughSocks(configurationID string, browserID string) error {
	return a.app.BrowserService.LaunchThroughSOCKS(context.Background(), configurationID, browserID)
}
