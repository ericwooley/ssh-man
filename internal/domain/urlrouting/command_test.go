package urlrouting

import (
	"net/url"
	"reflect"
	"testing"
)

func TestCommandTemplateArgumentsPreserveURLAsArgvData(t *testing.T) {
	rawURL := `https://example.com/a path?q='";$((1+1));$(printf unsafe)&x=1`
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{name: "unquoted", template: `open <URL>`, want: []string{"/usr/bin/open", rawURL}},
		{name: "double quoted embedding", template: `open "container:<URL>"`, want: []string{"/usr/bin/open", "container:" + rawURL}},
		{name: "single quoted embedding", template: `open 'container:<URL>'`, want: []string{"/usr/bin/open", "container:" + rawURL}},
		{name: "multiline argv", template: "open\n<URL>", want: []string{"/usr/bin/open", rawURL}},
		{name: "absolute open path", template: `/usr/bin/open <URL>`, want: []string{"/usr/bin/open", rawURL}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := commandTemplateArguments(tt.template, rawURL)
			if err != nil {
				t.Fatalf("parse command template: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("arguments = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCommandTemplateArgumentsEncodeURLWhenEmbeddedInQueryValue(t *testing.T) {
	rawURL := "https://identity.example.test/oauth2/authorize?response_type=code&client_id=client.apps.example.test&redirect_uri=http%3A%2F%2Flocalhost%3A8086%2F&scope=openid+profile&state=test-state&access_type=offline&code_challenge=test-challenge&code_challenge_method=S256"

	got, err := commandTemplateArguments(
		`open -a "Work Browser" "ext+container:name=Work&url=<URL>"`,
		rawURL,
	)
	if err != nil {
		t.Fatalf("parse command template: %v", err)
	}
	want := []string{
		"/usr/bin/open",
		"-a",
		"Work Browser",
		"ext+container:name=Work&url=" + url.QueryEscape(rawURL),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func TestCommandTemplateArgumentsDoNotClassifyURLPathAsExecutable(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/env",
		"https://example.com/eval",
		"https://example.com/xargs",
		"https://example.com/osascript",
		"-c",
	} {
		got, err := commandTemplateArguments("open <URL>", rawURL)
		if err != nil {
			t.Fatalf("parse URL %q: %v", rawURL, err)
		}
		want := []string{"/usr/bin/open", rawURL}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("arguments = %#v, want %#v", got, want)
		}
	}
}

func TestCommandTemplateArgumentsRejectShellAndReparsingForms(t *testing.T) {
	for _, template := range []string{
		"cat <<EOF\n<URL>\nEOF",
		`printf '%s' <URL> | cat`,
		`eval "printf '%s' <URL>"`,
		`/bin/sh -c "printf '%s' <URL>"`,
		`/bin/zsh -lc "printf '%s' <URL>"`,
		`python3 -c "print('<URL>')"`,
		`/usr/bin/nice /bin/sh -c "printf '%s' <URL>"`,
		`/usr/bin/nohup python3 -c "print('<URL>')"`,
		`find . -exec /bin/sh -c "printf '%s' <URL>" \;`,
		`python3.13 -c "print('<URL>')"`,
		`nodejs --eval "console.log('<URL>')"`,
		`powershell.exe -Command "Write-Output '<URL>'"`,
		`cmd.exe /c "echo <URL>"`,
		`python3 "-cprint('<URL>')"`,
		`node "--eval=console.log('<URL>')"`,
		`perl5.34 "-eprint('<URL>')"`,
		`lua5.5 "-eprint('<URL>')"`,
		`/usr/bin/caffeinate /bin/sh -c "printf '%s' <URL>"`,
		`cmd.exe "/cecho <URL>"`,
		`awk "BEGIN { system(\"printf %s <URL>\") }"`,
		`awk <URL>`,
		`/usr/bin/caffeinate /usr/bin/osascript -e "do shell script \"echo <URL>\""`,
		`/usr/bin/caffeinate /usr/bin/env -S "printf %s <URL>"`,
		`/usr/bin/caffeinate parallel <URL> ::: 1`,
		`su -c <URL>`,
		`ssh host <URL>`,
		`script -c <URL>`,
		`csh -c <URL>`,
		`open -a Terminal --args -c <URL>`,
		`/tmp/open <URL>`,
		`./open <URL>`,
		`OPEN.EXE <URL>`,
		`<URL>/open`,
	} {
		if _, err := commandTemplateArguments(template, "https://example.com/$((1+1))"); err == nil {
			t.Fatalf("expected reparsing template %q to fail", template)
		}
	}
}

func TestRunCommandTemplateRejectsReparsedURLArguments(t *testing.T) {
	for _, template := range []string{
		`python3 "-cprint('<URL>')"`,
		`perl5.34 "-eprint('<URL>')"`,
		`/usr/bin/caffeinate /bin/sh -c "printf '%s' <URL>"`,
		`cmd.exe "/cecho <URL>"`,
		`awk "BEGIN { system(\"printf %s <URL>\") }"`,
		`/usr/bin/caffeinate /usr/bin/osascript -e "do shell script \"echo <URL>\""`,
		`/usr/bin/caffeinate /usr/bin/env -S "printf %s <URL>"`,
		`/usr/bin/caffeinate parallel <URL> ::: 1`,
		`su -c <URL>`,
		`ssh host <URL>`,
		`/tmp/open <URL>`,
		`<URL>/open`,
	} {
		started := false
		err := runCommandTemplateWithStarter(
			template,
			"https://example.com/",
			func(string, []string) error {
				started = true
				return nil
			},
		)
		if err == nil {
			t.Fatalf("expected template %q to fail", template)
		}
		if started {
			t.Fatalf("process starter ran for rejected template %q", template)
		}
	}
}

func TestCommandTemplateArgumentsRequirePlaceholderAndBalancedQuotes(t *testing.T) {
	for _, template := range []string{"open -a Safari", `open "<URL>`, "open '<URL>"} {
		if _, err := commandTemplateArguments(template, "https://example.com"); err == nil {
			t.Fatalf("expected %q to fail", template)
		}
	}
}
func TestRunCommandTemplateStartsExactArguments(t *testing.T) {
	rawURL := `https://example.com/?q=$(printf unsafe)&value=$((1+1))`
	var gotExecutable string
	var gotArguments []string
	err := runCommandTemplateWithStarter(
		`open -a "Work Browser" "container:<URL>"`,
		rawURL,
		func(executable string, arguments []string) error {
			gotExecutable = executable
			gotArguments = append([]string(nil), arguments...)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("run command template: %v", err)
	}
	if gotExecutable != "/usr/bin/open" {
		t.Fatalf("executable = %q, want /usr/bin/open", gotExecutable)
	}
	wantArguments := []string{"-a", "Work Browser", "container:" + rawURL}
	if !reflect.DeepEqual(gotArguments, wantArguments) {
		t.Fatalf("arguments = %#v, want %#v", gotArguments, wantArguments)
	}
}
