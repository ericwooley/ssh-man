package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	serverdomain "ssh-man/internal/domain/server"
	"ssh-man/internal/ssh/auth"
)

type memoryNode struct {
	name    string
	content []byte
	mode    os.FileMode
	modTime time.Time
}

func (n memoryNode) Name() string       { return n.name }
func (n memoryNode) Size() int64        { return int64(len(n.content)) }
func (n memoryNode) Mode() os.FileMode  { return n.mode }
func (n memoryNode) ModTime() time.Time { return n.modTime }
func (n memoryNode) IsDir() bool        { return n.mode.IsDir() }
func (n memoryNode) Sys() any           { return nil }

type memoryFile struct{ *bytes.Reader }

func (memoryFile) Close() error { return nil }

type memoryWriteFile struct {
	buffer bytes.Buffer
	fs     *memoryFS
	name   string
	closed bool
}

func (f *memoryWriteFile) Write(content []byte) (int, error) {
	if path.Base(f.name) == f.fs.failWriteName {
		written, _ := f.buffer.Write(content[:len(content)/2])
		return written, errors.New("simulated remote write failure")
	}
	return f.buffer.Write(content)
}

func (f *memoryWriteFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	f.fs.nodes[f.name] = memoryNode{
		name:    path.Base(f.name),
		content: append([]byte(nil), f.buffer.Bytes()...),
		mode:    f.fs.nodes[f.name].mode,
		modTime: time.Now(),
	}
	return nil
}

type memoryFS struct {
	home              string
	nodes             map[string]memoryNode
	symlinkTargets    map[string]string
	onOpenFile        func()
	failWriteName     string
	openFileErrorName string
	openFileError     error
	chmodErrorName    string
	chmodError        error
	removeErrorName   string
	removeError       error
}

func (f *memoryFS) Getwd() (string, error) { return f.home, nil }
func (f *memoryFS) Close() error           { return nil }
func (f *memoryFS) Lstat(name string) (os.FileInfo, error) {
	node, ok := f.nodes[cleanRemotePath(name)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return node, nil
}
func (f *memoryFS) Stat(name string) (os.FileInfo, error) {
	name = cleanRemotePath(name)
	node, ok := f.nodes[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	if node.Mode()&os.ModeSymlink == 0 {
		return node, nil
	}
	target, ok := f.symlinkTargets[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	node, ok = f.nodes[cleanRemotePath(target)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return node, nil
}
func (f *memoryFS) Open(name string) (ReadSeekCloser, error) {
	node, ok := f.nodes[cleanRemotePath(name)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return memoryFile{Reader: bytes.NewReader(node.content)}, nil
}
func (f *memoryFS) OpenFile(name string, flag int, mode os.FileMode) (io.WriteCloser, error) {
	name = cleanRemotePath(name)
	if _, exists := f.nodes[name]; exists && flag&os.O_EXCL != 0 {
		return nil, os.ErrExist
	}
	if path.Base(name) == f.openFileErrorName {
		return nil, f.openFileError
	}
	f.nodes[name] = memoryNode{
		name:    path.Base(name),
		mode:    0o600,
		modTime: time.Now(),
	}
	if f.onOpenFile != nil {
		f.onOpenFile()
	}
	return &memoryWriteFile{fs: f, name: name}, nil
}
func (f *memoryFS) Chmod(name string, mode os.FileMode) error {
	name = cleanRemotePath(name)
	if path.Base(name) == f.chmodErrorName {
		return f.chmodError
	}
	node, exists := f.nodes[name]
	if !exists {
		return os.ErrNotExist
	}
	node.mode = node.mode.Type() | mode.Perm()
	f.nodes[name] = node
	return nil
}
func (f *memoryFS) Mkdir(name string) error {
	name = cleanRemotePath(name)
	if _, exists := f.nodes[name]; exists {
		return os.ErrExist
	}
	if parent, exists := f.nodes[path.Dir(name)]; !exists || !parent.IsDir() {
		return os.ErrNotExist
	}
	f.nodes[name] = memoryNode{
		name:    path.Base(name),
		mode:    os.ModeDir | 0o755,
		modTime: time.Now(),
	}
	return nil
}
func (f *memoryFS) PosixRename(oldName, newName string) error {
	oldName = cleanRemotePath(oldName)
	newName = cleanRemotePath(newName)
	node, exists := f.nodes[oldName]
	if !exists {
		return os.ErrNotExist
	}
	descendants := map[string]memoryNode{}
	for nodePath, child := range f.nodes {
		if strings.HasPrefix(nodePath, oldName+"/") {
			descendants[newName+strings.TrimPrefix(nodePath, oldName)] = child
			delete(f.nodes, nodePath)
		}
	}
	delete(f.nodes, oldName)
	node.name = path.Base(newName)
	f.nodes[newName] = node
	for nodePath, child := range descendants {
		f.nodes[nodePath] = child
	}
	return nil
}
func (f *memoryFS) Remove(name string) error {
	name = cleanRemotePath(name)
	if path.Base(name) == f.removeErrorName {
		return f.removeError
	}
	if _, exists := f.nodes[name]; !exists {
		return os.ErrNotExist
	}
	delete(f.nodes, name)
	return nil
}
func (f *memoryFS) RemoveDirectory(name string) error {
	name = cleanRemotePath(name)
	node, exists := f.nodes[name]
	if !exists {
		return os.ErrNotExist
	}
	if !node.IsDir() {
		return errors.New("not a directory")
	}
	for nodePath := range f.nodes {
		if strings.HasPrefix(nodePath, name+"/") {
			return errors.New("directory not empty")
		}
	}
	delete(f.nodes, name)
	return nil
}
func (f *memoryFS) ReadDir(name string) ([]os.FileInfo, error) {
	name = cleanRemotePath(name)
	if node, ok := f.nodes[name]; !ok || !node.IsDir() {
		return nil, os.ErrNotExist
	}
	children := []os.FileInfo{}
	for itemPath, node := range f.nodes {
		if itemPath != name && path.Dir(itemPath) == name {
			children = append(children, node)
		}
	}
	return children, nil
}

func testMemoryFS() *memoryFS {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	return &memoryFS{
		home:           "/home/eric",
		symlinkTargets: map[string]string{},
		nodes: map[string]memoryNode{
			"/home/eric":             {name: "eric", mode: os.ModeDir | 0o755, modTime: now},
			"/home/eric/Projects":    {name: "Projects", mode: os.ModeDir | 0o755, modTime: now},
			"/home/eric/Projects/a":  {name: "a", mode: 0o644, content: []byte("alpha"), modTime: now},
			"/home/eric/README.md":   {name: "README.md", mode: 0o644, content: []byte("# Hello"), modTime: now},
			"/home/eric/archive.bin": {name: "archive.bin", mode: 0o644, content: []byte{'a', 0, 'b'}, modTime: now},
			"/home/eric/.env":        {name: ".env", mode: 0o600, content: []byte("KEY=value"), modTime: now},
		},
	}
}

func connectedTestService(t *testing.T) *Service {
	t.Helper()
	remoteFS := testMemoryFS()
	service := NewServiceWithDialer(serverdomain.Server{ID: "server-1"}, func(context.Context, serverdomain.Server, string) (FileSystem, io.Closer, error) {
		return remoteFS, nil, nil
	})
	result, err := service.Connect(context.Background(), "")
	if err != nil || !result.Connected {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
	return service
}

func TestConnectReportsEncryptedKeyWithoutReturningAnError(t *testing.T) {
	service := NewServiceWithDialer(serverdomain.Server{}, func(context.Context, serverdomain.Server, string) (FileSystem, io.Closer, error) {
		return nil, nil, auth.ErrPassphraseRequired
	})

	result, err := service.Connect(context.Background(), "")

	if err != nil || !result.NeedsPassphrase || result.Connected {
		t.Fatalf("Connect() = %#v, %v", result, err)
	}
}

func TestListResolvesHomeAndSortsDirectoriesBeforeFiles(t *testing.T) {
	service := connectedTestService(t)

	directory, err := service.List("~")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(directory.Entries))
	for _, entry := range directory.Entries {
		names = append(names, entry.Name)
	}
	want := []string{"Projects", ".env", "archive.bin", "README.md"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("entry names = %#v, want %#v", names, want)
	}
	if !directory.Entries[1].Hidden {
		t.Fatal("dotfile should be marked hidden")
	}
}

func TestPreviewClassifiesMarkdownAndRejectsBinaryText(t *testing.T) {
	service := connectedTestService(t)

	markdown, err := service.Preview("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if markdown.Kind != "markdown" || markdown.Content != "# Hello" {
		t.Fatalf("markdown preview = %#v", markdown)
	}

	binary, err := service.Preview("archive.bin")
	if err != nil {
		t.Fatal(err)
	}
	if binary.Kind != "unsupported" || binary.Content != "" {
		t.Fatalf("binary preview = %#v", binary)
	}
}

func TestClassifyPreviewTreatsVideoFormatsAsVideo(t *testing.T) {
	for _, name := range []string{"capture.mp4", "capture.webm"} {
		t.Run(name, func(t *testing.T) {
			kind, mimeType := classifyPreview(name)

			if kind != "video" {
				t.Fatalf("classifyPreview() kind = %q, want video", kind)
			}
			if !strings.HasPrefix(mimeType, "video/") {
				t.Fatalf("classifyPreview() MIME type = %q, want video MIME type", mimeType)
			}
		})
	}
}

func TestSaveReplacesRemoteTextAndPreservesPermissions(t *testing.T) {
	service := connectedTestService(t)
	preview, err := service.Preview("README.md")
	if err != nil {
		t.Fatal(err)
	}

	saved, err := service.Save("README.md", "# Updated\n", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}

	if saved.Content != "# Updated\n" || saved.Revision == preview.Revision {
		t.Fatalf("saved preview = %#v", saved)
	}
	remoteFS := service.fs.(*memoryFS)
	node := remoteFS.nodes["/home/eric/README.md"]
	if string(node.content) != "# Updated\n" || node.mode.Perm() != 0o644 {
		t.Fatalf("saved node = %#v", node)
	}
	for nodePath := range remoteFS.nodes {
		if strings.Contains(nodePath, ".ssh-man-save-") {
			t.Fatalf("temporary save file was not removed: %s", nodePath)
		}
	}
}

func TestSaveRejectsAnExternallyChangedRemoteFile(t *testing.T) {
	service := connectedTestService(t)
	preview, err := service.Preview("README.md")
	if err != nil {
		t.Fatal(err)
	}
	remoteFS := service.fs.(*memoryFS)
	node := remoteFS.nodes["/home/eric/README.md"]
	node.content = []byte("# Changed elsewhere")
	remoteFS.nodes["/home/eric/README.md"] = node

	_, err = service.Save("README.md", "# Local edit", preview.Revision)

	if !errors.Is(err, ErrRemoteFileChanged) {
		t.Fatalf("Save() error = %v, want ErrRemoteFileChanged", err)
	}
	if got := string(remoteFS.nodes["/home/eric/README.md"].content); got != "# Changed elsewhere" {
		t.Fatalf("remote content = %q, want external edit preserved", got)
	}
}

func TestUploadCopiesLocalFilesIntoRemoteDirectory(t *testing.T) {
	service := connectedTestService(t)
	localDirectory := t.TempDir()
	localPath := filepath.Join(localDirectory, "report.txt")
	if err := os.WriteFile(localPath, []byte("upload contents"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(localPath, 0o750); err != nil {
		t.Fatal(err)
	}

	result, err := service.Upload(context.Background(), []string{localPath}, "~/Projects")
	if err != nil {
		t.Fatal(err)
	}

	wantPath := "/home/eric/Projects/report.txt"
	if !reflect.DeepEqual(result.Uploaded, []string{wantPath}) || len(result.Failures) != 0 {
		t.Fatalf("Upload() = %#v, want uploaded path %q", result, wantPath)
	}
	node := service.fs.(*memoryFS).nodes[wantPath]
	if got := string(node.content); got != "upload contents" {
		t.Fatalf("uploaded content = %q", got)
	}
	if got := node.mode.Perm(); got != 0o750 {
		t.Fatalf("uploaded mode = %v, want %v", got, os.FileMode(0o750))
	}
}

func TestUploadReportsByteProgressAndTerminalStateForEveryFile(t *testing.T) {
	service := connectedTestService(t)
	localDirectory := t.TempDir()
	firstPath := filepath.Join(localDirectory, "first.txt")
	existingPath := filepath.Join(localDirectory, "README.md")
	if err := os.WriteFile(firstPath, []byte("first upload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingPath, []byte("existing name"), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress []UploadProgress

	result, err := service.UploadWithProgress(
		context.Background(),
		[]string{firstPath, existingPath},
		"~",
		func(next UploadProgress) {
			progress = append(progress, next)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Uploaded, []string{"/home/eric/first.txt"}) {
		t.Fatalf("uploaded paths = %#v", result.Uploaded)
	}

	var started, completed, failed *UploadProgress
	for index := range progress {
		next := &progress[index]
		switch {
		case next.FileIndex == 0 && next.Status == UploadStatusTransferring && next.BytesTransferred == 0:
			started = next
		case next.FileIndex == 0 && next.Status == UploadStatusCompleted:
			completed = next
		case next.FileIndex == 1 && next.Status == UploadStatusFailed:
			failed = next
		}
	}
	if started == nil || started.Name != "first.txt" || started.TotalBytes != int64(len("first upload")) {
		t.Fatalf("starting progress = %#v", started)
	}
	if completed == nil || completed.BytesTransferred != completed.TotalBytes || completed.FilesProcessed != 1 {
		t.Fatalf("completed progress = %#v", completed)
	}
	if failed == nil || failed.FailureCode != UploadFailureExists || failed.FilesProcessed != 2 {
		t.Fatalf("failed progress = %#v", failed)
	}
	if failed.OverallBytesProcessed != failed.OverallBytesTotal {
		t.Fatalf("final overall progress = %d/%d, want all work settled", failed.OverallBytesProcessed, failed.OverallBytesTotal)
	}
}

func TestUploadRemovesUnsafeWritePermissionsFromLocalFiles(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "shared-script.sh")
	if err := os.WriteFile(localPath, []byte("#!/bin/sh\n"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(localPath, 0o777); err != nil {
		t.Fatal(err)
	}

	result, err := service.Upload(context.Background(), []string{localPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	wantPath := "/home/eric/shared-script.sh"
	if !reflect.DeepEqual(result.Uploaded, []string{wantPath}) || len(result.Failures) != 0 {
		t.Fatalf("Upload() = %#v, want uploaded path %q", result, wantPath)
	}
	if got := service.fs.(*memoryFS).nodes[wantPath].mode.Perm(); got != 0o755 {
		t.Fatalf("uploaded mode = %v, want %v", got, os.FileMode(0o755))
	}
}

func TestUploadFailureCodeSeparatesLocalReadPermissions(t *testing.T) {
	err := fmt.Errorf("%w: open local file", ErrLocalFileUnreadable)

	if got := uploadFailureCode(err); got != UploadFailureLocalPermission {
		t.Fatalf("uploadFailureCode() = %q, want %q", got, UploadFailureLocalPermission)
	}
}

func TestUploadReportsUnreadableLocalFiles(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(localPath, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(localPath, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(localPath, 0o600) }()
	if file, err := os.Open(localPath); err == nil {
		_ = file.Close()
		t.Log("filesystem allows the current process to read mode-000 files; local permission behavior was not exercised")
		return
	}

	result, err := service.Upload(context.Background(), []string{localPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	want := []UploadFailure{{Name: "private.txt", Code: UploadFailureLocalPermission}}
	if len(result.Uploaded) != 0 || !reflect.DeepEqual(result.Failures, want) {
		t.Fatalf("Upload() = %#v, want local permission failure", result)
	}
}

func TestUploadReportsMissingLocalFiles(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "moved.txt")

	result, err := service.Upload(context.Background(), []string{localPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	want := []UploadFailure{{Name: "moved.txt", Code: UploadFailureMissing}}
	if len(result.Uploaded) != 0 || !reflect.DeepEqual(result.Failures, want) {
		t.Fatalf("Upload() = %#v, want missing-local-file failure", result)
	}
}

func TestUploadContinuesAfterAnExistingRemoteFile(t *testing.T) {
	service := connectedTestService(t)
	localDirectory := t.TempDir()
	localPaths := []string{
		filepath.Join(localDirectory, "before.txt"),
		filepath.Join(localDirectory, "README.md"),
		filepath.Join(localDirectory, "after.txt"),
	}
	for _, localPath := range localPaths {
		if err := os.WriteFile(localPath, []byte(filepath.Base(localPath)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := service.Upload(context.Background(), localPaths, "~")
	if err != nil {
		t.Fatal(err)
	}

	wantUploaded := []string{"/home/eric/before.txt", "/home/eric/after.txt"}
	if !reflect.DeepEqual(result.Uploaded, wantUploaded) {
		t.Fatalf("uploaded paths = %#v, want %#v", result.Uploaded, wantUploaded)
	}
	wantFailures := []UploadFailure{{Name: "README.md", Code: UploadFailureExists}}
	if !reflect.DeepEqual(result.Failures, wantFailures) {
		t.Fatalf("upload failures = %#v, want %#v", result.Failures, wantFailures)
	}
	if got := string(service.fs.(*memoryFS).nodes["/home/eric/README.md"].content); got != "# Hello" {
		t.Fatalf("existing remote content = %q, want original content", got)
	}
}

func TestUploadRejectsLocalDirectories(t *testing.T) {
	service := connectedTestService(t)
	localDirectory := t.TempDir()

	result, err := service.Upload(context.Background(), []string{localDirectory}, "~")
	if err != nil {
		t.Fatal(err)
	}

	want := UploadFailure{Name: filepath.Base(localDirectory), Code: UploadFailureDirectory}
	if len(result.Uploaded) != 0 || !reflect.DeepEqual(result.Failures, []UploadFailure{want}) {
		t.Fatalf("Upload() = %#v, want directory failure", result)
	}
}

func TestUploadReportsRemotePermissionDenials(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(localPath, []byte("<h1>Hello</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	remoteFS := service.fs.(*memoryFS)
	remoteFS.openFileErrorName = "index.html"
	remoteFS.openFileError = os.ErrPermission

	result, err := service.Upload(context.Background(), []string{localPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	want := []UploadFailure{{Name: "index.html", Code: UploadFailurePermission}}
	if len(result.Uploaded) != 0 || !reflect.DeepEqual(result.Failures, want) {
		t.Fatalf("Upload() = %#v, want permission failure", result)
	}
	if _, exists := remoteFS.nodes["/home/eric/index.html"]; exists {
		t.Fatal("permission-denied upload created a remote file")
	}
}

func TestUploadReportsPermissionSettingFailuresWithoutLeavingAFile(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "secret.pem")
	if err := os.WriteFile(localPath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	remoteFS := service.fs.(*memoryFS)
	remoteFS.chmodErrorName = "secret.pem"
	remoteFS.chmodError = os.ErrPermission

	result, err := service.Upload(context.Background(), []string{localPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	want := []UploadFailure{{Name: "secret.pem", Code: UploadFailurePermissions}}
	if len(result.Uploaded) != 0 || !reflect.DeepEqual(result.Failures, want) {
		t.Fatalf("Upload() = %#v, want permission-setting failure", result)
	}
	if _, exists := remoteFS.nodes["/home/eric/secret.pem"]; exists {
		t.Fatal("permission-setting failure left a remote file")
	}
}

func TestUploadFollowsLocalFileSymlinks(t *testing.T) {
	service := connectedTestService(t)
	localDirectory := t.TempDir()
	target := filepath.Join(localDirectory, "target.txt")
	link := filepath.Join(localDirectory, "linked.txt")
	if err := os.WriteFile(target, []byte("linked contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("create local symlink: %v", err)
	}

	result, err := service.Upload(context.Background(), []string{link}, "~")
	if err != nil {
		t.Fatal(err)
	}

	wantPath := "/home/eric/linked.txt"
	if !reflect.DeepEqual(result.Uploaded, []string{wantPath}) || len(result.Failures) != 0 {
		t.Fatalf("Upload() = %#v, want uploaded symlink target", result)
	}
	if got := string(service.fs.(*memoryFS).nodes[wantPath].content); got != "linked contents" {
		t.Fatalf("uploaded content = %q", got)
	}
}

func TestUploadAcceptsASymlinkedRemoteDirectory(t *testing.T) {
	service := connectedTestService(t)
	remoteFS := service.fs.(*memoryFS)
	remoteFS.nodes["/home/eric/current"] = memoryNode{
		name: "current",
		mode: os.ModeSymlink | 0o777,
	}
	remoteFS.symlinkTargets["/home/eric/current"] = "/home/eric/Projects"
	localPath := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(localPath, []byte("contents"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := service.Upload(context.Background(), []string{localPath}, "~/current")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result.Uploaded, []string{"/home/eric/current/report.txt"}) || len(result.Failures) != 0 {
		t.Fatalf("Upload() = %#v, want symlinked destination upload", result)
	}
}

func TestUploadRejectsASymlinkedRemoteFileAsADestination(t *testing.T) {
	service := connectedTestService(t)
	remoteFS := service.fs.(*memoryFS)
	remoteFS.nodes["/home/eric/current"] = memoryNode{
		name: "current",
		mode: os.ModeSymlink | 0o777,
	}
	remoteFS.symlinkTargets["/home/eric/current"] = "/home/eric/README.md"
	localPath := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(localPath, []byte("contents"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := service.Upload(context.Background(), []string{localPath}, "~/current")

	if err == nil {
		t.Fatalf("Upload() = %#v, nil; want non-directory destination error", result)
	}
	if len(result.Uploaded) != 0 || len(result.Failures) != 0 {
		t.Fatalf("Upload() = %#v, want no attempted files", result)
	}
}

func TestUploadRemovesAFileWhenCopyingIsCancelled(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "cancelled.txt")
	if err := os.WriteFile(localPath, []byte("partial contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.fs.(*memoryFS).onOpenFile = cancel

	_, err := service.Upload(ctx, []string{localPath}, "~")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Upload() error = %v, want context canceled", err)
	}
	if _, exists := service.fs.(*memoryFS).nodes["/home/eric/cancelled.txt"]; exists {
		t.Fatal("partially uploaded remote file was not removed")
	}
}

func TestUploadRemovesAPartialFileAndContinuesAfterARemoteWriteFailure(t *testing.T) {
	service := connectedTestService(t)
	localDirectory := t.TempDir()
	brokenPath := filepath.Join(localDirectory, "broken.txt")
	afterPath := filepath.Join(localDirectory, "after.txt")
	if err := os.WriteFile(brokenPath, []byte("partial contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterPath, []byte("complete contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	service.fs.(*memoryFS).failWriteName = "broken.txt"

	result, err := service.Upload(context.Background(), []string{brokenPath, afterPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	wantFailures := []UploadFailure{{Name: "broken.txt", Code: UploadFailureFailed}}
	if !reflect.DeepEqual(result.Failures, wantFailures) {
		t.Fatalf("upload failures = %#v, want %#v", result.Failures, wantFailures)
	}
	if !reflect.DeepEqual(result.Uploaded, []string{"/home/eric/after.txt"}) {
		t.Fatalf("uploaded paths = %#v", result.Uploaded)
	}
	remoteFS := service.fs.(*memoryFS)
	if _, exists := remoteFS.nodes["/home/eric/broken.txt"]; exists {
		t.Fatal("partially uploaded remote file was not removed")
	}
	if got := string(remoteFS.nodes["/home/eric/after.txt"].content); got != "complete contents" {
		t.Fatalf("later uploaded content = %q", got)
	}
}

func TestUploadReportsAnIncompleteRemoteFileWhenCleanupFails(t *testing.T) {
	service := connectedTestService(t)
	localPath := filepath.Join(t.TempDir(), "broken.txt")
	if err := os.WriteFile(localPath, []byte("partial contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	remoteFS := service.fs.(*memoryFS)
	remoteFS.failWriteName = "broken.txt"
	remoteFS.removeErrorName = "broken.txt"
	remoteFS.removeError = errors.New("connection closed")

	result, err := service.Upload(context.Background(), []string{localPath}, "~")
	if err != nil {
		t.Fatal(err)
	}

	wantFailures := []UploadFailure{{Name: "broken.txt", Code: UploadFailureIncomplete}}
	if len(result.Uploaded) != 0 || !reflect.DeepEqual(result.Failures, wantFailures) {
		t.Fatalf("Upload() = %#v, want incomplete-file failure", result)
	}
	if _, exists := remoteFS.nodes["/home/eric/broken.txt"]; !exists {
		t.Fatal("cleanup-failure test did not retain the incomplete remote file")
	}
}

func TestCreateFolderAndRenameRemoteItems(t *testing.T) {
	service := connectedTestService(t)

	folderPath, err := service.CreateFolder("~", "Release notes")
	if err != nil {
		t.Fatal(err)
	}
	if folderPath != "/home/eric/Release notes" {
		t.Fatalf("CreateFolder() = %q", folderPath)
	}

	renamedPath, err := service.Rename("/home/eric/README.md", "README-old.md")
	if err != nil {
		t.Fatal(err)
	}
	if renamedPath != "/home/eric/README-old.md" {
		t.Fatalf("Rename() = %q", renamedPath)
	}
	remoteFS := service.fs.(*memoryFS)
	if _, exists := remoteFS.nodes["/home/eric/README.md"]; exists {
		t.Fatal("original file still exists after rename")
	}
	if got := string(remoteFS.nodes[renamedPath].content); got != "# Hello" {
		t.Fatalf("renamed content = %q", got)
	}
}

func TestCopyCreatesRecursiveCollisionSafeDuplicates(t *testing.T) {
	service := connectedTestService(t)

	paths, err := service.Copy([]string{
		"/home/eric/Projects",
		"/home/eric/README.md",
	}, "/home/eric")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/eric/Projects copy", "/home/eric/README copy.md"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("Copy() = %#v, want %#v", paths, want)
	}
	remoteFS := service.fs.(*memoryFS)
	if got := string(remoteFS.nodes["/home/eric/Projects copy/a"].content); got != "alpha" {
		t.Fatalf("copied nested content = %q", got)
	}
	if got := remoteFS.nodes["/home/eric/README copy.md"].mode.Perm(); got != 0o644 {
		t.Fatalf("copied permissions = %o", got)
	}
}

func TestCopyRejectsSymbolicLinksWithoutCreatingATarget(t *testing.T) {
	service := connectedTestService(t)
	remoteFS := service.fs.(*memoryFS)
	remoteFS.nodes["/home/eric/latest"] = memoryNode{
		name:    "latest",
		mode:    os.ModeSymlink | 0o777,
		modTime: time.Now(),
	}
	if _, err := service.CreateFolder("~", "Archive"); err != nil {
		t.Fatal(err)
	}

	_, err := service.Copy([]string{"/home/eric/latest"}, "/home/eric/Archive")

	if !errors.Is(err, ErrUnsupportedSymlink) {
		t.Fatalf("Copy() error = %v, want ErrUnsupportedSymlink", err)
	}
	if _, exists := remoteFS.nodes["/home/eric/Archive/latest"]; exists {
		t.Fatal("Copy() created a target for an unsupported symbolic link")
	}
}

func TestMoveUsesTheDestinationAndPreservesDirectoryContents(t *testing.T) {
	service := connectedTestService(t)
	if _, err := service.CreateFolder("~", "Archive"); err != nil {
		t.Fatal(err)
	}

	paths, err := service.Move([]string{"/home/eric/Projects"}, "/home/eric/Archive")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/eric/Archive/Projects"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("Move() = %#v, want %#v", paths, want)
	}
	remoteFS := service.fs.(*memoryFS)
	if _, exists := remoteFS.nodes["/home/eric/Projects"]; exists {
		t.Fatal("source folder still exists after move")
	}
	if got := string(remoteFS.nodes["/home/eric/Archive/Projects/a"].content); got != "alpha" {
		t.Fatalf("moved nested content = %q", got)
	}
}

func TestDeleteRecursivelyRemovesItemsButProtectsRootAndHome(t *testing.T) {
	service := connectedTestService(t)

	if err := service.Delete([]string{"/home/eric/Projects"}); err != nil {
		t.Fatal(err)
	}
	remoteFS := service.fs.(*memoryFS)
	for nodePath := range remoteFS.nodes {
		if strings.HasPrefix(nodePath, "/home/eric/Projects") {
			t.Fatalf("deleted tree still contains %q", nodePath)
		}
	}
	for _, protected := range []string{"/", "/home/eric"} {
		if err := service.Delete([]string{protected}); !errors.Is(err, ErrProtectedPath) {
			t.Fatalf("Delete(%q) error = %v, want ErrProtectedPath", protected, err)
		}
	}
}

func TestCopyAndMoveRejectDestinationsInsideTheSource(t *testing.T) {
	service := connectedTestService(t)

	if _, err := service.Copy([]string{"/home/eric/Projects"}, "/home/eric/Projects"); err == nil {
		t.Fatal("Copy() into the source unexpectedly succeeded")
	}
	if _, err := service.Move([]string{"/home/eric/Projects"}, "/home/eric/Projects"); err == nil {
		t.Fatal("Move() into the source unexpectedly succeeded")
	}
}

func TestDownloadCopiesFoldersAndChoosesANonOverwritingTarget(t *testing.T) {
	service := connectedTestService(t)
	destination := t.TempDir()
	if err := os.Mkdir(filepath.Join(destination, "Projects"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := service.Download(context.Background(), []string{"/home/eric/Projects"}, destination)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget := filepath.Join(destination, "Projects (1)")
	if !reflect.DeepEqual(paths, []string{wantTarget}) {
		t.Fatalf("download paths = %#v, want %q", paths, wantTarget)
	}
	content, err := os.ReadFile(filepath.Join(wantTarget, "a"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "alpha" {
		t.Fatalf("downloaded content = %q", content)
	}
}

func TestDownloadHonorsCancellationBeforeWriting(t *testing.T) {
	service := connectedTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Download(ctx, []string{"README.md"}, t.TempDir())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Download() error = %v, want canceled", err)
	}
}

func TestContentMiddlewareStreamsRemoteFilesAndSandboxesHTML(t *testing.T) {
	service := connectedTestService(t)
	service.fs.(*memoryFS).nodes["/home/eric/index.html"] = memoryNode{
		name: "index.html", mode: 0o644, content: []byte("<h1>Hello</h1>"), modTime: time.Now(),
	}
	handler := service.ContentMiddleware(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, ContentPathPrefix+"/home/eric/index.html", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "<h1>Hello</h1>" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if value := response.Header().Get("Content-Security-Policy"); value == "" {
		t.Fatal("HTML previews must be sandboxed")
	}
}

func TestRemoteDownloadEntryNamesCannotEscapeTheDestination(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../outside", "nested/file"} {
		if err := validateRemoteEntryName(name); err == nil {
			t.Fatalf("validateRemoteEntryName(%q) unexpectedly succeeded", name)
		}
	}
	if err := validateRemoteEntryName("safe file.txt"); err != nil {
		t.Fatalf("safe filename rejected: %v", err)
	}
}
