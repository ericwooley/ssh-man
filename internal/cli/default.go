package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"ssh-man/internal/control"
	"ssh-man/internal/platform/paths"
)

type launcher func(context.Context) error

func defaultConnector(launch launcher) Connector {
	return func(ctx context.Context, options ConnectOptions) (Caller, error) {
		configDir, err := paths.ConfigDir()
		if err != nil {
			return nil, err
		}
		client := control.NewClient(paths.ControlSocketPath(configDir), options.RequestTimeout)

		pingContext, cancelPing := context.WithTimeout(ctx, minDuration(options.ConnectTimeout, time.Second))
		err = client.Ping(pingContext)
		cancelPing()
		if err == nil {
			return client, nil
		}
		var protocolMismatch *control.ProtocolMismatchError
		var remoteError *control.RemoteError
		if errors.As(err, &protocolMismatch) || (errors.As(err, &remoteError) && remoteError.Code == "protocol_mismatch") || strings.Contains(strings.ToLower(err.Error()), "protocol mismatch") {
			return nil, err
		}
		if !options.Autostart {
			return nil, fmt.Errorf("SSH Man is not running; open the app or remove --no-autostart")
		}
		if launch == nil {
			return nil, fmt.Errorf("SSH Man is not running and cannot be started automatically")
		}
		if err := launch(ctx); err != nil {
			return nil, fmt.Errorf("start SSH Man: %w", err)
		}

		waitContext, cancelWait := context.WithTimeout(ctx, options.ConnectTimeout)
		defer cancelWait()
		if err := client.Wait(waitContext); err != nil {
			return nil, fmt.Errorf("SSH Man did not become ready; if it was just upgraded, quit and reopen the menu-bar app: %w", err)
		}
		return client, nil
	}
}

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func stdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func readTerminalPassword() ([]byte, error) {
	return term.ReadPassword(int(os.Stdin.Fd()))
}
