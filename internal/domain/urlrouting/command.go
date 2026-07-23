package urlrouting

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type shellQuoteState uint8

const (
	shellUnquoted shellQuoteState = iota
	shellSingleQuoted
	shellDoubleQuoted
)

func expandCommandTemplate(template, rawURL string) (string, error) {
	if !strings.Contains(template, "<URL>") {
		return "", fmt.Errorf("command must contain <URL>")
	}

	var result strings.Builder
	state := shellUnquoted
	escaped := false
	for index := 0; index < len(template); {
		if !escaped && strings.HasPrefix(template[index:], "<URL>") {
			switch state {
			case shellSingleQuoted:
				result.WriteString(strings.ReplaceAll(rawURL, "'", "'\\''"))
			case shellDoubleQuoted:
				replacer := strings.NewReplacer(
					`\`, `\\`,
					`"`, `\"`,
					`$`, `\$`,
					"`", "\\`",
				)
				result.WriteString(replacer.Replace(rawURL))
			default:
				result.WriteString(shellQuote(rawURL))
			}
			index += len("<URL>")
			continue
		}

		value := template[index]
		result.WriteByte(value)
		index++
		if escaped {
			escaped = false
			continue
		}
		if value == '\\' && state != shellSingleQuoted {
			escaped = true
			continue
		}
		switch value {
		case '\'':
			if state == shellUnquoted {
				state = shellSingleQuoted
			} else if state == shellSingleQuoted {
				state = shellUnquoted
			}
		case '"':
			if state == shellUnquoted {
				state = shellDoubleQuoted
			} else if state == shellDoubleQuoted {
				state = shellUnquoted
			}
		}
	}
	if escaped || state != shellUnquoted {
		return "", fmt.Errorf("command contains unbalanced quotes or escapes")
	}
	return result.String(), nil
}

func runCommandTemplate(template, rawURL string) error {
	command, err := expandCommandTemplate(template, rawURL)
	if err != nil {
		return err
	}
	shell := "/bin/sh"
	if runtime.GOOS == "darwin" {
		shell = "/bin/zsh"
	}
	if err := exec.Command(shell, "-lc", command).Start(); err != nil {
		return fmt.Errorf("execute URL command: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
