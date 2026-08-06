//go:build windows

package menubar

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"fyne.io/systray"

	buildassets "ssh-man/build"
)

const (
	windowsTrayStartTimeout = 5 * time.Second
	windowsTrayStopTimeout  = 2 * time.Second
)

type windowsTrayCallbacks struct {
	Open func()
	Quit func()
}

type windowsTray interface {
	Start([]byte, windowsTrayCallbacks) error
	Stop()
}

type windowsService struct {
	mu         sync.Mutex
	tray       windowsTray
	icon       []byte
	startupErr error
	show       func()
	quit       func()
	started    bool
}

func New(callbacks Callbacks) Service {
	icon, err := windowsIconFromPNG(buildassets.ApplicationIconPNG())
	if err != nil {
		err = fmt.Errorf("prepare Windows tray icon: %w", err)
	}
	return newWindowsService(callbacks, &fyneWindowsTray{}, icon, err)
}

func newWindowsService(callbacks Callbacks, tray windowsTray, icon []byte, startupErr error) *windowsService {
	return &windowsService{
		tray:       tray,
		icon:       append([]byte(nil), icon...),
		startupErr: startupErr,
		show:       callbacks.Show,
		quit:       callbacks.Quit,
	}
}

func (s *windowsService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if s.startupErr != nil {
		return s.startupErr
	}
	if s.tray == nil {
		return errors.New("Windows tray runtime is unavailable")
	}
	if err := s.tray.Start(s.icon, windowsTrayCallbacks{
		Open: s.show,
		Quit: s.quit,
	}); err != nil {
		return err
	}
	s.started = true
	return nil
}

func (s *windowsService) Show() bool {
	s.mu.Lock()
	started := s.started
	show := s.show
	s.mu.Unlock()
	if !started || show == nil {
		return false
	}
	show()
	return true
}

func (s *windowsService) ShowBrowserSwitcher() bool {
	return false
}

func (s *windowsService) CancelBrowserSwitchSession() {}

func (s *windowsService) SetBrowserShortcuts(string, string) error {
	return nil
}

func (s *windowsService) ShouldHideWindowOnClose() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *windowsService) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	tray := s.tray
	s.mu.Unlock()

	tray.Stop()
}

type fyneWindowsTray struct {
	mu      sync.Mutex
	running bool
	exited  chan struct{}
}

func (t *fyneWindowsTray) Start(icon []byte, callbacks windowsTrayCallbacks) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return nil
	}
	started := make(chan error, 1)
	exited := make(chan struct{})
	t.running = true
	t.exited = exited
	t.mu.Unlock()

	go t.run(icon, callbacks, started, exited)

	timer := time.NewTimer(windowsTrayStartTimeout)
	defer timer.Stop()
	select {
	case err := <-started:
		if err != nil {
			t.Stop()
		}
		return err
	case <-exited:
		return errors.New("Windows tray stopped before startup completed")
	case <-timer.C:
		t.Stop()
		return errors.New("Windows tray startup timed out")
	}
}

func (t *fyneWindowsTray) run(
	icon []byte,
	callbacks windowsTrayCallbacks,
	started chan<- error,
	exited chan struct{},
) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var exitOnce sync.Once

	systray.Run(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				started <- fmt.Errorf("start Windows tray: %v", recovered)
			}
		}()

		if err := installWindowsTrayIcon(icon, systray.SetIconFromFilePath); err != nil {
			started <- err
			return
		}
		systray.SetTooltip("SSH Man")
		systray.SetOnTapped(func() {
			dispatchWindowsTrayCallback(callbacks.Open)
		})

		openItem := systray.AddMenuItem("Open SSH Man", "Show the SSH Man window")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit SSH Man", "Stop sessions and quit SSH Man")
		go listenForWindowsTrayActions(openItem.ClickedCh, quitItem.ClickedCh, callbacks, exited)
		started <- nil
	}, func() {
		t.completeExit(exited, &exitOnce)
	})
}

func (t *fyneWindowsTray) completeExit(exited chan struct{}, exitOnce *sync.Once) {
	exitOnce.Do(func() {
		t.mu.Lock()
		if t.exited == exited {
			t.running = false
		}
		t.mu.Unlock()
		close(exited)
	})
}

func (t *fyneWindowsTray) Stop() {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return
	}
	exited := t.exited
	t.mu.Unlock()

	systray.Quit()

	timer := time.NewTimer(windowsTrayStopTimeout)
	defer timer.Stop()
	select {
	case <-exited:
	case <-timer.C:
	}
}

func listenForWindowsTrayActions(
	open <-chan struct{},
	quit <-chan struct{},
	callbacks windowsTrayCallbacks,
	exited <-chan struct{},
) {
	for {
		select {
		case <-open:
			dispatchWindowsTrayCallback(callbacks.Open)
		case <-quit:
			dispatchWindowsTrayCallback(callbacks.Quit)
		case <-exited:
			return
		}
	}
}

func dispatchWindowsTrayCallback(callback func()) {
	if callback != nil {
		go callback()
	}
}

func installWindowsTrayIcon(icon []byte, install func(string) error) error {
	if install == nil {
		return errors.New("Windows tray icon installer is unavailable")
	}

	file, err := os.CreateTemp("", "ssh-man-tray-*.ico")
	if err != nil {
		return fmt.Errorf("create Windows tray icon file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)

	if _, err := file.Write(icon); err != nil {
		_ = file.Close()
		return fmt.Errorf("write Windows tray icon file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Windows tray icon file: %w", err)
	}
	if err := install(path); err != nil {
		return fmt.Errorf("load Windows tray icon: %w", err)
	}
	return nil
}

func windowsIconFromPNG(data []byte) ([]byte, error) {
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	if config.Width < 1 || config.Height < 1 {
		return nil, errors.New("PNG dimensions must be positive")
	}
	if config.Width > 256 || config.Height > 256 {
		data, config, err = resizeWindowsTrayPNG(data, config)
		if err != nil {
			return nil, err
		}
	}
	if len(data) > math.MaxUint32-22 {
		return nil, errors.New("PNG icon is too large")
	}

	icon := make([]byte, 22+len(data))
	binary.LittleEndian.PutUint16(icon[2:4], 1)
	binary.LittleEndian.PutUint16(icon[4:6], 1)
	if config.Width < 256 {
		icon[6] = byte(config.Width)
	}
	if config.Height < 256 {
		icon[7] = byte(config.Height)
	}
	binary.LittleEndian.PutUint16(icon[10:12], 1)
	binary.LittleEndian.PutUint16(icon[12:14], 32)
	binary.LittleEndian.PutUint32(icon[14:18], uint32(len(data)))
	binary.LittleEndian.PutUint32(icon[18:22], 22)
	copy(icon[22:], data)
	return icon, nil
}

func resizeWindowsTrayPNG(data []byte, config image.Config) ([]byte, image.Config, error) {
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, image.Config{}, fmt.Errorf("decode PNG pixels: %w", err)
	}

	width := config.Width
	height := config.Height
	if width >= height {
		height = max(1, height*256/width)
		width = 256
	} else {
		width = max(1, width*256/height)
		height = 256
	}

	sourceBounds := source.Bounds()
	resized := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		sourceY := sourceBounds.Min.Y + y*sourceBounds.Dy()/height
		for x := range width {
			sourceX := sourceBounds.Min.X + x*sourceBounds.Dx()/width
			resized.SetNRGBA(x, y, color.NRGBAModel.Convert(source.At(sourceX, sourceY)).(color.NRGBA))
		}
	}

	var output bytes.Buffer
	if err := png.Encode(&output, resized); err != nil {
		return nil, image.Config{}, fmt.Errorf("encode resized PNG: %w", err)
	}
	return output.Bytes(), image.Config{
		ColorModel: color.NRGBAModel,
		Width:      width,
		Height:     height,
	}, nil
}
