//go:build darwin && cgo

package menubar

/*
#cgo LDFLAGS: -framework Cocoa -framework Carbon
#include "native_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"

	"ssh-man/internal/keyboardshortcut"
)

var (
	callbackMu                 sync.RWMutex
	quitHandler                func()
	browserSwitchHandler       func(BrowserSwitchDirection, uint64)
	browserSwitchCommitHandler func(uint64)
	browserSwitchCancelHandler func(uint64)
)

type darwinService struct {
	mu                  sync.Mutex
	started             bool
	quit                func()
	switchBrowsers      func(BrowserSwitchDirection, uint64)
	commitBrowserSwitch func(uint64)
	cancelBrowserSwitch func(uint64)
}

func shouldDismissPopupForOutsideClick(applicationActive bool, popupVisible bool) bool {
	active := C.int(0)
	if applicationActive {
		active = 1
	}
	visible := C.int(0)
	if popupVisible {
		visible = 1
	}
	return C.SSHManMenuBarShouldDismissPopupForOutsideClick(active, visible) != 0
}

func New(callbacks Callbacks) Service {
	return &darwinService{
		quit:                callbacks.Quit,
		switchBrowsers:      callbacks.SwitchBrowsers,
		commitBrowserSwitch: callbacks.CommitBrowserSwitch,
		cancelBrowserSwitch: callbacks.CancelBrowserSwitch,
	}
}

func (s *darwinService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}

	callbackMu.Lock()
	quitHandler = s.quit
	browserSwitchHandler = s.switchBrowsers
	browserSwitchCommitHandler = s.commitBrowserSwitch
	browserSwitchCancelHandler = s.cancelBrowserSwitch
	callbackMu.Unlock()

	if C.SSHManMenuBarStart() == 0 {
		callbackMu.Lock()
		quitHandler = nil
		browserSwitchHandler = nil
		browserSwitchCommitHandler = nil
		browserSwitchCancelHandler = nil
		callbackMu.Unlock()
		return fmt.Errorf("create macOS menu-bar item")
	}

	s.started = true
	return nil
}

func (s *darwinService) ShowBrowserSwitcher() bool {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	return started && C.SSHManMenuBarShowBrowserSwitcher() != 0
}

func (s *darwinService) CancelBrowserSwitchSession() {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if started {
		C.SSHManMenuBarCancelBrowserSwitchSession()
	}
}

func (s *darwinService) SetBrowserShortcuts(forwardValue string, backwardValue string) error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return fmt.Errorf("macOS menu-bar integration is not running")
	}

	forward, backward, err := parseDarwinBrowserShortcuts(forwardValue, backwardValue)
	if err != nil {
		return err
	}
	if C.SSHManMenuBarSetBrowserShortcuts(
		C.uint(forward.keyCode),
		C.uint(forward.modifiers),
		C.uint(backward.keyCode),
		C.uint(backward.modifiers),
	) == 0 {
		return fmt.Errorf(
			"register global browser shortcuts %s and %s; one may already be in use",
			forward.canonical,
			backward.canonical,
		)
	}
	return nil
}

type darwinShortcut struct {
	canonical string
	keyCode   uint32
	modifiers uint32
	shortcut  keyboardshortcut.Shortcut
}

func parseDarwinBrowserShortcuts(forwardValue string, backwardValue string) (darwinShortcut, darwinShortcut, error) {
	forward, err := parseDarwinShortcut(forwardValue)
	if err != nil {
		return darwinShortcut{}, darwinShortcut{}, fmt.Errorf("forward browser shortcut: %w", err)
	}
	backward, err := parseDarwinShortcut(backwardValue)
	if err != nil {
		return darwinShortcut{}, darwinShortcut{}, fmt.Errorf("backward browser shortcut: %w", err)
	}
	if forward.canonical == backward.canonical {
		return darwinShortcut{}, darwinShortcut{}, fmt.Errorf("forward and backward browser shortcuts must be different")
	}
	if !keyboardshortcut.SameHeldModifiers(forward.shortcut, backward.shortcut) {
		return darwinShortcut{}, darwinShortcut{}, fmt.Errorf("forward and backward browser shortcuts must use the same Control, Alt, and Command modifiers")
	}
	return forward, backward, nil
}

func parseDarwinShortcut(value string) (darwinShortcut, error) {
	shortcut, err := keyboardshortcut.Parse(value)
	if err != nil {
		return darwinShortcut{}, err
	}
	keyCode, ok := darwinKeyCodes[shortcut.Key]
	if !ok {
		return darwinShortcut{}, fmt.Errorf("shortcut key %q is unavailable on macOS", shortcut.Key)
	}

	var modifiers uint32
	if shortcut.Meta {
		modifiers |= carbonCommandKey
	}
	if shortcut.Shift {
		modifiers |= carbonShiftKey
	}
	if shortcut.Alt {
		modifiers |= carbonOptionKey
	}
	if shortcut.Control {
		modifiers |= carbonControlKey
	}
	return darwinShortcut{
		canonical: shortcut.String(),
		keyCode:   keyCode,
		modifiers: modifiers,
		shortcut:  shortcut,
	}, nil
}

func (s *darwinService) Show() bool {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	return started && C.SSHManMenuBarShow() != 0
}

func (s *darwinService) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.mu.Unlock()

	C.SSHManMenuBarStop()
	callbackMu.Lock()
	quitHandler = nil
	browserSwitchHandler = nil
	browserSwitchCommitHandler = nil
	browserSwitchCancelHandler = nil
	callbackMu.Unlock()
}

//export SSHManMenuBarQuitRequested
func SSHManMenuBarQuitRequested() {
	callbackMu.RLock()
	handler := quitHandler
	callbackMu.RUnlock()
	if handler != nil {
		handler()
	}
}

//export SSHManBrowserShortcutRequested
func SSHManBrowserShortcutRequested(direction C.uint, sessionID C.ulonglong) {
	var switchDirection BrowserSwitchDirection
	switch direction {
	case C.SSHManBrowserSwitchForward:
		switchDirection = BrowserSwitchForward
	case C.SSHManBrowserSwitchBackward:
		switchDirection = BrowserSwitchBackward
	default:
		return
	}

	callbackMu.RLock()
	handler := browserSwitchHandler
	callbackMu.RUnlock()
	if handler != nil {
		handler(switchDirection, uint64(sessionID))
	}
}

//export SSHManBrowserShortcutCommitted
func SSHManBrowserShortcutCommitted(sessionID C.ulonglong) {
	callbackMu.RLock()
	handler := browserSwitchCommitHandler
	callbackMu.RUnlock()
	if handler != nil {
		handler(uint64(sessionID))
	}
}

//export SSHManBrowserShortcutCanceled
func SSHManBrowserShortcutCanceled(sessionID C.ulonglong) {
	callbackMu.RLock()
	handler := browserSwitchCancelHandler
	callbackMu.RUnlock()
	if handler != nil {
		handler(uint64(sessionID))
	}
}

const (
	carbonCommandKey uint32 = 1 << 8
	carbonShiftKey   uint32 = 1 << 9
	carbonOptionKey  uint32 = 1 << 11
	carbonControlKey uint32 = 1 << 12
)

var darwinKeyCodes = map[string]uint32{
	"A": 0, "S": 1, "D": 2, "F": 3, "H": 4, "G": 5, "Z": 6, "X": 7,
	"C": 8, "V": 9, "B": 11, "Q": 12, "W": 13, "E": 14, "R": 15,
	"Y": 16, "T": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22,
	"5": 23, "=": 24, "9": 25, "7": 26, "-": 27, "8": 28, "0": 29,
	"]": 30, "O": 31, "U": 32, "[": 33, "I": 34, "P": 35, "Enter": 36,
	"L": 37, "J": 38, "'": 39, "K": 40, ";": 41, "\\": 42, ",": 43,
	"/": 44, "N": 45, "M": 46, ".": 47, "Tab": 48, "Space": 49, "`": 50,
	"Backspace": 51, "Escape": 53, "F17": 64, "F18": 79, "F19": 80,
	"F20": 90, "F5": 96, "F6": 97, "F7": 98, "F3": 99, "F8": 100,
	"F9": 101, "F11": 103, "F13": 105, "F16": 106, "F14": 107,
	"F10": 109, "F12": 111, "F15": 113, "F4": 118, "F2": 120, "F1": 122,
	"ArrowLeft": 123, "ArrowRight": 124, "ArrowDown": 125, "ArrowUp": 126,
	"Delete": 117,
}
