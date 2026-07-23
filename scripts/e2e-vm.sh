#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MULTIPASS_BIN="${SSH_MAN_E2E_MULTIPASS_BIN:-multipass}"
VM_IMAGE="${SSH_MAN_E2E_VM_IMAGE:-24.04}"
VM_CPUS="${SSH_MAN_E2E_VM_CPUS:-4}"
VM_MEMORY="${SSH_MAN_E2E_VM_MEMORY:-8G}"
VM_DISK="${SSH_MAN_E2E_VM_DISK:-20G}"
VM_TIMEOUT="${SSH_MAN_E2E_VM_TIMEOUT:-1200}"
VM_SSH_WAIT_SECONDS="${SSH_MAN_E2E_SSH_WAIT_SECONDS:-300}"
VM_LAUNCH_ATTEMPTS="${SSH_MAN_E2E_VM_LAUNCH_ATTEMPTS:-4}"
VM_LAUNCH_RETRY_SECONDS="${SSH_MAN_E2E_VM_LAUNCH_RETRY_SECONDS:-10}"
GO_VERSION="${SSH_MAN_E2E_GO_VERSION:-$(awk '/^go / { print $2; exit }' "$ROOT_DIR/go.mod")}"
NODE_MAJOR="${SSH_MAN_E2E_NODE_MAJOR:-26}"
PNPM_VERSION="${SSH_MAN_E2E_PNPM_VERSION:-11.17.0}"
VM_NAME="${SSH_MAN_E2E_VM_NAME:-ssh-man-e2e-$(date +%Y%m%d-%H%M%S)-$$}"
ARTIFACT_DIR="${SSH_MAN_E2E_ARTIFACT_DIR:-$ROOT_DIR/build/e2e-vm/$VM_NAME}"
VM_CREATED=0
RUN_DIR=""
GUEST_BINARY_PATH=""
GUEST_CONFIG_HOME=""
GUEST_APP_PID=""
GUEST_APP_LOG="/tmp/ssh-man-e2e-app.log"

log() {
  printf '\n==> %s\n' "$*"
}

fail() {
  printf 'ssh-man VM E2E: %s\n' "$*" >&2
  return 1
}

require_command() {
  local command_name="$1"

  if ! command -v "$command_name" >/dev/null 2>&1; then
    fail "missing required command: $command_name"
  fi
}

validate_vm_name() {
  local name="$1"
  [[ "$name" =~ ^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}$ ]]
}

validate_positive_integer() {
  local label="$1"
  local value="$2"

  if ! [[ "$value" =~ ^[1-9][0-9]*$ ]]; then
    fail "$label must be a positive integer (received: $value)"
  fi
}

is_retryable_vm_launch_failure() {
  local launch_log="$1"

  grep -Eiq \
    'unknown or unreachable|failed to (fetch|download)|download failed|network is unreachable|temporary failure|connection (reset|timed out)|timed out (while|waiting)|TLS handshake timeout|could not resolve' \
    "$launch_log"
}

cleanup_vm() {
  local delete_status=0

  if [ "$VM_CREATED" -ne 1 ]; then
    return 0
  fi

  log "Deleting disposable VM $VM_NAME"
  if ! "$MULTIPASS_BIN" delete --purge "$VM_NAME"; then
    printf 'warning: failed to purge disposable VM %s\n' "$VM_NAME" >&2
    delete_status=1
  fi
  VM_CREATED=0
  return "$delete_status"
}

cleanup_host() {
  local status=$?

  set +e
  cleanup_vm
  if [ -n "$RUN_DIR" ] && [ -d "$RUN_DIR" ]; then
    rm -rf "$RUN_DIR"
  fi
  return "$status"
}

launch_vm() {
  local attempt
  local launch_log
  local launch_status

  for attempt in $(seq 1 "$VM_LAUNCH_ATTEMPTS"); do
    launch_log="$ARTIFACT_DIR/multipass-launch-attempt-$attempt.log"

    log "Launching disposable Multipass VM $VM_NAME ($VM_IMAGE), attempt $attempt/$VM_LAUNCH_ATTEMPTS"
    if "$MULTIPASS_BIN" launch "$VM_IMAGE" \
      --name "$VM_NAME" \
      --cpus "$VM_CPUS" \
      --memory "$VM_MEMORY" \
      --disk "$VM_DISK" \
      --cloud-init "$1" \
      --timeout "$VM_TIMEOUT" >"$launch_log" 2>&1; then
      cat "$launch_log"
      VM_CREATED=1
      return 0
    else
      launch_status=$?
    fi

    cat "$launch_log" >&2

    # Multipass can leave an instance behind when provisioning fails after
    # creation. Remove it before retrying with the same isolated name.
    if "$MULTIPASS_BIN" info "$VM_NAME" >/dev/null 2>&1; then
      VM_CREATED=1
      if ! cleanup_vm; then
        fail "could not clean up the partially created VM after launch failure"
        return 1
      fi
    fi

    if ! is_retryable_vm_launch_failure "$launch_log"; then
      fail "Multipass launch failed with a non-retryable error (status $launch_status); see $launch_log"
      return "$launch_status"
    fi
    if [ "$attempt" -eq "$VM_LAUNCH_ATTEMPTS" ]; then
      fail "Multipass launch still failed after $VM_LAUNCH_ATTEMPTS attempts; see $launch_log"
      return "$launch_status"
    fi

    log "Retrying Multipass launch in ${VM_LAUNCH_RETRY_SECONDS}s after a transient provisioning failure"
    sleep "$VM_LAUNCH_RETRY_SECONDS"
  done
}

write_cloud_init() {
  local output_path="$1"

  cat >"$output_path" <<'CLOUD_INIT'
#cloud-config
package_update: true
package_upgrade: false
packages:
  - build-essential
  - ca-certificates
  - curl
  - dbus-x11
  - jq
  - libgtk-3-dev
  - libwebkit2gtk-4.1-dev
  - openssh-client
  - openssh-server
  - pkg-config
  - tar
  - xauth
  - xz-utils
  - xvfb
runcmd:
  - [systemctl, enable, --now, ssh]
CLOUD_INIT
}

create_source_archive() {
  local output_path="$1"

  (
    cd "$ROOT_DIR"
    COPYFILE_DISABLE=1 tar \
      --no-xattrs \
      --exclude='./.git' \
      --exclude='./.pnpm-store' \
      --exclude='./build' \
      --exclude='./node_modules' \
      --exclude='./frontend/node_modules' \
      --exclude='./coverage' \
      --exclude='./tmp' \
      -czf "$output_path" .
  )
}

wait_for_vm_ssh() {
  local deadline=$((SECONDS + VM_SSH_WAIT_SECONDS))

  log "Waiting for Multipass SSH (${VM_SSH_WAIT_SECONDS}s max)"
  until "$MULTIPASS_BIN" exec "$VM_NAME" -- true >/dev/null 2>&1; do
    if [ "$SECONDS" -ge "$deadline" ]; then
      "$MULTIPASS_BIN" info "$VM_NAME" >&2 || true
      fail "timed out waiting for Multipass SSH for $VM_NAME"
    fi
    sleep 3
  done
}

run_host() {
  local source_archive
  local cloud_init

  require_command "$MULTIPASS_BIN"
  require_command tar
  validate_positive_integer "SSH_MAN_E2E_VM_CPUS" "$VM_CPUS"
  validate_positive_integer "SSH_MAN_E2E_VM_TIMEOUT" "$VM_TIMEOUT"
  validate_positive_integer "SSH_MAN_E2E_SSH_WAIT_SECONDS" "$VM_SSH_WAIT_SECONDS"
  validate_positive_integer "SSH_MAN_E2E_VM_LAUNCH_ATTEMPTS" "$VM_LAUNCH_ATTEMPTS"
  validate_positive_integer "SSH_MAN_E2E_VM_LAUNCH_RETRY_SECONDS" "$VM_LAUNCH_RETRY_SECONDS"
  if ! validate_vm_name "$VM_NAME"; then
    fail "invalid VM name: $VM_NAME"
  fi
  if ! [[ "$GO_VERSION" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
    fail "invalid Go version: $GO_VERSION"
  fi
  if "$MULTIPASS_BIN" info "$VM_NAME" >/dev/null 2>&1; then
    fail "refusing to reuse existing VM: $VM_NAME"
  fi

  RUN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/ssh-man-e2e-vm.XXXXXX")"
  source_archive="$RUN_DIR/ssh-man-source.tar.gz"
  cloud_init="$RUN_DIR/cloud-init.yaml"
  mkdir -p "$ARTIFACT_DIR"
  trap cleanup_host EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  log "Creating an isolated source snapshot"
  create_source_archive "$source_archive"
  write_cloud_init "$cloud_init"

  launch_vm "$cloud_init"

  wait_for_vm_ssh

  log "Waiting for cloud-init and SSH dependencies"
  "$MULTIPASS_BIN" exec "$VM_NAME" -- \
    cloud-init status --wait --long

  log "Transferring the current working tree snapshot"
  "$MULTIPASS_BIN" transfer "$source_archive" "$VM_NAME:/tmp/ssh-man-source.tar.gz"
  "$MULTIPASS_BIN" transfer "${BASH_SOURCE[0]}" "$VM_NAME:/tmp/e2e-vm.sh"

  log "Running the SSH Man CLI scenario inside $VM_NAME"
  "$MULTIPASS_BIN" exec "$VM_NAME" -- \
    env "SSH_MAN_E2E_GO_VERSION=$GO_VERSION" \
    "SSH_MAN_E2E_NODE_MAJOR=$NODE_MAJOR" \
    "SSH_MAN_E2E_PNPM_VERSION=$PNPM_VERSION" \
    bash /tmp/e2e-vm.sh --guest /tmp/ssh-man-source.tar.gz \
    2>&1 | tee "$ARTIFACT_DIR/ssh-man-e2e-result.log"

  log "VM E2E passed"
}

install_guest_go() {
  local architecture
  local download_path="/tmp/ssh-man-e2e-go.tar.gz"
  local toolchain_root="$HOME/.local/ssh-man-e2e-toolchain"

  case "$(uname -m)" in
    x86_64) architecture="amd64" ;;
    aarch64|arm64) architecture="arm64" ;;
    *) fail "unsupported VM architecture: $(uname -m)" ;;
  esac

  mkdir -p "$toolchain_root"
  curl --fail --location --silent --show-error \
    "https://go.dev/dl/go${GO_VERSION}.linux-${architecture}.tar.gz" \
    --output "$download_path"
  tar -C "$toolchain_root" -xzf "$download_path"
  export PATH="$toolchain_root/go/bin:$PATH"

  if [ "$(go env GOVERSION)" != "go$GO_VERSION" ]; then
    fail "downloaded Go version does not match go.mod"
  fi
}

install_guest_node() {
  local architecture
  local node_version
  local download_path="/tmp/ssh-man-e2e-node.tar.xz"
  local toolchain_root="$HOME/.local/ssh-man-e2e-node"

  case "$(uname -m)" in
    x86_64) architecture="x64" ;;
    aarch64|arm64) architecture="arm64" ;;
    *) fail "unsupported VM architecture: $(uname -m)" ;;
  esac

  node_version="$(
    curl --fail --location --silent --show-error \
      https://nodejs.org/dist/index.json |
      jq -r --arg prefix "v$NODE_MAJOR." \
        '[.[] | select(.version | startswith($prefix))][0].version'
  )"
  if ! [[ "$node_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "could not resolve the latest Node.js $NODE_MAJOR release"
  fi

  mkdir -p "$toolchain_root"
  curl --fail --location --silent --show-error \
    "https://nodejs.org/dist/$node_version/node-$node_version-linux-$architecture.tar.xz" \
    --output "$download_path"
  tar -C "$toolchain_root" --strip-components=1 -xJf "$download_path"
  export PATH="$toolchain_root/bin:$PATH"

  npm install --global --prefix "$toolchain_root" "pnpm@$PNPM_VERSION"
  if [ "$(pnpm --version)" != "$PNPM_VERSION" ]; then
    fail "installed pnpm version does not match frontend/package.json"
  fi
}

configure_guest_ssh() {
  local alias_name="$1"
  local guest_user="$2"
  local key_path="$HOME/.ssh/id_ed25519"
  local scanned_host_keys="/tmp/ssh-man-e2e-host-keys"

  mkdir -p "$HOME/.ssh"
  chmod 700 "$HOME/.ssh"
  ssh-keygen -q -t ed25519 -N '' -f "$key_path"
  cp "$key_path.pub" "$HOME/.ssh/authorized_keys"
  chmod 600 "$HOME/.ssh/authorized_keys"

  cat >"$HOME/.ssh/config" <<EOF
Host $alias_name
  HostName 127.0.0.1
  User $guest_user
  Port 22
  IdentityFile $key_path
  IdentitiesOnly yes
  HostKeyAlias $alias_name
  StrictHostKeyChecking accept-new
EOF
  chmod 600 "$HOME/.ssh/config"

  sudo systemctl enable --now ssh

  ssh-keyscan 127.0.0.1 >"$scanned_host_keys"
  cp "$scanned_host_keys" "$HOME/.ssh/known_hosts"
  sed "s/^127\\.0\\.0\\.1/$alias_name/" \
    "$scanned_host_keys" >>"$HOME/.ssh/known_hosts"
  chmod 600 "$HOME/.ssh/known_hosts"

  ssh \
    -F /dev/null \
    -o BatchMode=yes \
    -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=yes \
    -i "$key_path" \
    "$guest_user@127.0.0.1" true
  ssh -o BatchMode=yes "$alias_name" true

  if ! ssh -G "$alias_name" |
    awk '$1 == "hostname" && $2 == "127.0.0.1" { found = 1 } END { exit !found }'; then
    fail "OpenSSH did not resolve $alias_name through ~/.ssh/config"
  fi
}

wait_for_guest_app() {
  local attempt
  local status_output

  for attempt in $(seq 1 60); do
    if status_output="$(cli app status 2>/dev/null)" &&
      jq -e '.running == true' >/dev/null <<<"$status_output"; then
      return 0
    fi
    sleep 1
  done

  fail "desktop agent did not become ready within 60 seconds"
}

run_proxy_case() {
  local server_name="$1"
  local host="$2"
  local tunnel_name="$3"
  local key_path="$4"
  local guest_user="$5"
  local case_dir="/tmp/ssh-man-e2e-$server_name"

  mkdir -p "$case_dir"
  printf '\n--- CLI case: %s (%s) ---\n' "$server_name" "$host"

  cli server add "$server_name" \
    --host "$host" \
    --port 22 \
    --user "$guest_user" \
    --auth key \
    --key "$key_path" \
    --socks-port auto >"$case_dir/server-add.json"

  cli tunnel add socks "$tunnel_name" \
    --server "$server_name" \
    --listen auto \
    --reconnect=false >"$case_dir/tunnel-add.json"

  if ! cli tunnel start "$tunnel_name" --server "$server_name" \
    >"$case_dir/tunnel-start.json" 2>"$case_dir/tunnel-start.err"; then
    printf 'Tunnel start failed for %s.\n' "$server_name" >&2
    cat "$case_dir/tunnel-start.json" >&2
    cat "$case_dir/tunnel-start.err" >&2
    printf 'Recent CLI history:\n' >&2
    cli tunnel history "$tunnel_name" --server "$server_name" --limit 5 >&2 || true
    return 1
  fi

  jq -e '.status == "connected"' \
    "$case_dir/tunnel-start.json" >/dev/null ||
    fail "CLI did not report a connected SOCKS tunnel for $server_name"

  cli tunnel stop "$tunnel_name" --server "$server_name" \
    >"$case_dir/tunnel-stop.json"
  cli server delete "$server_name" --yes --stop-active \
    >"$case_dir/server-delete.json"

  printf 'PASS: %s reached %s over SSH and opened a SOCKS listener.\n' \
    "$server_name" "$host"
}

run_guest() {
  local source_archive="${1:?source archive is required}"
  local source_dir
  local guest_user
  local alias_name="ssh-man-e2e.invalid"
  local key_path="$HOME/.ssh/id_ed25519"

  source_dir="$(mktemp -d "$HOME/ssh-man-e2e-source.XXXXXX")"
  guest_user="$(id -un)"
  tar -C "$source_dir" -xzf "$source_archive"
  GUEST_BINARY_PATH="$source_dir/build/bin/ssh-man"
  GUEST_CONFIG_HOME="$HOME/.config/ssh-man-e2e"
  GUEST_APP_PID=""

  guest_cleanup() {
    local status=$?

    set +e
    if [ -x "$GUEST_BINARY_PATH" ]; then
      XDG_CONFIG_HOME="$GUEST_CONFIG_HOME" "$GUEST_BINARY_PATH" \
        --no-autostart -o json app quit --yes >/dev/null 2>&1
    fi
    if [ -n "$GUEST_APP_PID" ] &&
      kill -0 "$GUEST_APP_PID" >/dev/null 2>&1; then
      kill "$GUEST_APP_PID" >/dev/null 2>&1
    fi
    if [ "$status" -ne 0 ]; then
      printf '\nDesktop agent log after failure:\n' >&2
      if [ -f "$GUEST_APP_LOG" ]; then
        tail -200 "$GUEST_APP_LOG" >&2
      else
        printf 'The desktop agent had not started yet.\n' >&2
      fi
    fi
    return "$status"
  }
  trap guest_cleanup EXIT

  cli() {
    XDG_CONFIG_HOME="$GUEST_CONFIG_HOME" "$GUEST_BINARY_PATH" \
      --no-autostart -o json "$@"
  }

  {
    log "Installing the Go toolchain declared by go.mod"
    install_guest_go

    log "Installing the Node.js and pnpm versions used by CI"
    install_guest_node

    log "Building the frontend and Linux desktop binary in the VM"
    cd "$source_dir"
    ./scripts/pnpm.sh install --frozen-lockfile
    ./scripts/pnpm.sh run build
    go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build \
      -clean \
      -s \
      -tags webkit2_41
    test -x "$GUEST_BINARY_PATH" ||
      fail "Wails did not produce $GUEST_BINARY_PATH"

    log "Configuring a local SSH server and an OpenSSH Host alias"
    configure_guest_ssh "$alias_name" "$guest_user"

    log "Starting the desktop agent headlessly; all product actions use its CLI"
    mkdir -p "$GUEST_CONFIG_HOME"
    nohup dbus-run-session -- \
      xvfb-run -a -s '-screen 0 1280x800x24' \
      env \
      "XDG_CONFIG_HOME=$GUEST_CONFIG_HOME" \
      WEBKIT_DISABLE_COMPOSITING_MODE=1 \
      "$GUEST_BINARY_PATH" >"$GUEST_APP_LOG" 2>&1 </dev/null &
    GUEST_APP_PID=$!
    wait_for_guest_app

    run_proxy_case \
      "Direct" "127.0.0.1" "DirectProxy" "$key_path" "$guest_user"
    run_proxy_case \
      "Alias" "$alias_name" "AliasProxy" "$key_path" "$guest_user"

    log "Both direct-address and OpenSSH-alias CLI cases passed"
  }
}

print_usage() {
  cat <<'USAGE'
Usage:
  ./scripts/e2e-vm.sh

Build SSH Man in a disposable Ubuntu Multipass VM, configure a local OpenSSH
server plus a Host alias, then use only the SSH Man CLI to create and start
SOCKS tunnels through both addresses. The VM is always purged on exit.

Optional environment:
  SSH_MAN_E2E_VM_IMAGE=24.04
  SSH_MAN_E2E_VM_CPUS=4
  SSH_MAN_E2E_VM_MEMORY=8G
  SSH_MAN_E2E_VM_DISK=20G
  SSH_MAN_E2E_VM_TIMEOUT=1200
  SSH_MAN_E2E_SSH_WAIT_SECONDS=300
  SSH_MAN_E2E_VM_LAUNCH_ATTEMPTS=4
  SSH_MAN_E2E_VM_LAUNCH_RETRY_SECONDS=10
  SSH_MAN_E2E_VM_NAME=ssh-man-e2e-custom
  SSH_MAN_E2E_GO_VERSION=1.26.5
  SSH_MAN_E2E_NODE_MAJOR=26
  SSH_MAN_E2E_PNPM_VERSION=11.17.0
USAGE
}

main() {
  case "${1:-}" in
    "")
      run_host
      ;;
    --guest)
      shift
      run_guest "$@"
      ;;
    -h|--help)
      print_usage
      ;;
    *)
      print_usage >&2
      return 2
      ;;
  esac
}

if [ "${SSH_MAN_E2E_SOURCE_ONLY:-0}" != "1" ]; then
  main "$@"
fi
