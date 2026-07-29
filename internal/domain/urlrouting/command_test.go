package urlrouting

import (
	"strings"
	"testing"
)

func TestExpandCommandTemplatePreservesURLAsDataAcrossQuoteContexts(t *testing.T) {
	rawURL := "https://example.com/a path?q='\"$`&x=1"
	tests := []struct {
		name     string
		template string
	}{
		{name: "unquoted", template: `open <URL>`},
		{name: "double quoted", template: `open "container:<URL>"`},
		{name: "single quoted", template: `open 'container:<URL>'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded, err := expandCommandTemplate(tt.template, rawURL)
			if err != nil {
				t.Fatalf("expand template: %v", err)
			}
			if strings.Contains(expanded, "<URL>") {
				t.Fatalf("placeholder remained in %q", expanded)
			}
			if !strings.Contains(expanded, "example.com") {
				t.Fatalf("URL missing from %q", expanded)
			}
		})
	}
}

func TestExpandCommandTemplateRequiresPlaceholderAndBalancedQuotes(t *testing.T) {
	for _, template := range []string{"open -a Safari", `open "<URL>`, "open '<URL>"} {
		if _, err := expandCommandTemplate(template, "https://example.com"); err == nil {
			t.Fatalf("expected %q to fail", template)
		}
	}
}
