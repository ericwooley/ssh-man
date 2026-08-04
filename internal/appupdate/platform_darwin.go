//go:build darwin

package appupdate

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const expectedBundleID = "tech.moonpixels.ssh-man"

type darwinInstaller struct{}

func newPlatformInstaller() platformInstaller {
	return darwinInstaller{}
}

func (darwinInstaller) supported() bool {
	return true
}

func (darwinInstaller) stage(
	ctx context.Context,
	client *Client,
	plan *updatePlan,
	configDir string,
) (_ *stagedUpdate, returnErr error) {
	currentApp, err := currentAppPath()
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(currentApp, "/Volumes/") {
		return nil, errors.New("the running app is on a read-only disk image; move SSH Man to Applications before enabling updates")
	}
	if err := unix.Access(filepath.Dir(currentApp), unix.W_OK); err != nil {
		return nil, fmt.Errorf("the app directory is not writable: %w", err)
	}

	updatesDir := filepath.Join(configDir, "updates")
	if err := os.MkdirAll(updatesDir, 0o700); err != nil {
		return nil, fmt.Errorf("create update directory: %w", err)
	}
	rootPath, err := os.MkdirTemp(updatesDir, plan.Version+"-")
	if err != nil {
		return nil, fmt.Errorf("create update staging directory: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = os.RemoveAll(rootPath)
		}
	}()

	dmgPath := filepath.Join(rootPath, releaseAssetName)
	if err := client.download(ctx, plan.Asset, dmgPath); err != nil {
		return nil, err
	}
	if _, err := runCommand(ctx, "/usr/bin/hdiutil", "verify", dmgPath); err != nil {
		return nil, fmt.Errorf("verify update disk image checksum: %w", err)
	}
	if _, err := runCommand(ctx, "/usr/bin/codesign", "--verify", "--strict", dmgPath); err != nil {
		return nil, fmt.Errorf("verify update disk image signature: %w", err)
	}
	if _, err := runCommand(
		ctx,
		"/usr/sbin/spctl",
		"--assess",
		"--type",
		"open",
		"--context",
		"context:primary-signature",
		dmgPath,
	); err != nil {
		return nil, fmt.Errorf("verify update disk image with Gatekeeper: %w", err)
	}

	mountPath := filepath.Join(rootPath, "mount")
	if err := os.Mkdir(mountPath, 0o700); err != nil {
		return nil, fmt.Errorf("create update mount point: %w", err)
	}
	attached := false
	defer func() {
		if !attached {
			return
		}
		detachContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = runCommand(detachContext, "/usr/bin/hdiutil", "detach", mountPath)
	}()

	if _, err := runCommand(
		ctx,
		"/usr/bin/hdiutil",
		"attach",
		"-nobrowse",
		"-readonly",
		"-mountpoint",
		mountPath,
		dmgPath,
	); err != nil {
		return nil, fmt.Errorf("mount verified update disk image: %w", err)
	}
	attached = true

	sourceApp := filepath.Join(mountPath, "ssh-man.app")
	if err := verifyBundle(ctx, currentApp, sourceApp, plan.Version); err != nil {
		return nil, fmt.Errorf("verify app in update disk image: %w", err)
	}
	stagedApp := filepath.Join(rootPath, "ssh-man.app")
	if _, err := runCommand(ctx, "/usr/bin/ditto", sourceApp, stagedApp); err != nil {
		return nil, fmt.Errorf("stage verified app bundle: %w", err)
	}
	if err := verifyBundle(ctx, currentApp, stagedApp, plan.Version); err != nil {
		return nil, fmt.Errorf("verify staged app bundle: %w", err)
	}

	detachContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, detachErr := runCommand(detachContext, "/usr/bin/hdiutil", "detach", mountPath)
	cancel()
	if detachErr != nil {
		return nil, fmt.Errorf("detach update disk image: %w", detachErr)
	}
	attached = false
	if err := os.RemoveAll(mountPath); err != nil {
		return nil, fmt.Errorf("remove update mount point: %w", err)
	}
	if err := os.Remove(dmgPath); err != nil {
		return nil, fmt.Errorf("remove staged disk image: %w", err)
	}

	return &stagedUpdate{
		Version:        plan.Version,
		AppPath:        stagedApp,
		RootPath:       rootPath,
		CurrentAppPath: currentApp,
	}, nil
}

func (darwinInstaller) prepare(staged *stagedUpdate, parentPID int) error {
	if staged == nil {
		return nil
	}
	if err := verifyBundle(context.Background(), staged.CurrentAppPath, staged.AppPath, staged.Version); err != nil {
		return fmt.Errorf("reverify staged app: %w", err)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate update helper executable: %w", err)
	}
	logPath := filepath.Join(staged.RootPath, "install.log")
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open update install log: %w", err)
	}

	command := exec.Command(
		executablePath,
		applyUpdateArgument,
		"--parent-pid", strconv.Itoa(parentPID),
		"--current-app", staged.CurrentAppPath,
		"--staged-app", staged.AppPath,
		"--version", staged.Version,
		"--root", staged.RootPath,
	)
	command.Stdout = logFile
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start update install helper: %w", err)
	}
	_ = logFile.Close()
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release update install helper: %w", err)
	}
	return nil
}

func (darwinInstaller) cleanup(staged *stagedUpdate) error {
	if staged == nil || staged.RootPath == "" {
		return nil
	}
	return os.RemoveAll(staged.RootPath)
}

func runApplyHelper(args []string) error {
	flags := flag.NewFlagSet("ssh-man automatic update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	parentPID := flags.Int("parent-pid", 0, "")
	currentApp := flags.String("current-app", "", "")
	stagedApp := flags.String("staged-app", "", "")
	version := flags.String("version", "", "")
	rootPath := flags.String("root", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *parentPID <= 0 || *currentApp == "" || *stagedApp == "" || *rootPath == "" {
		return errors.New("automatic update helper arguments are incomplete")
	}
	if _, ok := parseVersion(*version); !ok {
		return fmt.Errorf("automatic update helper version is invalid: %q", *version)
	}

	cleanRoot, err := filepath.Abs(*rootPath)
	if err != nil {
		return fmt.Errorf("resolve update root: %w", err)
	}
	cleanStaged, err := filepath.Abs(*stagedApp)
	if err != nil {
		return fmt.Errorf("resolve staged app: %w", err)
	}
	if cleanStaged != filepath.Join(cleanRoot, "ssh-man.app") ||
		filepath.Base(filepath.Dir(cleanRoot)) != "updates" ||
		!strings.HasPrefix(filepath.Base(cleanRoot), *version+"-") {
		return errors.New("staged app and update root do not match SSH Man's staging layout")
	}
	cleanCurrent, err := filepath.Abs(*currentApp)
	if err != nil {
		return fmt.Errorf("resolve current app: %w", err)
	}
	if filepath.Ext(cleanCurrent) != ".app" || filepath.Ext(cleanStaged) != ".app" {
		return errors.New("automatic update paths must point to app bundles")
	}

	if err := waitForProcessExit(*parentPID, 5*time.Minute); err != nil {
		return err
	}
	return applyStagedUpdate(cleanCurrent, cleanStaged, cleanRoot, *version, *parentPID)
}

func applyStagedUpdate(currentApp, stagedApp, rootPath, version string, parentPID int) (returnErr error) {
	parentDir := filepath.Dir(currentApp)
	if err := unix.Access(parentDir, unix.W_OK); err != nil {
		return fmt.Errorf("the app directory is not writable: %w", err)
	}
	candidateApp := filepath.Join(parentDir, fmt.Sprintf(".ssh-man-update-%d.app", parentPID))
	backupApp := filepath.Join(parentDir, fmt.Sprintf(".ssh-man-backup-%d.app", parentPID))
	for _, path := range []string{candidateApp, backupApp} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("automatic update temporary path already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect automatic update temporary path %q: %w", path, err)
		}
	}

	if _, err := runCommand(context.Background(), "/usr/bin/ditto", stagedApp, candidateApp); err != nil {
		return fmt.Errorf("copy update beside installed app: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = os.RemoveAll(candidateApp)
		}
	}()
	if err := verifyBundle(context.Background(), currentApp, candidateApp, version); err != nil {
		return fmt.Errorf("verify copied update: %w", err)
	}

	if err := os.Rename(currentApp, backupApp); err != nil {
		return fmt.Errorf("move installed app to backup: %w", err)
	}
	restoreBackup := true
	installedCandidate := false
	defer func() {
		if !restoreBackup {
			return
		}
		var restoreErr error
		if installedCandidate {
			if err := os.Rename(currentApp, candidateApp); err != nil {
				restoreErr = fmt.Errorf("move failed update aside for rollback: %w", err)
			}
		}
		if restoreErr == nil {
			if err := os.Rename(backupApp, currentApp); err != nil {
				restoreErr = fmt.Errorf("restore installed app after failed update: %w", err)
			}
		}
		if restoreErr != nil {
			returnErr = errors.Join(returnErr, restoreErr)
		}
	}()
	if err := os.Rename(candidateApp, currentApp); err != nil {
		return fmt.Errorf("install verified update: %w", err)
	}
	installedCandidate = true
	if err := verifyBundle(context.Background(), backupApp, currentApp, version); err != nil {
		return fmt.Errorf("verify installed update: %w", err)
	}
	restoreBackup = false

	if err := os.RemoveAll(backupApp); err != nil {
		return fmt.Errorf("remove previous app bundle: %w", err)
	}
	if err := os.RemoveAll(rootPath); err != nil {
		return fmt.Errorf("remove update staging directory: %w", err)
	}
	return nil
}

func verifyBundle(ctx context.Context, currentApp, candidateApp, expectedVersion string) error {
	for label, appPath := range map[string]string{
		"current":   currentApp,
		"candidate": candidateApp,
	} {
		if info, err := os.Stat(appPath); err != nil || !info.IsDir() {
			if err == nil {
				err = errors.New("path is not a directory")
			}
			return fmt.Errorf("%s app bundle %q is unavailable: %w", label, appPath, err)
		}
		if _, err := runCommand(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", appPath); err != nil {
			return fmt.Errorf("%s app code signature: %w", label, err)
		}
	}

	currentBundleID, err := plistValue(ctx, currentApp, "CFBundleIdentifier")
	if err != nil {
		return err
	}
	candidateBundleID, err := plistValue(ctx, candidateApp, "CFBundleIdentifier")
	if err != nil {
		return err
	}
	if currentBundleID != expectedBundleID || candidateBundleID != currentBundleID {
		return fmt.Errorf("candidate bundle identifier %q does not match installed bundle %q", candidateBundleID, currentBundleID)
	}
	candidateVersion, err := plistValue(ctx, candidateApp, "CFBundleShortVersionString")
	if err != nil {
		return err
	}
	if candidateVersion != expectedVersion {
		return fmt.Errorf("candidate bundle version = %q, want %q", candidateVersion, expectedVersion)
	}

	currentTeam, err := teamIdentifier(ctx, currentApp)
	if err != nil {
		return fmt.Errorf("read installed app signing team: %w", err)
	}
	candidateTeam, err := teamIdentifier(ctx, candidateApp)
	if err != nil {
		return fmt.Errorf("read candidate app signing team: %w", err)
	}
	if currentTeam == "" || candidateTeam != currentTeam {
		return fmt.Errorf("candidate signing team %q does not match installed app signing team %q", candidateTeam, currentTeam)
	}
	if _, err := runCommand(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", candidateApp); err != nil {
		return fmt.Errorf("candidate app Gatekeeper assessment: %w", err)
	}
	return nil
}

func currentAppPath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executablePath); resolveErr == nil {
		executablePath = resolved
	}
	return bundlePathFromExecutable(executablePath)
}

func bundlePathFromExecutable(executablePath string) (string, error) {
	marker := ".app" + string(filepath.Separator) + "Contents" + string(filepath.Separator) + "MacOS" + string(filepath.Separator)
	index := strings.LastIndex(executablePath, marker)
	if index < 0 {
		return "", fmt.Errorf("running executable is not inside a macOS app bundle: %s", executablePath)
	}
	return filepath.Clean(executablePath[:index+len(".app")]), nil
}

func plistValue(ctx context.Context, appPath, key string) (string, error) {
	output, err := runCommand(
		ctx,
		"/usr/libexec/PlistBuddy",
		"-c",
		"Print :"+key,
		filepath.Join(appPath, "Contents", "Info.plist"),
	)
	if err != nil {
		return "", fmt.Errorf("read %s from %q: %w", key, appPath, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func teamIdentifier(ctx context.Context, appPath string) (string, error) {
	output, err := runCommand(ctx, "/usr/bin/codesign", "--display", "--verbose=4", appPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "TeamIdentifier="); found {
			return strings.TrimSpace(value), nil
		}
	}
	return "", errors.New("code signature has no TeamIdentifier")
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return nil, fmt.Errorf("%s: %w", detail, err)
		}
		return nil, err
	}
	return output, nil
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("inspect parent process %d: %w", pid, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for parent process %d to exit", pid)
}
