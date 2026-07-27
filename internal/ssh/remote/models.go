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

type UploadFailure struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type UploadResult struct {
	Uploaded []string        `json:"uploaded"`
	Failures []UploadFailure `json:"failures"`
}

type UploadStatus string

const (
	UploadStatusTransferring UploadStatus = "transferring"
	UploadStatusCompleted    UploadStatus = "completed"
	UploadStatusFailed       UploadStatus = "failed"
)

type UploadProgress struct {
	UploadID              int          `json:"uploadId"`
	FileIndex             int          `json:"fileIndex"`
	Name                  string       `json:"name"`
	Status                UploadStatus `json:"status"`
	BytesTransferred      int64        `json:"bytesTransferred"`
	TotalBytes            int64        `json:"totalBytes"`
	OverallBytesProcessed int64        `json:"overallBytesProcessed"`
	OverallBytesTotal     int64        `json:"overallBytesTotal"`
	FilesProcessed        int          `json:"filesProcessed"`
	FilesTotal            int          `json:"filesTotal"`
	FailureCode           string       `json:"failureCode,omitempty"`
}

type UploadProgressReporter func(UploadProgress)
