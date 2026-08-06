package buildassets

import _ "embed"

//go:embed appicon.png
var applicationIconPNG []byte

// ApplicationIconPNG returns a copy of the application icon.
func ApplicationIconPNG() []byte {
	return append([]byte(nil), applicationIconPNG...)
}
