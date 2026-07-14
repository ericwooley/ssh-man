//go:build darwin && cgo

package menubar

/*
#cgo LDFLAGS: -framework Cocoa
#include "native_darwin.h"
*/
import "C"

import (
	"fmt"
	"sync"
)

var (
	callbackMu  sync.RWMutex
	quitHandler func()
)

type darwinService struct {
	mu      sync.Mutex
	started bool
	quit    func()
}

func New(callbacks Callbacks) Service {
	return &darwinService{quit: callbacks.Quit}
}

func (s *darwinService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}

	callbackMu.Lock()
	quitHandler = s.quit
	callbackMu.Unlock()

	if C.SSHManMenuBarStart() == 0 {
		callbackMu.Lock()
		quitHandler = nil
		callbackMu.Unlock()
		return fmt.Errorf("create macOS menu-bar item")
	}

	s.started = true
	return nil
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
