package remote

import "time"

const MaxPreviewBytes int64 = 2 * 1024 * 1024

type Entry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Kind       string    `json:"kind"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
	Mode       string    `json:"mode"`
	Hidden     bool      `json:"hidden"`
}

type Directory struct {
	Path    string  `json:"path"`
	Entries []Entry `json:"entries"`
}

type Preview struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	MimeType  string `json:"mimeType"`
	Content   string `json:"content,omitempty"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
	Revision  string `json:"revision,omitempty"`
}

type ConnectResult struct {
	Connected       bool   `json:"connected"`
	NeedsPassphrase bool   `json:"needsPassphrase,omitempty"`
	HomePath        string `json:"homePath,omitempty"`
}
