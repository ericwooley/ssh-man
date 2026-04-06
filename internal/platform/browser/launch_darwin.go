package browser

import (
	"fmt"
	"os/exec"
)

func launchDarwin(option BrowserOption, socksPort int) error {
	if !option.SupportsProxyLaunch {
		return fmt.Errorf("%s does not support proxy launch in this MVP", option.DisplayName)
	}
	cmd := exec.Command("open", "-na", option.LaunchReference, "--args", fmt.Sprintf("--proxy-server=socks5://127.0.0.1:%d", socksPort))
	return cmd.Start()
}
