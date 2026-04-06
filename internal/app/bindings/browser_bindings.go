package bindings

import "context"

func (a *AppBindings) DiscoverBrowsers() (any, error) {
	return a.app.BrowserService.Discover(context.Background())
}

func (a *AppBindings) LaunchBrowserThroughSocks(configurationID string, browserID string) error {
	return a.app.BrowserService.LaunchThroughSOCKS(context.Background(), configurationID, browserID)
}
