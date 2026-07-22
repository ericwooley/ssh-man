package explorerwindow

import "testing"

func TestServerIDFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{name: "separate", args: []string{ServerArgument, "server-1"}, want: "server-1", ok: true},
		{name: "inline", args: []string{ServerArgument + "=server-2"}, want: "server-2", ok: true},
		{name: "missing", args: []string{"status"}},
		{name: "blank", args: []string{ServerArgument, "  "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ServerIDFromArgs(test.args)
			if got != test.want || ok != test.ok {
				t.Fatalf("ServerIDFromArgs() = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}
