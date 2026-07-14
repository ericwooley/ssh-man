package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type globalOptions struct {
	Output         string
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
	NoAutostart    bool
	Help           bool
	Version        bool
}

func defaultGlobalOptions() globalOptions {
	return globalOptions{
		Output:         "table",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 30 * time.Second,
	}
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	options := defaultGlobalOptions()
	remaining := make([]string, 0, len(args))

	for index := 0; index < len(args); index++ {
		argument := args[index]
		name, value, hasValue := splitLongOption(argument)
		switch name {
		case "--output", "-o":
			if !hasValue {
				index++
				if index >= len(args) {
					return options, nil, usageErrorf("%s requires a value", name)
				}
				value = args[index]
			}
			if value != "table" && value != "json" && value != "jsonl" {
				return options, nil, usageErrorf("output must be table, json, or jsonl")
			}
			options.Output = value
		case "--connect-timeout":
			parsed, next, err := durationOption(args, index, value, hasValue, name)
			if err != nil {
				return options, nil, err
			}
			options.ConnectTimeout = parsed
			index = next
		case "--request-timeout":
			parsed, next, err := durationOption(args, index, value, hasValue, name)
			if err != nil {
				return options, nil, err
			}
			options.RequestTimeout = parsed
			index = next
		case "--no-autostart":
			options.NoAutostart = true
		case "--help", "-h":
			options.Help = true
		case "--version":
			options.Version = true
		default:
			remaining = append(remaining, argument)
		}
	}

	return options, remaining, nil
}

func durationOption(args []string, index int, inlineValue string, hasInlineValue bool, name string) (time.Duration, int, error) {
	value := inlineValue
	if !hasInlineValue {
		index++
		if index >= len(args) {
			return 0, index, usageErrorf("%s requires a duration", name)
		}
		value = args[index]
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, index, usageErrorf("%s must be a positive duration", name)
	}
	return parsed, index, nil
}

type optionKind int

const (
	stringOption optionKind = iota
	boolOption
)

type parsedOptions struct {
	values      map[string]string
	present     map[string]bool
	positionals []string
}

func parseOptions(args []string, specs map[string]optionKind) (parsedOptions, error) {
	parsed := parsedOptions{values: map[string]string{}, present: map[string]bool{}}
	positionalOnly := false

	for index := 0; index < len(args); index++ {
		argument := args[index]
		if positionalOnly || !strings.HasPrefix(argument, "-") || argument == "-" {
			parsed.positionals = append(parsed.positionals, argument)
			continue
		}
		if argument == "--" {
			positionalOnly = true
			continue
		}

		name, value, hasValue := splitLongOption(argument)
		kind, ok := specs[name]
		if !ok {
			return parsed, usageErrorf("unknown option %s", name)
		}

		switch kind {
		case stringOption:
			if !hasValue {
				index++
				if index >= len(args) {
					return parsed, usageErrorf("%s requires a value", name)
				}
				value = args[index]
			}
		case boolOption:
			if !hasValue {
				value = "true"
				if index+1 < len(args) {
					if _, err := strconv.ParseBool(args[index+1]); err == nil {
						index++
						value = args[index]
					}
				}
			}
			if _, err := strconv.ParseBool(value); err != nil {
				return parsed, usageErrorf("%s must be true or false", name)
			}
		}

		parsed.values[name] = value
		parsed.present[name] = true
	}

	return parsed, nil
}

func splitLongOption(argument string) (string, string, bool) {
	if index := strings.IndexByte(argument, '='); index >= 0 {
		return argument[:index], argument[index+1:], true
	}
	return argument, "", false
}

func (p parsedOptions) string(name string) string {
	return p.values[name]
}

func (p parsedOptions) has(name string) bool {
	return p.present[name]
}

func (p parsedOptions) bool(name string) bool {
	value, _ := strconv.ParseBool(p.values[name])
	return value
}

func (p parsedOptions) int(name string) (int, error) {
	value := p.string(name)
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, usageErrorf("%s must be a number", name)
	}
	return parsed, nil
}

func requirePositionals(parsed parsedOptions, count int, usage string) error {
	if len(parsed.positionals) != count {
		return usageErrorf("usage: %s", usage)
	}
	return nil
}

func requireOption(parsed parsedOptions, name string) (string, error) {
	value := strings.TrimSpace(parsed.string(name))
	if value == "" {
		return "", usageErrorf("%s is required", name)
	}
	return value, nil
}

type usageError struct{ message string }

func (e *usageError) Error() string { return e.message }

func usageErrorf(format string, values ...any) error {
	return &usageError{message: fmt.Sprintf(format, values...)}
}
