#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-e2e-vm-test.XXXXXX")"

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

if [ "${1:-}" = "info" ] && [ "${FAKE_MULTIPASS_INFO_MISSING:-0}" = "1" ]; then
  exit 1
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

printf 'e2e-vm script tests passed\n'
