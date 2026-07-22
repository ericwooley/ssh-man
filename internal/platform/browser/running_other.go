//go:build !darwin

package browser

import (
	"context"
	"fmt"
	"runtime"

	serverdomain "ssh-man/internal/domain/server"
)

func listRunningBrowserTargets(context.Context, string, []BrowserOption, []serverdomain.Server) ([]RunningTarget, error) {
	return nil, fmt.Errorf("browser switching is not supported on %s", runtime.GOOS)
}

func activateRunningBrowser(int) error {
	return fmt.Errorf("browser switching is not supported on %s", runtime.GOOS)
}
