package bootstrap

import (
	"context"

	urlroutingdomain "ssh-man/internal/domain/urlrouting"
	"ssh-man/internal/platform/browser"
)

type urlRoutingBrowserAdapter struct {
	service *browser.Service
}

func (a urlRoutingBrowserAdapter) ListDestinations(ctx context.Context) ([]urlroutingdomain.BrowserDestination, error) {
	options, err := a.service.Discover(ctx)
	if err != nil {
		return nil, err
	}
	destinations := make([]urlroutingdomain.BrowserDestination, 0, len(options))
	for _, option := range options {
		destinations = append(destinations, urlroutingdomain.BrowserDestination{
			ID:            option.ID,
			Name:          option.DisplayName,
			SupportsProxy: option.SupportsProxyLaunch,
			Command:       option.CommandTemplate,
			Icon:          option.Icon,
		})
	}
	return destinations, nil
}

func (a urlRoutingBrowserAdapter) OpenURL(ctx context.Context, browserID, rawURL string) error {
	return a.service.OpenURL(ctx, browserID, rawURL)
}

func (a urlRoutingBrowserAdapter) LaunchThroughSOCKSURL(ctx context.Context, configurationID, browserID, rawURL string) error {
	return a.service.LaunchThroughSOCKSURL(ctx, configurationID, browserID, rawURL)
}
