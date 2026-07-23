#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-e2e-vm-test.XXXXXX")"
CI_WORKFLOW="$ROOT_DIR/.github/workflows/ci.yml"

cleanup_test() {
  rm -rf "$TEST_DIR"
}
trap cleanup_test EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

SSH_MAN_E2E_SOURCE_ONLY=1
# shellcheck source=scripts/e2e-vm.sh
source "$ROOT_DIR/scripts/e2e-vm.sh"

validate_vm_name "ssh-man-e2e-20260723-123456"
if validate_vm_name "not valid"; then
  fail "VM names containing spaces must be rejected"
fi
if validate_vm_name "starts_with_underscore"; then
  fail "VM names containing underscores must be rejected"
fi

FAKE_MULTIPASS="$TEST_DIR/multipass"
TOOL_LOG="$TEST_DIR/multipass.log"
cat >"$FAKE_MULTIPASS" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf '<%s>' "$@" >>"${FAKE_MULTIPASS_LOG:?}"
printf '\n' >>"$FAKE_MULTIPASS_LOG"

if [ "${1:-}" = "info" ]; then
  if [ "${FAKE_MULTIPASS_INFO_AFTER_LAUNCH_FAILURE:-0}" = "1" ] &&
    [ -f "${FAKE_MULTIPASS_LAUNCH_STATE:?}" ]; then
    exit 0
  fi
  if [ "${FAKE_MULTIPASS_INFO_MISSING:-0}" = "1" ]; then
    exit 1
  fi
fi
if [ "${1:-}" = "launch" ] &&
  [ "${FAKE_MULTIPASS_LAUNCH_FAILURES:-0}" -gt 0 ]; then
  launch_state="${FAKE_MULTIPASS_LAUNCH_STATE:?}"
  launch_attempt=0
  if [ -f "$launch_state" ]; then
    launch_attempt="$(cat "$launch_state")"
  fi
  launch_attempt=$((launch_attempt + 1))
  printf '%s\n' "$launch_attempt" >"$launch_state"
  if [ "$launch_attempt" -le "$FAKE_MULTIPASS_LAUNCH_FAILURES" ]; then
    printf '%s\n' \
      "${FAKE_MULTIPASS_LAUNCH_ERROR:-launch failed: Remote \"release\" is unknown or unreachable.}" >&2
    exit 2
  fi
fi
if [ "${1:-}" = "exec" ] &&
  [ "${FAKE_MULTIPASS_GUEST_FAILURE:-0}" = "1" ] &&
  printf '%s\n' "$*" | grep -Fq '/tmp/e2e-vm.sh --guest'; then
  exit 42
fi
FAKE
chmod +x "$FAKE_MULTIPASS"

MULTIPASS_BIN="$FAKE_MULTIPASS"
FAKE_MULTIPASS_LOG="$TOOL_LOG"
export FAKE_MULTIPASS_LOG
VM_NAME="ssh-man-e2e-cleanup"
VM_CREATED=1
ARTIFACT_DIR="$TEST_DIR/artifacts"
mkdir -p "$ARTIFACT_DIR"

cleanup_vm

grep -Fq '<delete><--purge><ssh-man-e2e-cleanup>' "$TOOL_LOG" ||
  fail "cleanup must purge the disposable VM"

: >"$TOOL_LOG"
VM_CREATED=0
cleanup_vm
[ ! -s "$TOOL_LOG" ] || fail "cleanup must not delete a VM that was not created"

: >"$TOOL_LOG"
set +e
FAKE_MULTIPASS_INFO_MISSING=1 \
FAKE_MULTIPASS_INFO_AFTER_LAUNCH_FAILURE=1 \
FAKE_MULTIPASS_LAUNCH_FAILURES=2 \
FAKE_MULTIPASS_LAUNCH_STATE="$TEST_DIR/retryable-launch.state" \
FAKE_MULTIPASS_GUEST_FAILURE=1 \
SSH_MAN_E2E_MULTIPASS_BIN="$FAKE_MULTIPASS" \
SSH_MAN_E2E_VM_NAME="ssh-man-e2e-launch-retry" \
SSH_MAN_E2E_VM_LAUNCH_RETRY_SECONDS=1 \
SSH_MAN_E2E_ARTIFACT_DIR="$TEST_DIR/retryable-launch-artifacts" \
FAKE_MULTIPASS_LOG="$TOOL_LOG" \
  bash "$ROOT_DIR/scripts/e2e-vm.sh" >"$TEST_DIR/retryable-launch.log" 2>&1
retryable_launch_status=$?
set -e
[ "$retryable_launch_status" -eq 42 ] ||
  fail "host orchestration must retry transient launch failures and reach the guest scenario"
[ "$(grep -Fc '<launch><24.04>' "$TOOL_LOG")" -eq 3 ] ||
  fail "transient Multipass launch failures must be retried twice before success"
[ "$(grep -Fc '<delete><--purge><ssh-man-e2e-launch-retry>' "$TOOL_LOG")" -eq 3 ] ||
  fail "partial and successful VM instances must each be purged exactly once"
for attempt in 1 2 3; do
  [ -f "$TEST_DIR/retryable-launch-artifacts/multipass-launch-attempt-$attempt.log" ] ||
    fail "each Multipass launch attempt must preserve a diagnostic log"
done
grep -Fq 'Retrying Multipass launch' "$TEST_DIR/retryable-launch.log" ||
  fail "transient launch retries must be visible in the host log"

: >"$TOOL_LOG"
set +e
FAKE_MULTIPASS_INFO_MISSING=1 \
FAKE_MULTIPASS_LAUNCH_FAILURES=4 \
FAKE_MULTIPASS_LAUNCH_STATE="$TEST_DIR/permanent-launch.state" \
FAKE_MULTIPASS_LAUNCH_ERROR='launch failed: Invalid memory size' \
SSH_MAN_E2E_MULTIPASS_BIN="$FAKE_MULTIPASS" \
SSH_MAN_E2E_VM_NAME="ssh-man-e2e-launch-permanent" \
SSH_MAN_E2E_VM_LAUNCH_RETRY_SECONDS=1 \
SSH_MAN_E2E_ARTIFACT_DIR="$TEST_DIR/permanent-launch-artifacts" \
FAKE_MULTIPASS_LOG="$TOOL_LOG" \
  bash "$ROOT_DIR/scripts/e2e-vm.sh" >"$TEST_DIR/permanent-launch.log" 2>&1
permanent_launch_status=$?
set -e
[ "$permanent_launch_status" -ne 0 ] ||
  fail "a permanent Multipass launch failure must fail the host orchestration"
[ "$(grep -Fc '<launch><24.04>' "$TOOL_LOG")" -eq 1 ] ||
  fail "permanent Multipass launch failures must not be retried"
if grep -Fq '<delete><--purge><ssh-man-e2e-launch-permanent>' "$TOOL_LOG"; then
  fail "a launch failure before VM creation must not trigger a bogus delete"
fi

: >"$TOOL_LOG"
set +e
FAKE_MULTIPASS_INFO_MISSING=1 \
FAKE_MULTIPASS_GUEST_FAILURE=1 \
SSH_MAN_E2E_MULTIPASS_BIN="$FAKE_MULTIPASS" \
SSH_MAN_E2E_VM_NAME="ssh-man-e2e-host-failure" \
SSH_MAN_E2E_ARTIFACT_DIR="$TEST_DIR/host-artifacts" \
FAKE_MULTIPASS_LOG="$TOOL_LOG" \
  bash "$ROOT_DIR/scripts/e2e-vm.sh" >"$TEST_DIR/host-run.log" 2>&1
host_status=$?
set -e
[ "$host_status" -eq 42 ] ||
  fail "host orchestration must preserve a guest test failure"
[ -f "$TEST_DIR/host-artifacts/ssh-man-e2e-result.log" ] ||
  fail "host orchestration must preserve the guest output"
grep -Fq '<launch><24.04><--name><ssh-man-e2e-host-failure>' "$TOOL_LOG" ||
  fail "host orchestration must launch the disposable VM"
grep -Fq '</tmp/e2e-vm.sh><--guest>' "$TOOL_LOG" ||
  fail "host orchestration must run the guest scenario"
grep -Fq '<delete><--purge><ssh-man-e2e-host-failure>' "$TOOL_LOG" ||
  fail "a failed guest scenario must still purge the disposable VM"

grep -Fq 'server add' "$ROOT_DIR/scripts/e2e-vm.sh" ||
  fail "guest scenario must create servers through the CLI"
grep -Fq 'tunnel add socks' "$ROOT_DIR/scripts/e2e-vm.sh" ||
  fail "guest scenario must create SOCKS tunnels through the CLI"
grep -Fq 'tunnel start' "$ROOT_DIR/scripts/e2e-vm.sh" ||
  fail "guest scenario must start tunnels through the CLI"
grep -Fq 'ssh-man-e2e.invalid' "$ROOT_DIR/scripts/e2e-vm.sh" ||
  fail "guest scenario must exercise an OpenSSH Host alias"

VM_E2E_JOB="$(
  awk '
    /^  vm-e2e:$/ {
      in_job = 1
    }
    in_job && /^  [a-zA-Z0-9_-]+:$/ && $0 != "  vm-e2e:" {
      exit
    }
    in_job {
      print
    }
  ' "$CI_WORKFLOW"
)"

[ -n "$VM_E2E_JOB" ] ||
  fail "CI must define a vm-e2e job"
grep -Fqx '    needs: validate' <<<"$VM_E2E_JOB" ||
  fail "VM E2E must wait for regular validation to pass"
grep -Fqx '    runs-on: ubuntu-24.04' <<<"$VM_E2E_JOB" ||
  fail "VM E2E must use the pinned Ubuntu runner required by Multipass"
grep -Fq 'sudo snap install multipass' <<<"$VM_E2E_JOB" ||
  fail "VM E2E must install Multipass"
grep -Fq "sudo multipass version" <<<"$VM_E2E_JOB" ||
  fail "VM E2E must use Multipass with access to its root-owned socket"
grep -Fq "TMPDIR=/var/snap/multipass/common/ssh-man-e2e-tmp" <<<"$VM_E2E_JOB" ||
  fail "VM E2E inputs must be staged somewhere readable by the confined Multipass snap"
grep -Eq '^[[:space:]]+\./scripts/e2e-vm\.sh$' <<<"$VM_E2E_JOB" ||
  fail "VM E2E must run the real disposable-VM script"
grep -Fq 'if: ${{ always() }}' <<<"$VM_E2E_JOB" ||
  fail "VM E2E logs must be uploaded even after failure"

printf 'e2e-vm script tests passed\n'
