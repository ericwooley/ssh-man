package appupdate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type recordingInstaller struct {
	prepared bool
	cleaned  bool
	staged   *stagedUpdate
}

func (*recordingInstaller) supported() bool {
	return true
}

func (i *recordingInstaller) stage(context.Context, *Client, *updatePlan, string) (*stagedUpdate, error) {
	return i.staged, nil
}

func (i *recordingInstaller) prepare(*stagedUpdate, int) error {
	i.prepared = true
	return nil
}

func (i *recordingInstaller) cleanup(*stagedUpdate) error {
	i.cleaned = true
	return nil
}

func TestManagerPublishesReadyExperimentalUpdate(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{
			"tag_name":"1.2.0",
			"draft":false,
			"prerelease":true,
			"assets":[{
				"name":"ssh-man.dmg",
				"size":42,
				"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"browser_download_url":%q
			}]
		}]`, server.URL+"/ssh-man.dmg")
	}))
	t.Cleanup(server.Close)

	installer := &recordingInstaller{
		staged: &stagedUpdate{Version: "1.2.0", RootPath: "/tmp/update"},
	}
	manager := &Manager{
		currentVersion: "1.0.0",
		client: &Client{
			httpClient:       server.Client(),
			latestReleaseURL: server.URL + "/latest",
			releasesURL:      server.URL,
			allowDownloadURL: func(rawURL string) bool { return strings.HasPrefix(rawURL, server.URL) },
		},
		installer: installer,
	}
	statuses := make(chan Status, 8)
	manager.SetStatusObserver(func(status Status) {
		statuses <- status
	})

	manager.Configure(true, true)

	var ready Status
	for ready.State != StateReady {
		select {
		case ready = <-statuses:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for a ready update")
		}
	}
	if ready.Version != "1.2.0" || ready.Channel != ChannelExperimental {
		t.Fatalf("ready status = %#v", ready)
	}
	if err := manager.Install(); err != nil {
		t.Fatalf("install ready update: %v", err)
	}
	if err := manager.StopAndPrepare(true, 123); err != nil {
		t.Fatalf("prepare ready update: %v", err)
	}
	if !installer.prepared {
		t.Fatal("one-click update did not prepare the ready update")
	}
}

func TestStopAndPrepareHonorsAutomaticUpdateOptOut(t *testing.T) {
	installer := &recordingInstaller{}
	manager := &Manager{
		installer: installer,
		staged:    &stagedUpdate{Version: "1.2.3", RootPath: "/tmp/update"},
	}

	if err := manager.StopAndPrepare(false, 123); err != nil {
		t.Fatalf("stop updater: %v", err)
	}
	if !installer.cleaned {
		t.Fatal("disabled updater did not clean the staged update")
	}
	if installer.prepared {
		t.Fatal("disabled updater prepared an installation")
	}
}

func TestSetEnabledImmediatelyDiscardsStagedUpdate(t *testing.T) {
	installer := &recordingInstaller{}
	manager := &Manager{
		enabled:   true,
		installer: installer,
		staged:    &stagedUpdate{Version: "1.2.3", RootPath: "/tmp/update"},
	}

	manager.SetEnabled(false)

	if !installer.cleaned {
		t.Fatal("turning off automatic updates did not clean the staged update")
	}
	if manager.staged != nil {
		t.Fatalf("staged update = %#v, want nil", manager.staged)
	}
}

func TestSetEnabledRestartsCheckAfterDisableAndReenable(t *testing.T) {
	attempts := make(chan struct{}, 2)
	client := &Client{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				attempts <- struct{}{}
				<-request.Context().Done()
				return nil, request.Context().Err()
			}),
		},
		latestReleaseURL: "https://example.com/latest",
	}
	manager := &Manager{
		currentVersion: "1.0.0",
		client:         client,
		installer:      &recordingInstaller{},
	}
	defer func() {
		manager.SetEnabled(false)
		manager.wait.Wait()
	}()

	manager.SetEnabled(true)
	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("initial automatic update check did not start")
	}

	manager.SetEnabled(false)
	manager.SetEnabled(true)

	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("automatic update check did not restart after re-enabling")
	}
}

func TestStopAndPrepareLaunchesEnabledAutomaticUpdate(t *testing.T) {
	installer := &recordingInstaller{}
	manager := &Manager{
		installer: installer,
		staged:    &stagedUpdate{Version: "1.2.3", RootPath: "/tmp/update"},
	}

	if err := manager.StopAndPrepare(true, 123); err != nil {
		t.Fatalf("stop updater: %v", err)
	}
	if !installer.prepared {
		t.Fatal("enabled updater did not prepare the staged installation")
	}
	if installer.cleaned {
		t.Fatal("enabled updater cleaned the staged update before installation")
	}
}

func TestRunHelperLeavesNormalApplicationArgumentsAlone(t *testing.T) {
	handled, err := RunHelper([]string{"version", "--output", "json"})
	if err != nil {
		t.Fatalf("run helper: %v", err)
	}
	if handled {
		t.Fatal("normal application arguments were treated as updater arguments")
	}
}
