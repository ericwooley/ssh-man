package remote

import (
	"bytes"
	"context"
	"errors"
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
	bytes.Buffer
	fs     *memoryFS
	name   string
	mode   os.FileMode
	closed bool
}

func (f *memoryWriteFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	f.fs.nodes[f.name] = memoryNode{
		name:    path.Base(f.name),
		content: append([]byte(nil), f.Bytes()...),
		mode:    f.mode,
		modTime: time.Now(),
	}
	return nil
}

type memoryFS struct {
	home  string
	nodes map[string]memoryNode
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
	return &memoryWriteFile{fs: f, name: name, mode: mode}, nil
}
func (f *memoryFS) Chmod(name string, mode os.FileMode) error {
	name = cleanRemotePath(name)
	node, exists := f.nodes[name]
	if !exists {
		return os.ErrNotExist
	}
	node.mode = mode
	f.nodes[name] = node
	return nil
}
func (f *memoryFS) PosixRename(oldName, newName string) error {
	oldName = cleanRemotePath(oldName)
	newName = cleanRemotePath(newName)
	node, exists := f.nodes[oldName]
	if !exists {
		return os.ErrNotExist
	}
	delete(f.nodes, oldName)
	node.name = path.Base(newName)
	f.nodes[newName] = node
	return nil
}
func (f *memoryFS) Remove(name string) error {
	name = cleanRemotePath(name)
	if _, exists := f.nodes[name]; !exists {
		return os.ErrNotExist
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
		home: "/home/eric",
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
