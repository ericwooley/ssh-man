package commandwindow

import "testing"

func TestServerIDFromArgs(t *testing.T) {
	got, ok := ServerIDFromArgs([]string{ServerArgument, "server-1"})
	if !ok || got != "server-1" {
		t.Fatalf("ServerIDFromArgs() = %q, %v", got, ok)
	}
}
