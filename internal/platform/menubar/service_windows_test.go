//go:build windows

package menubar

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/png"
	"os"
	"strings"
	"testing"

	buildassets "ssh-man/build"
)

type fakeWindowsTray struct {
	startErr  error
	starts    int
	stops     int
	icon      []byte
	callbacks windowsTrayCallbacks
}

func (f *fakeWindowsTray) Start(icon []byte, callbacks windowsTrayCallbacks) error {
	f.starts++
	f.icon = append([]byte(nil), icon...)
	f.callbacks = callbacks
	return f.startErr
}

func (f *fakeWindowsTray) Stop() {
	f.stops++
}

func TestWindowsServiceRoutesTrayActions(t *testing.T) {
	tray := &fakeWindowsTray{}
	shows := 0
	quits := 0
	service := newWindowsService(Callbacks{
		Show: func() { shows++ },
		Quit: func() { quits++ },
	}, tray, []byte("icon"), nil)

	if service.Show() {
		t.Fatal("Show() = true before Start()")
	}
	if service.ShouldHideWindowOnClose() {
		t.Fatal("ShouldHideWindowOnClose() = true before Start()")
	}
	if err := service.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := service.Start(); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	if tray.starts != 1 {
		t.Fatalf("tray starts = %d, want 1", tray.starts)
	}
	if !service.ShouldHideWindowOnClose() {
		t.Fatal("ShouldHideWindowOnClose() = false after Start()")
	}
	if !bytes.Equal(tray.icon, []byte("icon")) {
		t.Fatalf("tray icon = %q, want icon", tray.icon)
	}

	if !service.Show() {
		t.Fatal("Show() = false after Start()")
	}
	tray.callbacks.Open()
	tray.callbacks.Quit()
	if shows != 2 {
		t.Fatalf("show callbacks = %d, want 2", shows)
	}
	if quits != 1 {
		t.Fatalf("quit callbacks = %d, want 1", quits)
	}

	service.Stop()
	service.Stop()
	if tray.stops != 1 {
		t.Fatalf("tray stops = %d, want 1", tray.stops)
	}
	if service.Show() {
		t.Fatal("Show() = true after Stop()")
	}
	if service.ShouldHideWindowOnClose() {
		t.Fatal("ShouldHideWindowOnClose() = true after Stop()")
	}
}

func TestWindowsServiceReportsTrayStartupFailure(t *testing.T) {
	wantErr := errors.New("tray failed")
	tray := &fakeWindowsTray{startErr: wantErr}
	service := newWindowsService(Callbacks{Show: func() {}}, tray, []byte("icon"), nil)

	if err := service.Start(); !errors.Is(err, wantErr) {
		t.Fatalf("Start() error = %v, want %v", err, wantErr)
	}
	if service.Show() {
		t.Fatal("Show() = true after failed startup")
	}
	service.Stop()
	if tray.stops != 0 {
		t.Fatalf("tray stops = %d, want 0", tray.stops)
	}
}

func TestWindowsServiceReportsIconFailure(t *testing.T) {
	wantErr := errors.New("icon failed")
	tray := &fakeWindowsTray{}
	service := newWindowsService(Callbacks{}, tray, nil, wantErr)

	if err := service.Start(); !errors.Is(err, wantErr) {
		t.Fatalf("Start() error = %v, want %v", err, wantErr)
	}
	if tray.starts != 0 {
		t.Fatalf("tray starts = %d, want 0", tray.starts)
	}
}

func TestWindowsIconFromPNGBuildsICOResource(t *testing.T) {
	pngData := buildassets.ApplicationIconPNG()
	icon, err := windowsIconFromPNG(pngData)
	if err != nil {
		t.Fatalf("windowsIconFromPNG() error = %v", err)
	}
	icoPNG := icon[22:]
	config, err := png.DecodeConfig(bytes.NewReader(icoPNG))
	if err != nil {
		t.Fatalf("decode ICO PNG: %v", err)
	}
	if config.Width > 256 || config.Height > 256 {
		t.Fatalf("ICO PNG dimensions = %dx%d, want at most 256x256", config.Width, config.Height)
	}
	if binary.LittleEndian.Uint16(icon[2:4]) != 1 {
		t.Fatal("ICO type is not icon")
	}
	if binary.LittleEndian.Uint16(icon[4:6]) != 1 {
		t.Fatal("ICO image count is not 1")
	}
	if got := iconDimension(icon[6]); got != config.Width {
		t.Fatalf("ICO width = %d, want %d", got, config.Width)
	}
	if got := iconDimension(icon[7]); got != config.Height {
		t.Fatalf("ICO height = %d, want %d", got, config.Height)
	}
	if got := binary.LittleEndian.Uint32(icon[14:18]); got != uint32(len(icoPNG)) {
		t.Fatalf("ICO data length = %d, want %d", got, len(icoPNG))
	}
	if got := binary.LittleEndian.Uint32(icon[18:22]); got != 22 {
		t.Fatalf("ICO data offset = %d, want 22", got)
	}
}

func TestWindowsIconFromPNGRejectsInvalidData(t *testing.T) {
	if _, err := windowsIconFromPNG([]byte("not png")); err == nil {
		t.Fatal("windowsIconFromPNG() error = nil for invalid PNG")
	}
}

func TestInstallWindowsTrayIconReportsNativeLoadFailure(t *testing.T) {
	wantErr := errors.New("native icon load failed")
	var iconPath string
	err := installWindowsTrayIcon([]byte("icon"), func(path string) error {
		iconPath = path
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat temporary icon: %v", err)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("installWindowsTrayIcon() error = %v, want %v", err, wantErr)
	}
	if _, err := os.Stat(iconPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary icon remains after installation: %v", err)
	}
}

func TestFyneWindowsTrayStartReportsNativeIconLoadFailure(t *testing.T) {
	tray := &fyneWindowsTray{}
	err := tray.Start([]byte("not an icon"), windowsTrayCallbacks{})
	if err == nil || !strings.Contains(err.Error(), "load Windows tray icon") {
		t.Fatalf("Start() error = %v, want native icon load error", err)
	}
}

func iconDimension(value byte) int {
	if value == 0 {
		return 256
	}
	return int(value)
}
