package appupdate

import "fmt"

const applyUpdateArgument = "--ssh-man-apply-update"

func RunHelper(args []string) (bool, error) {
	if len(args) == 0 || args[0] != applyUpdateArgument {
		return false, nil
	}
	if err := runApplyHelper(args[1:]); err != nil {
		return true, fmt.Errorf("apply automatic update: %w", err)
	}
	return true, nil
}
