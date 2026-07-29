package browser

import "testing"

func TestWindowsPowerShellQuotePreservesLiteralArguments(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "simple", value: "msedge.exe", want: "'msedge.exe'"},
		{name: "empty", value: "", want: "''"},
		{name: "path with spaces", value: `C:\Program Files\Microsoft\Edge\msedge.exe`, want: `'C:\Program Files\Microsoft\Edge\msedge.exe'`},
		{name: "embedded double quote", value: `say "hello"`, want: `'say "hello"'`},
		{name: "embedded single quote", value: `it's literal`, want: `'it''s literal'`},
		{name: "shell metacharacters", value: `<-loopback>;$env:PATH&exit`, want: `'<-loopback>;$env:PATH&exit'`},
		{name: "trailing slash", value: `C:\path with spaces\`, want: `'C:\path with spaces\'`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := windowsPowerShellQuote(test.value); got != test.want {
				t.Fatalf("windowsPowerShellQuote(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestFormatWindowsCommandProducesCopyablePowerShell(t *testing.T) {
	got := formatWindowsCommand(
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`--proxy-server=socks5://127.0.0.1:41001`,
		`--proxy-bypass-list=<-loopback>`,
		`--user-data-dir=C:\Users\Test\App Data\ssh-man\browser-profiles\server-1\microsoft-edge\chromium`,
	)
	want := `& 'C:\Program Files\Microsoft\Edge\Application\msedge.exe' '--proxy-server=socks5://127.0.0.1:41001' '--proxy-bypass-list=<-loopback>' '--user-data-dir=C:\Users\Test\App Data\ssh-man\browser-profiles\server-1\microsoft-edge\chromium'`
	if got != want {
		t.Fatalf("formatWindowsCommand() = %q, want %q", got, want)
	}
}
