package appupdate

import (
	"context"
	"net/http"
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
}

func (*recordingInstaller) supported() bool {
	return true
}

func (i *recordingInstaller) stage(context.Context, *Client, *updatePlan, string) (*stagedUpdate, error) {
	return nil, nil
}

func (i *recordingInstaller) prepare(*stagedUpdate, int) error {
	i.prepared = true
	return nil
}

func (i *recordingInstaller) cleanup(*stagedUpdate) error {
	i.cleaned = true
	return nil
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
