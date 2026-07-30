package urlrouting

import (
	"fmt"
	"os/exec"

	"ssh-man/internal/domain/browsercommand"
)

func commandTemplateArguments(template, rawURL string) ([]string, error) {
	return browsercommand.Arguments(template, rawURL)
}

func runCommandTemplate(template, rawURL string) error {
	return runCommandTemplateWithStarter(template, rawURL, startCommand)
}

func runCommandTemplateWithStarter(
	template string,
	rawURL string,
	start func(executable string, arguments []string) error,
) error {
	arguments, err := commandTemplateArguments(template, rawURL)
	if err != nil {
		return err
	}
	if err := start(arguments[0], arguments[1:]); err != nil {
		return fmt.Errorf("execute URL command: %w", err)
	}
	return nil
}

func startCommand(executable string, arguments []string) error {
	return exec.Command(executable, arguments...).Start()
}
