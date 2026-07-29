//go:build !darwin

package appupdate

import (
	"context"
	"errors"
	"os"
)

type unsupportedInstaller struct{}

func newPlatformInstaller() platformInstaller {
	return unsupportedInstaller{}
}

func (unsupportedInstaller) supported() bool {
	return false
}

func (unsupportedInstaller) stage(context.Context, *Client, *updatePlan, string) (*stagedUpdate, error) {
	return nil, errors.New("automatic updates are currently available only on macOS")
}

func (unsupportedInstaller) prepare(*stagedUpdate, int) error {
	return errors.New("automatic updates are currently available only on macOS")
}

func (unsupportedInstaller) cleanup(staged *stagedUpdate) error {
	if staged == nil || staged.RootPath == "" {
		return nil
	}
	return os.RemoveAll(staged.RootPath)
}

func runApplyHelper([]string) error {
	return errors.New("automatic update helper is currently available only on macOS")
}
