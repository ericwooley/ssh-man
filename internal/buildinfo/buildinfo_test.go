package buildinfo

import "testing"

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "local build", version: "dev", want: "Dev build"},
		{name: "missing build metadata", version: "", want: "Dev build"},
		{name: "official release", version: "1.2.3", want: "1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayVersion(tt.version); got != tt.want {
				t.Fatalf("DisplayVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
