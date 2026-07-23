package defaultbrowser

import (
	"errors"
	"testing"
)

func TestManagerReportsAndSetsDefaultBrowser(t *testing.T) {
	current := false
	manager := NewManager()
	manager.supported = func() bool { return true }
	manager.isDefault = func() (bool, error) { return current, nil }
	manager.setDefault = func() error {
		current = true
		return nil
	}

	status, err := manager.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Supported || status.IsDefault {
		t.Fatalf("initial status = %#v", status)
	}

	status, err = manager.SetAsDefault()
	if err != nil {
		t.Fatalf("set default: %v", err)
	}
	if !status.Supported || !status.IsDefault {
		t.Fatalf("updated status = %#v", status)
	}
}

func TestManagerRejectsUnsupportedPlatform(t *testing.T) {
	manager := NewManager()
	manager.supported = func() bool { return false }

	if _, err := manager.SetAsDefault(); err == nil {
		t.Fatal("expected unsupported-platform error")
	}
}

func TestManagerPropagatesRegistrationAndVerificationFailures(t *testing.T) {
	manager := NewManager()
	manager.supported = func() bool { return true }
	manager.setDefault = func() error { return errors.New("registration failed") }
	if _, err := manager.SetAsDefault(); err == nil {
		t.Fatal("expected registration error")
	}

	manager.setDefault = func() error { return nil }
	manager.isDefault = func() (bool, error) { return false, errors.New("verification failed") }
	if _, err := manager.SetAsDefault(); err == nil {
		t.Fatal("expected verification error")
	}
}
