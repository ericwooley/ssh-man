package browsercommand

import (
	"fmt"
	"net/url"
	"strings"
)

type quoteState uint8

const (
	unquoted quoteState = iota
	singleQuoted
	doubleQuoted
)

// Arguments parses a macOS open command and substitutes rawURL as argv data.
// Quoting and escaping are supported, but shell syntax and child-process
// argument forwarding are deliberately excluded.
func Arguments(template, rawURL string) ([]string, error) {
	var arguments []string
	var argument strings.Builder
	state := unquoted
	wordStarted := false
	replacedURL := false

	finishArgument := func() {
		if !wordStarted {
			return
		}
		arguments = append(arguments, argument.String())
		argument.Reset()
		wordStarted = false
	}

	for index := 0; index < len(template); {
		if strings.HasPrefix(template[index:], "<URL>") {
			argument.WriteString(urlPlaceholderValue(argument.String(), rawURL))
			wordStarted = true
			replacedURL = true
			index += len("<URL>")
			continue
		}

		value := template[index]
		if state == unquoted && isWhitespace(value) {
			finishArgument()
			index++
			continue
		}
		if state == unquoted && strings.ContainsRune("|&;<>", rune(value)) {
			return nil, fmt.Errorf("command templates do not support shell operators or redirects")
		}
		if value == '\\' && state != singleQuoted {
			if index+1 >= len(template) {
				return nil, fmt.Errorf("command contains an unbalanced escape")
			}
			argument.WriteByte(template[index+1])
			wordStarted = true
			index += 2
			continue
		}

		switch value {
		case '\'':
			if state == unquoted {
				state = singleQuoted
				wordStarted = true
				index++
				continue
			}
			if state == singleQuoted {
				state = unquoted
				index++
				continue
			}
		case '"':
			if state == unquoted {
				state = doubleQuoted
				wordStarted = true
				index++
				continue
			}
			if state == doubleQuoted {
				state = unquoted
				index++
				continue
			}
		}

		argument.WriteByte(value)
		wordStarted = true
		index++
	}

	if state != unquoted {
		return nil, fmt.Errorf("command contains unbalanced quotes")
	}
	finishArgument()
	if !replacedURL {
		return nil, fmt.Errorf("command must contain <URL>")
	}
	if len(arguments) == 0 || arguments[0] == "" {
		return nil, fmt.Errorf("command executable is required")
	}
	switch arguments[0] {
	case "open":
		arguments[0] = "/usr/bin/open"
	case "/usr/bin/open":
	default:
		return nil, fmt.Errorf("command executable must be open or /usr/bin/open")
	}
	for _, value := range arguments[1:] {
		if value == "--args" {
			return nil, fmt.Errorf("open command must not forward child-process arguments")
		}
	}
	return arguments, nil
}

func urlPlaceholderValue(argumentPrefix, rawURL string) string {
	queryDelimiter := strings.LastIndexAny(argumentPrefix, "?&")
	if queryDelimiter < 0 {
		return rawURL
	}
	if strings.Contains(argumentPrefix[queryDelimiter+1:], "=") {
		return url.QueryEscape(rawURL)
	}
	return rawURL
}

// Validate checks the same grammar used at execution time.
func Validate(template string) error {
	_, err := Arguments(template, "https://ssh-man.invalid/")
	return err
}

func isWhitespace(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}
