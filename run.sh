#!/usr/bin/env bash
set -euo pipefail

SCRIPT_NAME="Lulynx Server Status"
SCRIPT_VERSION="v0.3.12"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
SCRIPT_PATH="$SCRIPT_DIR/$(basename -- "${BASH_SOURCE[0]}")"
BASE_DIR="${TZ_BASE_DIR:-$SCRIPT_DIR}"
BIN_DIR="${TZ_BIN_DIR:-$BASE_DIR/bin}"
ETC_DIR="${TZ_ETC_DIR:-$BASE_DIR/config}"
DATA_DIR_DEFAULT="${TZ_DATA_DIR:-$BASE_DIR/data}"
LOG_DIR_DEFAULT="${TZ_LOG_DIR:-/var/log}"
TZ_LANG="${TZ_LANG:-zh}"
TZ_UI="${TZ_UI:-auto}" # auto|dialog|text
TZ_NO_COLOR="${TZ_NO_COLOR:-0}"
TZ_FORCE_COLOR="${TZ_FORCE_COLOR:-0}"
TZ_WIDTH="${TZ_WIDTH:-70}" # fixed width for text UI; set 0 to auto-detect
TZ_ASSUME_YES="${TZ_ASSUME_YES:-0}" # 1 = assume yes for prompts in some actions
TZ_UNINSTALL_PURGE="${TZ_UNINSTALL_PURGE:-0}" # 1 = uninstall + purge data/logs/backups (server)

release="${TZ_RELEASE:-}"
pkg_mgr="${TZ_PKG_MGR:-}"

CENTER_NAME="tanzhen-center"
PROBE_NAME="tanzhen-probe"
LEGACY_PROBE_NAME="tanzhen-agent"

# If invoked via "bash run.sh", make the script executable for next time.
if [ -f "$SCRIPT_PATH" ] && [ ! -x "$SCRIPT_PATH" ]; then
  chmod +x "$SCRIPT_PATH" 2>/dev/null || true
fi

L() {
  local zh="$1"
  local en="$2"
  if [ "$TZ_LANG" = "en" ]; then
    printf '%s' "$en"
  else
    printf '%s' "$zh"
  fi
}

usage() {
  if [ "$TZ_LANG" = "en" ]; then
    cat <<'EOF'
Usage:
  ./run.sh [server|client] [install|configure|start|stop|restart|status|show-config|uninstall|download]

Quick start (server):
  ./run.sh server install
  ./run.sh server configure
  ./run.sh server start

Quick start (client):
  ./run.sh client install
  ./run.sh client configure
  ./run.sh client start

  Notes:
  - Put this script and the binary in the same directory (or ./dist) before running.
    Expected filenames:
      server: tanzhen-center
      client: tanzhen-probe (legacy: tanzhen-agent also accepted)
  - Prefers systemd if available; falls back to nohup+pidfile.
  - Override paths via env (BASE_DIR defaults to script directory):
      TZ_BASE_DIR, TZ_BIN_DIR, TZ_ETC_DIR, TZ_DATA_DIR, TZ_LOG_DIR
    Default config dir:
      $BASE_DIR/config (or override with TZ_ETC_DIR)
  - Language:
      TZ_LANG=zh (default) or TZ_LANG=en
  - Colors:
      TZ_NO_COLOR=1 disables ANSI colors
      TZ_FORCE_COLOR=1 forces ANSI colors (if your SSH console supports it)
  - Width:
      TZ_WIDTH=70 sets fixed menu width (set 0 to auto-detect)
  - Uninstall:
      TZ_UNINSTALL_PURGE=1 purges data/logs/backups (server)
  - Download:
      (placeholder for now) set TZ_DOWNLOAD_URL_CENTER / TZ_DOWNLOAD_URL_PROBE
      Legacy env name also works: TZ_DOWNLOAD_URL_AGENT
EOF
  else
    cat <<'EOF'
用法：
  ./run.sh [服务端|客户端] [install|configure|start|stop|restart|status|show-config|uninstall|download]

快速开始（服务端）：
  ./run.sh server install
  ./run.sh server configure
  ./run.sh server start

快速开始（客户端）：
  ./run.sh client install
  ./run.sh client configure
  ./run.sh client start

 说明：
  - 把本脚本和二进制放在同一目录（或 dist/）后再运行。
    期望文件名：
      服务端：tanzhen-center
      客户端：tanzhen-probe（兼容旧名：tanzhen-agent）
  - 优先使用 systemd；没有 systemd 则使用 nohup+pidfile。
  - 可用环境变量覆盖路径（默认 BASE_DIR 为脚本所在目录）：
      TZ_BASE_DIR, TZ_BIN_DIR, TZ_ETC_DIR, TZ_DATA_DIR, TZ_LOG_DIR
    配置目录默认规则：
      默认使用 $BASE_DIR/config（或用 TZ_ETC_DIR 覆盖）
  - 语言切换：
      TZ_LANG=zh（默认）或 TZ_LANG=en
  - 颜色：
      TZ_NO_COLOR=1 可关闭颜色输出
      TZ_FORCE_COLOR=1 强制颜色输出（取决于终端是否支持）
  - 宽度：
      TZ_WIDTH=70 固定菜单宽度（设为 0 则自动探测）
  - 卸载：
      TZ_UNINSTALL_PURGE=1 卸载并清理数据/日志/备份（服务器）
  - 下载：
      （暂占位）可临时设置 TZ_DOWNLOAD_URL_CENTER / TZ_DOWNLOAD_URL_PROBE（兼容旧名：TZ_DOWNLOAD_URL_AGENT）
EOF
  fi
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "$(L "缺少依赖：" "Missing dependency: ")$1" >&2
    exit 1
  }
}

is_root() { [ "$(id -u)" -eq 0 ]; }

is_tty() { [ -t 0 ] && [ -t 1 ]; }

color_enabled() {
  if [ "$TZ_FORCE_COLOR" = "1" ]; then
    return 0
  fi
  # Follow common shell-script convention: colors ON by default, OFF only if user opts out.
  # (Some consoles don't set TERM but still render ANSI colors.)
  [ "$TZ_NO_COLOR" != "1" ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-xterm}" != "dumb" ]
}

CE() {
  local code="$1"
  shift
  if color_enabled; then
    printf '\033[%sm%s\033[0m' "$code" "$*"
  else
    printf '%s' "$*"
  fi
}

C_RED() { CE "31" "$*"; }
C_GREEN() { CE "32" "$*"; }
C_DIM() { CE "2" "$*"; }

check_sys() {
  if [ -n "${release:-}" ] && [ -n "${pkg_mgr:-}" ]; then
    return 0
  fi
  if [ -f /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    case "${ID:-}" in
      ubuntu|debian) release="$ID" ;;
      centos|rhel|rocky|almalinux|fedora) release="$ID" ;;
      alpine) release="alpine" ;;
      *) release="${ID:-unknown}" ;;
    esac
  elif [ -f /etc/redhat-release ]; then
    release="centos"
  else
    release="unknown"
  fi

  if command -v apt-get >/dev/null 2>&1; then
    pkg_mgr="apt"
  elif command -v dnf >/dev/null 2>&1; then
    pkg_mgr="dnf"
  elif command -v yum >/dev/null 2>&1; then
    pkg_mgr="yum"
  elif command -v apk >/dev/null 2>&1; then
    pkg_mgr="apk"
  else
    pkg_mgr="unknown"
  fi
}

assume_yes() {
  [ "$TZ_ASSUME_YES" = "1" ]
}

sudo_prefix() {
  if is_root; then
    echo ""
  elif command -v sudo >/dev/null 2>&1; then
    echo "sudo"
  else
    echo "$(L "错误：需要 root 权限（请用 root 运行或安装 sudo）。" "ERROR: need root privileges (run as root or install sudo).")" >&2
    exit 1
  fi
}

have_systemd() {
  command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]
}

need_root_for() {
  local what="$1"
  if is_root; then
    return 0
  fi
  if command -v sudo >/dev/null 2>&1; then
    return 0
  fi
  echo "$(L "错误：需要 root 权限来执行：" "ERROR: root privileges required for: ")$what" >&2
  exit 1
}

term_cols() {
  if [ -n "${TZ_WIDTH:-}" ]; then
    case "$TZ_WIDTH" in
      *[!0-9]*|"") : ;;
      0) : ;;
      *)
        # Avoid wrapping on the last column.
        if [ "$TZ_WIDTH" -gt 10 ]; then
          echo $((TZ_WIDTH - 1))
        else
          echo "$TZ_WIDTH"
        fi
        return 0
        ;;
    esac
  fi

  local cols=""
  if [ -n "${COLUMNS:-}" ]; then
    cols="${COLUMNS:-}"
  fi
  if [ -z "$cols" ] && is_tty && command -v stty >/dev/null 2>&1; then
    # stty size => "rows cols"
    cols="$(stty size 2>/dev/null | awk '{print $2}' || true)"
  fi
  if [ -z "$cols" ] && is_tty && command -v tput >/dev/null 2>&1; then
    cols="$(tput cols 2>/dev/null || true)"
  fi
  if [ -z "$cols" ]; then cols="80"; fi
  case "$cols" in
    *[!0-9]*|"") cols="80" ;;
  esac
  # Avoid wrapping on the last column.
  if [ "$cols" -gt 10 ]; then cols=$((cols - 1)); fi
  if [ "$cols" -lt 60 ]; then cols=60; fi
  if [ "$cols" -gt 120 ]; then cols=120; fi
  echo "$cols"
}

repeat_char() {
  local ch="$1"
  local n="$2"
  # prints n copies of ch
  printf '%*s' "$n" '' | tr ' ' "$ch"
}

fmt_menu_num() {
  # Right-align numbers to 2 chars: " 0".."10"
  printf '%2s' "${1:-}"
}

fmt_menu_num_green() {
  C_GREEN "$(fmt_menu_num "$1")"
}

use_dialog() {
  if [ "$TZ_UI" = "text" ]; then
    return 1
  fi
  if ! is_tty; then
    return 1
  fi
  if [ "${TERM:-}" = "dumb" ] || [ -z "${TERM:-}" ]; then
    return 1
  fi
  command -v dialog >/dev/null 2>&1
}

json_escape() {
  # Escapes a string for JSON value context.
  # Disallows newlines and quotes by escaping them.
  local s="${1-}"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/}"
  printf '%s' "$s"
}

gen_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24
    return 0
  fi
  if command -v dd >/dev/null 2>&1 && command -v od >/dev/null 2>&1; then
    dd if=/dev/urandom bs=1 count=24 2>/dev/null | od -An -tx1 | tr -d ' \n'
    return 0
  fi
  echo ""
}

prompt() {
  local label="$1"
  local def="${2-}"
  local out
  if use_dialog; then
    out="$(
      dialog --clear --stdout \
        --title "$SCRIPT_NAME" \
        --inputbox "$label" 10 70 "$def"
    )" || out=""
    printf '%s' "$out"
    return 0
  fi
  if [ -n "$def" ]; then
    read -r -p "$label [$def]: " out || true
    if [ -z "${out:-}" ]; then out="$def"; fi
  else
    read -r -p "$label: " out || true
  fi
  printf '%s' "$out"
}

pause_enter() {
  local _;
  if use_dialog; then return 0; fi
  read -r -p "$(L "按回车继续..." "Press Enter to continue...")" _ || true
}

prompt_yesno() {
  local label="$1"
  local def="${2:-y}"
  local out
  if use_dialog; then
    local default_yes=0
    if [ "$def" = "n" ] || [ "$def" = "N" ]; then default_yes=1; fi
    if [ "$default_yes" -eq 1 ]; then
      dialog --clear --title "$SCRIPT_NAME" --defaultno --yesno "$label" 10 70
    else
      dialog --clear --title "$SCRIPT_NAME" --yesno "$label" 10 70
    fi
    case "$?" in
      0) echo "true" ;;
      1) echo "false" ;;
      *) echo "false" ;;
    esac
    return 0
  fi
  if [ "$TZ_LANG" = "en" ]; then
    read -r -p "$label (y/n) [$def]: " out || true
  else
    read -r -p "$label（y/n）[$def]： " out || true
  fi
  out="${out:-$def}"
  case "$out" in
    y|Y|yes|YES) echo "true" ;;
    n|N|no|NO) echo "false" ;;
    *) echo "false" ;;
  esac
}

ensure_dirs() {
  local SUDO; SUDO="$(sudo_prefix)"
  $SUDO mkdir -p "$BIN_DIR" "$ETC_DIR" >/dev/null
}

ensure_exec() {
  local path="${1-}"
  [ -z "$path" ] && return 0
  [ -f "$path" ] || return 0
  if [ ! -x "$path" ]; then
    local SUDO; SUDO="$(sudo_prefix)"
    chmod 0755 "$path" 2>/dev/null || $SUDO chmod 0755 "$path" 2>/dev/null || true
  fi
}

resolve_probe_name() {
  local new="$PROBE_NAME"
  local old="$LEGACY_PROBE_NAME"

  # Prefer existing systemd unit to keep managing the already-installed service name.
  if have_systemd; then
    if [ -f "$(unit_path_for "$new")" ]; then
      echo "$new"
      return 0
    fi
    if [ -f "$(unit_path_for "$old")" ]; then
      echo "$old"
      return 0
    fi
  fi

  # Prefer the new name if both exist.
  if [ -e "$BIN_DIR/$new" ] || [ -e "$BASE_DIR/${new}.pid" ]; then
    echo "$new"
    return 0
  fi
  if [ -e "$BIN_DIR/$old" ] || [ -e "$BASE_DIR/${old}.pid" ]; then
    echo "$old"
    return 0
  fi

  echo "$new"
}

service_name_for() {
  case "$1" in
    center) echo "$CENTER_NAME" ;;
    agent|probe) resolve_probe_name ;;
    *) echo "" ;;
  esac
}

binary_name_for() {
  case "$1" in
    center) echo "$CENTER_NAME" ;;
    agent|probe) resolve_probe_name ;;
    *) echo "" ;;
  esac
}

binary_candidates_for() {
  case "$1" in
    center) echo "$CENTER_NAME" ;;
    agent|probe) echo "$PROBE_NAME $LEGACY_PROBE_NAME" ;;
    *) echo "" ;;
  esac
}

config_path_for() {
  case "$1" in
    center)
      if [ -f "$SCRIPT_DIR/center.json" ]; then
        echo "$SCRIPT_DIR/center.json"
      elif [ -f "$SCRIPT_DIR/config/center.json" ]; then
        echo "$SCRIPT_DIR/config/center.json"
      elif [ -f "$SCRIPT_DIR/configs/center.json" ]; then
        echo "$SCRIPT_DIR/configs/center.json"
      elif [ -f "$ETC_DIR/center.json" ]; then
        echo "$ETC_DIR/center.json"
      else
        echo "$ETC_DIR/center.json"
      fi
      ;;
    agent|probe)
      if [ -f "$SCRIPT_DIR/probe.json" ]; then
        echo "$SCRIPT_DIR/probe.json"
      elif [ -f "$SCRIPT_DIR/agent.json" ]; then
        echo "$SCRIPT_DIR/agent.json"
      elif [ -f "$SCRIPT_DIR/config/probe.json" ]; then
        echo "$SCRIPT_DIR/config/probe.json"
      elif [ -f "$SCRIPT_DIR/config/agent.json" ]; then
        echo "$SCRIPT_DIR/config/agent.json"
      elif [ -f "$SCRIPT_DIR/configs/probe.json" ]; then
        echo "$SCRIPT_DIR/configs/probe.json"
      elif [ -f "$SCRIPT_DIR/configs/agent.json" ]; then
        echo "$SCRIPT_DIR/configs/agent.json"
      elif [ -f "$ETC_DIR/probe.json" ]; then
        echo "$ETC_DIR/probe.json"
      elif [ -f "$ETC_DIR/agent.json" ]; then
        echo "$ETC_DIR/agent.json"
      else
        echo "$ETC_DIR/probe.json"
      fi
      ;;
    *) echo "" ;;
  esac
}

unit_path_for() {
  local svc="$1"
  echo "/etc/systemd/system/${svc}.service"
}

pid_path_for() {
  local svc="$1"
  echo "$BASE_DIR/${svc}.pid"
}

log_path_for() {
  local svc="$1"
  echo "$LOG_DIR_DEFAULT/${svc}.log"
}

backup_dir_for() {
  local svc="$1"
  echo "$BASE_DIR/backups/$svc"
}

backup_binary_if_exists() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  if [ ! -f "$bin" ]; then
    return 0
  fi
  local bdir; bdir="$(backup_dir_for "$svc")"
  $SUDO mkdir -p "$bdir" >/dev/null 2>&1 || true
  local ts
  ts="$(date +%Y%m%d%H%M%S 2>/dev/null || echo "backup")"
  $SUDO cp -f "$bin" "$bdir/${svc}-${ts}" || true
  $SUDO cp -f "$bin" "$bdir/${svc}-latest" || true
}

rollback_binary() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  local bdir; bdir="$(backup_dir_for "$svc")"
  local latest="$bdir/${svc}-latest"
  if [ ! -f "$latest" ]; then
    echo "$(L "没有找到可回滚的备份：" "No rollback backup found: ")$latest" >&2
    return 1
  fi
  $SUDO install -m 0755 "$latest" "$bin"
  echo "$(L "已回滚二进制到：" "Rolled back binary to: ")$bin"
  return 0
}

find_local_binary() {
  # Usage: find_local_binary preferredName [altName...]
  # Prefer exact name next to this script; otherwise accept a single matching suffix build.
  # Also supports placing binaries in ./dist (next to this script).
  local names=("$@")
  if [ "${#names[@]}" -eq 0 ]; then
    return 1
  fi
  local preferred="${names[0]}"

  local n
  for n in "${names[@]}"; do
    local exact="$SCRIPT_DIR/$n"
    if [ -f "$exact" ]; then
      echo "$exact"
      return 0
    fi
    local exact_dist="$SCRIPT_DIR/dist/$n"
    if [ -f "$exact_dist" ]; then
      echo "$exact_dist"
      return 0
    fi
  done

  local matches=()
  local d
  for d in "$SCRIPT_DIR" "$SCRIPT_DIR/dist"; do
    [ -d "$d" ] || continue
    for n in "${names[@]}"; do
      while IFS= read -r -d '' f; do
        case "$f" in
          *.sha256|*.sig) continue ;;
        esac
        matches+=("$f")
      done < <(find "$d" -maxdepth 1 -type f -name "${n}-*" -print0 2>/dev/null || true)
    done
  done

  if [ "${#matches[@]}" -eq 1 ]; then
    echo "${matches[0]}"
    return 0
  fi
  if [ "${#matches[@]}" -gt 1 ]; then
    # Try to pick the best match by uname (e.g. *-linux-amd64).
    local os arch want
    os="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' || echo "")"
    arch="$(uname -m 2>/dev/null || echo "")"
    case "$arch" in
      x86_64|amd64) arch="amd64" ;;
      aarch64|arm64) arch="arm64" ;;
      i386|i686) arch="386" ;;
    esac
    if [ -n "$os" ] && [ -n "$arch" ]; then
      for n in "${names[@]}"; do
        want="${n}-${os}-${arch}"
        for m in "${matches[@]}"; do
          if [ "$(basename "$m")" = "$want" ]; then
            echo "$m"
            return 0
          fi
        done
      done
    fi

    # If uname isn't available, prefer the preferred prefix if possible.
    for m in "${matches[@]}"; do
      case "$(basename "$m")" in
        "${preferred}-"*) echo "$m"; return 0 ;;
      esac
    done

    echo "$(L "错误：找到多个候选二进制，请只保留一个（或把正确的重命名为）：" "ERROR: multiple candidate binaries found; keep only one (or rename the correct one):")" >&2
    for m in "${matches[@]}"; do echo "  - $m" >&2; done
    return 1
  fi
  return 1
}

install_local() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local binName; binName="$(binary_name_for "$mode")"
  local src
  local candidates=()
  candidates+=("$binName")
  local c
  for c in $(binary_candidates_for "$mode"); do
    if [ "$c" != "$binName" ]; then
      candidates+=("$c")
    fi
  done
  src="$(find_local_binary "${candidates[@]}" || true)"
  local dst="$BIN_DIR/$binName"
  ensure_dirs
  if [ -z "${src:-}" ] || [ ! -f "$src" ]; then
    echo "$(L "错误：找不到二进制文件（run.sh 同目录或 dist/）：" "ERROR: missing binary (next to run.sh or in ./dist): ")" >&2
    for c in "${candidates[@]}"; do
      echo "  - $SCRIPT_DIR/$c" >&2
    done
    echo "$(L "提示：文件名也可以是带平台后缀的 release 产物（例如 *-linux-amd64）。" "Hint: release assets with platform suffix also work (e.g. *-linux-amd64).")" >&2
    exit 1
  fi
  ensure_exec "$src"
  backup_binary_if_exists "$mode"
  $SUDO install -m 0755 "$src" "$dst"
  echo "$(L "已安装：" "Installed: ")$dst"
}

update_binary() {
  local mode="$1"
  local binName; binName="$(binary_name_for "$mode")"
  local candidates=()
  candidates+=("$binName")
  local c
  for c in $(binary_candidates_for "$mode"); do
    if [ "$c" != "$binName" ]; then
      candidates+=("$c")
    fi
  done
  local src
  src="$(find_local_binary "${candidates[@]}" || true)"
  if [ -n "${src:-}" ] && [ -f "$src" ]; then
    install_local "$mode"
    return 0
  fi
  download_binary "$mode"
}

ensure_binary_installed() {
  local mode="$1"
  local binName; binName="$(binary_name_for "$mode")"
  local bin="$BIN_DIR/$binName"

  if [ -f "$bin" ] && [ ! -x "$bin" ]; then
    ensure_exec "$bin"
  fi
  if [ -x "$bin" ]; then
    return 0
  fi

  # If user placed the binary next to this script, auto-install it before start.
  local localSrc
  local candidates=()
  candidates+=("$binName")
  local c
  for c in $(binary_candidates_for "$mode"); do
    if [ "$c" != "$binName" ]; then
      candidates+=("$c")
    fi
  done
  localSrc="$(find_local_binary "${candidates[@]}" || true)"
  if [ -n "${localSrc:-}" ] && [ -f "$localSrc" ]; then
    ensure_exec "$localSrc"
    install_local "$mode"
  fi

  if [ ! -x "$bin" ]; then
    echo "$(L "错误：未安装二进制：" "ERROR: binary not installed: ")$bin $(L "（请先运行：" "(run:") ./run.sh $mode install$(L "）" ")")" >&2
    exit 1
  fi
}

download_binary() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  ensure_dirs

  # Placeholder: wire this to GitHub Release assets after the project is open-sourced.
  # For now you can set a fixed URL via env var.
  local url=""
  if [ "$mode" = "center" ]; then
    url="${TZ_DOWNLOAD_URL_CENTER:-}"
  else
    url="${TZ_DOWNLOAD_URL_PROBE:-}"
    if [ -z "$url" ]; then
      url="${TZ_DOWNLOAD_URL_AGENT:-}"
    fi
  fi
  if [ -z "$url" ]; then
    echo "$(L \
      "下载功能暂未配置（开源到 GitHub 后会改为自动下载 release 资产）。你可以临时设置环境变量 TZ_DOWNLOAD_URL_CENTER / TZ_DOWNLOAD_URL_PROBE 指向二进制直链。" \
      "Download is not configured yet (will be wired to GitHub Releases later). For now, set TZ_DOWNLOAD_URL_CENTER / TZ_DOWNLOAD_URL_PROBE to a direct binary URL.")" >&2
    exit 1
  fi

  local tmp
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$tmp"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp" "$url"
  else
    echo "$(L "错误：需要 curl 或 wget 才能下载。" "ERROR: need curl or wget to download.")" >&2
    exit 1
  fi

  local dst="$BIN_DIR/$(binary_name_for "$mode")"
  backup_binary_if_exists "$mode"
  $SUDO install -m 0755 "$tmp" "$dst"
  echo "$(L "已下载并安装：" "Downloaded & installed: ")$dst"
}

dialog_msg() {
  local text="$1"
  if use_dialog; then
    dialog --clear --title "$SCRIPT_NAME" --msgbox "$text" 14 80 || true
  else
    echo "$text"
  fi
}

dialog_textbox() {
  local title="$1"
  local file="$2"
  if use_dialog; then
    dialog --clear --title "$title" --textbox "$file" 22 90 || true
  else
    cat "$file" || true
  fi
}

run_capture() {
  # Usage: out="$(run_capture somefunc arg1 arg2)" ; rc=$?
  local out
  out="$("$@" 2>&1)" || {
    printf '%s' "$out"
    return 1
  }
  printf '%s' "$out"
  return 0
}

dialog_action() {
  local out rc
  out="$(run_capture "$@")"; rc=$?
  if [ -z "${out:-}" ]; then
    out="$(L "操作完成。" "Done.")"
  fi
  dialog_msg "$out"
  return $rc
}

upgrade_script() {
  # Placeholder: wire to GitHub later; allow override via env var for now.
  local url="${TZ_SCRIPT_URL:-}"
  if [ -z "$url" ]; then
    echo "$(L \
      "脚本升级暂未配置（开源到 GitHub 后会改为自动升级）。" \
      "Script upgrade is not configured yet (will be wired to GitHub later).")" >&2
    return 1
  fi
  local SUDO; SUDO="$(sudo_prefix)"
  local tmp; tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$tmp"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp" "$url"
  else
    echo "$(L "错误：需要 curl 或 wget 才能升级脚本。" "ERROR: need curl or wget to upgrade script.")" >&2
    return 1
  fi
  if [ ! -s "$tmp" ]; then
    echo "$(L "错误：下载到的脚本为空。" "ERROR: downloaded script is empty.")" >&2
    return 1
  fi
  local ts; ts="$(date +%Y%m%d%H%M%S 2>/dev/null || echo "backup")"
  $SUDO cp -f "$0" "${0}.bak-${ts}" || true
  $SUDO install -m 0755 "$tmp" "$0"
  echo "$(L "已升级脚本：" "Script upgraded: ")$0"
  return 0
}

write_center_config() {
  local SUDO; SUDO="$(sudo_prefix)"
  ensure_dirs

  # Keep prompts minimal: only ask listen/data (+ admin login on first init). Passwords are generated/reused.
  local cfg; cfg="$(config_path_for center)"
  local listen_addr data_dir ingest_token admin_token enroll_token allow_auto stealth
  local had_ingest="true" had_admin="true" had_enroll="true"
  local admin_changed="false" admin_password_changed="false"
  local def_listen def_data
  def_listen="$(json_get_string "$cfg" "listen_addr" || true)"
  def_data="$(json_get_string "$cfg" "data_dir" || true)"
  if [ -z "$def_listen" ]; then def_listen=":38088"; fi
  if [ -z "$def_data" ]; then def_data="$DATA_DIR_DEFAULT"; fi

  ingest_token="$(json_get_string "$cfg" "ingest_token" || true)"
  local admin_user admin_password
  admin_user="$(json_get_string "$cfg" "admin_user" || true)"
  admin_password="$(json_get_string "$cfg" "admin_password" || true)"
  if [ -z "$admin_password" ]; then
    admin_password="$(json_get_string "$cfg" "admin_token" || true)"
  fi
  enroll_token="$(json_get_string "$cfg" "enroll_token" || true)"
  allow_auto="$(json_get_bool "$cfg" "allow_auto_register" || true)"
  stealth="$(json_get_bool "$cfg" "stealth_ingest_unauthorized" || true)"
  if [ -z "$allow_auto" ]; then allow_auto="true"; fi
  if [ -z "$stealth" ]; then stealth="true"; fi
  if use_dialog; then
    local form
    form="$(
      dialog --clear --stdout --title "$(L "配置服务器" "Configure Server")" \
        --form "$(L "填写配置（仅需监听地址/数据目录，密码自动生成/复用）" "Fill config (only listen/data required; passwords are auto-generated/reused)")" \
        12 90 0 \
        "$(L "监听地址" "Listen addr")" 1 1 "$def_listen" 1 25 20 0 \
        "$(L "数据目录" "Data dir")" 2 1 "$def_data" 2 25 60 0
    )" || return 1
    IFS=$'\n' read -r listen_addr data_dir <<<"$form"
  else
    listen_addr="$(prompt "$(L "监听地址" "Listen addr")" "$def_listen")"
    data_dir="$(prompt "$(L "数据目录" "Data dir")" "$def_data")"
  fi

  local port old_port
  port="$(parse_port_from_listen_addr "$listen_addr" || true)"
  if [ -z "$port" ]; then
    echo "$(L "错误：listen_addr 端口解析失败（例如 :38088）。" "ERROR: failed to parse listen_addr port (e.g. :38088).")" >&2
    exit 1
  fi
  old_port="$(parse_port_from_listen_addr "$def_listen" || true)"
  if [ -z "${old_port:-}" ] || [ "$port" != "$old_port" ]; then
    confirm_port_available "$port" "$(L "监听端口" "Listen port")" || exit 1
  fi
  if [ -z "${data_dir:-}" ]; then
    echo "$(L "错误：数据目录不能为空。" "ERROR: data dir must not be empty.")" >&2
    exit 1
  fi

  if [ -z "$ingest_token" ]; then had_ingest="false"; ingest_token="$(gen_token)"; fi
  if [ -z "$admin_user" ]; then admin_user="admin"; fi
  if [ -z "$admin_password" ]; then
    had_admin="false"
    admin_changed="true"
    admin_password_changed="true"
    admin_user="$(prompt "$(L "管理面板用户名" "Admin username")" "$admin_user")"
    admin_password="$(prompt "$(L "管理面板密码（留空自动生成）" "Admin password (leave empty to auto-generate)")" "")"
    if [ -z "$admin_password" ]; then
      admin_password="$(gen_token)"
    fi
  else
    local change_admin
    change_admin="$(prompt_yesno "$(L "是否修改管理面板账号/密码？" "Change admin username/password?")" "n")"
    if [ "$change_admin" = "true" ]; then
      local old_admin_password new_admin_password
      old_admin_password="$admin_password"
      admin_changed="true"
      admin_user="$(prompt "$(L "管理面板用户名" "Admin username")" "$admin_user")"
      new_admin_password="$(prompt "$(L "管理面板密码（留空保持不变；输入 AUTO 自动生成）" "Admin password (empty=keep; AUTO=generate)")" "")"
      case "${new_admin_password:-}" in
        "" )
          # keep
          ;;
        AUTO|auto|Auto )
          admin_password="$(gen_token)"
          admin_password_changed="true"
          ;;
        * )
          admin_password="$new_admin_password"
          admin_password_changed="true"
          ;;
      esac

      # Keep enroll_token in sync if it was sharing the same secret.
      if [ -n "$enroll_token" ] && [ "$enroll_token" = "$old_admin_password" ] && [ "$admin_password_changed" = "true" ]; then
        enroll_token="$admin_password"
      fi
    fi
  fi
  if [ -z "$enroll_token" ]; then had_enroll="false"; enroll_token=""; fi

  if [ -z "$ingest_token" ] || [ -z "$admin_password" ]; then
    echo "$(L "错误：密码生成失败（建议安装 openssl）或请手动输入。" "ERROR: failed to generate passwords (install openssl) or input manually.")" >&2
    exit 1
  fi

  $SUDO mkdir -p "$data_dir"

  local tmp; tmp="$(mktemp)"
  cat >"$tmp" <<EOF
{
  "listen_addr": "$(json_escape "$listen_addr")",
  "data_dir": "$(json_escape "$data_dir")",
  "ingest_token": "$(json_escape "$ingest_token")",
  "admin_user": "$(json_escape "$admin_user")",
  "admin_password": "$(json_escape "$admin_password")",
  "enroll_token": "$(json_escape "$enroll_token")",
  "enroll_max_fails": 5,
  "enroll_ban_hours": 8,
  "trust_proxy": false,
  "allow_auto_register": $allow_auto,
  "stealth_ingest_unauthorized": $stealth,
  "default_collect_interval_seconds": 5,
  "default_retention_days": 30,
  "dashboard_poll_seconds": 3
}
EOF
  $SUDO install -m 0600 "$tmp" "$cfg"
  rm -f "$tmp"
  echo "$(L "已写入配置：" "Wrote config: ")$cfg"
  if [ "$admin_changed" = "true" ] || [ "$had_ingest" = "false" ] || [ "$had_enroll" = "false" ]; then
    echo "$(L "管理面板用户名：" "Admin username: ")$admin_user"
    if [ "$admin_password_changed" = "true" ] || [ "$had_admin" = "false" ]; then
      echo "$(L "管理面板密码（请保存）：" "Admin password (save it): ")$admin_password"
    fi
    if [ -n "$enroll_token" ] && [ "$enroll_token" != "$admin_password" ]; then
      echo "$(L "客户端接入密码（覆盖中心密码）：" "Probe enroll password (override): ")$enroll_token"
    fi
  fi
  if [ "$admin_changed" = "true" ]; then
    echo "$(L "提示：如修改了管理账号/密码，请重启中心端以生效（菜单 6. 重启 服务器）。" "Note: if you changed admin login, restart the center to apply (menu 6 Restart Server).")"
  else
    echo "$(L "提示：管理面板账号/密码未变更（如需查看：./run.sh server show-config 或 cat 配置文件）。" "Note: admin login unchanged (view via ./run.sh server show-config or cat the config file).")"
  fi
}

center_data_dir_default() {
  # Prefer keeping data close to the installed backend for simple deployments.
  echo "$BASE_DIR/data"
}

write_center_config_quick() {
  local SUDO; SUDO="$(sudo_prefix)"
  ensure_dirs

  local port data_dir listen_addr ingest_token admin_user admin_password enroll_token

  port="$(prompt "$(L "监听端口" "Listen port")" "38088")"
  if ! validate_port "$port"; then
    echo "$(L "错误：端口不合法（1-65535）。" "ERROR: invalid port (1-65535).")" >&2
    exit 1
  fi
  confirm_port_available "$port" "$(L "监听端口" "Listen port")" || exit 1
  listen_addr=":${port}"

  data_dir="$(prompt "$(L "数据目录（默认：后端目录下 data/）" "Data dir (default: data/ under backend dir)")" "$(center_data_dir_default)")"
  if [ -z "$data_dir" ]; then
    echo "$(L "错误：数据目录不能为空。" "ERROR: data dir must not be empty.")" >&2
    exit 1
  fi

  admin_user="$(prompt "$(L "管理面板用户名" "Admin username")" "admin")"
  ingest_token="$(prompt "$(L "中心密码（用于管理面板登录 + 客户端上报）（留空自动生成）" "Center password (for /admin login + probe push) (empty=auto-generate)")" "")"
  if [ -z "$ingest_token" ]; then
    ingest_token="$(gen_token)"
  fi
  admin_password="$ingest_token"
  enroll_token=""

  if [ -z "$ingest_token" ] || [ -z "$admin_password" ]; then
    echo "$(L "错误：密码生成失败（建议安装 openssl）或请手动在配置文件中填写。" "ERROR: failed to generate passwords (install openssl) or fill them manually in config.")" >&2
    exit 1
  fi

  $SUDO mkdir -p "$data_dir"

  local cfg; cfg="$(config_path_for center)"
  local tmp; tmp="$(mktemp)"
  cat >"$tmp" <<EOF
{
  "listen_addr": "$(json_escape "$listen_addr")",
  "data_dir": "$(json_escape "$data_dir")",
  "ingest_token": "$(json_escape "$ingest_token")",
  "admin_user": "$(json_escape "$admin_user")",
  "admin_password": "$(json_escape "$admin_password")",
  "enroll_token": "$(json_escape "$enroll_token")",
  "enroll_max_fails": 5,
  "enroll_ban_hours": 8,
  "trust_proxy": false,
  "allow_auto_register": true,
  "stealth_ingest_unauthorized": true,
  "default_collect_interval_seconds": 5,
  "default_retention_days": 30,
  "dashboard_poll_seconds": 3
}
EOF
  $SUDO install -m 0600 "$tmp" "$cfg"
  rm -f "$tmp"

  echo "$(L "已写入配置：" "Wrote config: ")$cfg"
  echo "$(L "面板地址：" "Dashboard: ")http://127.0.0.1:${port}/ $(L "（外网请替换为服务器 IP/域名）" "(replace with server IP/domain for remote access)")"
  echo "$(L "控制面板：" "Admin panel: ")http://127.0.0.1:${port}/admin $(L "（外网请替换为服务器 IP/域名）" "(replace with server IP/domain for remote access)")"
  echo "$(L "管理面板用户名：" "Admin username: ")$admin_user"
  echo "$(L "中心密码（请保存）：" "Center password (save it): ")$admin_password"
}

first_run_wizard() {
  local mode=""
  if use_dialog; then
    mode="$(
      dialog --clear --stdout --title "$SCRIPT_NAME" \
        --menu "$(L "首次初始化：请选择要部署的角色" "First-time setup: choose role")" 12 70 4 \
        center "$(L "服务端（中心节点）" "Server (center)")" \
        agent "$(L "客户端（被监控机）" "Client (probe)")"
    )" || mode=""
  else
    echo
    echo "$(L "检测到尚未创建配置文件，进入首次初始化。" "No config found, entering first-time setup.")"
    echo "  1) $(L "服务端（中心节点）" "Server (center)")"
    echo "  2) $(L "客户端（被监控机）" "Client (probe)")"
    local n=""
    read -r -p "$(L "请选择 [1-2]：" "Select [1-2]: ")" n || true
    case "${n:-}" in
      1) mode="center" ;;
      2) mode="agent" ;;
      *) mode="" ;;
    esac
  fi

  if [ "$mode" = "center" ]; then
    write_center_config_quick
    if use_dialog; then menu_loop_dialog "center"; else menu_loop "center"; fi
    exit 0
  fi
  if [ "$mode" = "agent" ]; then
    write_agent_config_quick
    if use_dialog; then menu_loop_dialog "agent"; else menu_loop "agent"; fi
    exit 0
  fi
}

default_agent_id() {
  local h=""
  if command -v hostname >/dev/null 2>&1; then
    h="$(hostname 2>/dev/null || true)"
  fi
  h="${h:-}"
  h="$(echo "$h" | tr 'A-Z' 'a-z' | tr -cs 'a-z0-9._-' '-' | sed 's/^-*//;s/-*$//')"
  if [ -z "$h" ]; then
    h="probe-$(date +%s 2>/dev/null || echo 0)"
  fi
  echo "$h"
}

normalize_central_url() {
  local u="${1-}"
  u="$(echo "$u" | tr -d ' ')"
  if [ -z "$u" ]; then
    echo ""
    return 0
  fi
  case "$u" in
    http://*|https://*) echo "$u" ;;
    *) echo "http://$u" ;;
  esac
}

write_agent_config_quick() {
  local SUDO; SUDO="$(sudo_prefix)"
  ensure_dirs

  local cfg; cfg="$(config_path_for agent)"

  local agent_id name central_url ingest_token enroll_token collect_interval disk_mount net_iface tcp_conn_enabled
  local port_probe_enabled port_probe_host ports_json

  agent_id="$(default_agent_id)"
  name="$agent_id"
  central_url="$(prompt "$(L "中心地址（例如 1.2.3.4:38088 或 http://1.2.3.4:38088）" "Central URL (e.g. 1.2.3.4:38088 or http://1.2.3.4:38088)")" "")"
  central_url="$(normalize_central_url "$central_url")"
  if [ -z "$central_url" ]; then
    echo "$(L "错误：central_url 必填。" "ERROR: central_url required.")" >&2
    exit 1
  fi

  ingest_token="$(prompt "$(L "上报密码（节点密码）" "Ingest password (node password)")" "")"
  if [ -z "$ingest_token" ]; then
    echo "$(L "错误：密码必填。" "ERROR: password required.")" >&2
    exit 1
  fi
  enroll_token=""

  collect_interval=5
  disk_mount="/"
  net_iface=""
  tcp_conn_enabled="true"
  port_probe_enabled="false"
  port_probe_host="127.0.0.1"
  ports_json="[]"

  local tmp; tmp="$(mktemp)"
  cat >"$tmp" <<EOF
{
  "agent_id": "$(json_escape "$agent_id")",
  "name": "$(json_escape "$name")",
  "central_url": "$(json_escape "$central_url")",
  "ingest_token": "$(json_escape "$ingest_token")",
  "enroll_token": "$(json_escape "$enroll_token")",
  "collect_interval_seconds": $collect_interval,
  "disk_mount": "$(json_escape "$disk_mount")",
  "net_iface": "$(json_escape "$net_iface")",
  "port_probe_enabled": $port_probe_enabled,
  "port_probe_host": "$(json_escape "$port_probe_host")",
  "ports": $ports_json,
  "tcp_conn_enabled": $tcp_conn_enabled
}
EOF
  $SUDO install -m 0600 "$tmp" "$cfg"
  rm -f "$tmp"
  echo "$(L "已写入配置：" "Wrote config: ")$cfg"
}

default_mode_for_noargs() {
  local center_cfg agent_cfg
  center_cfg="$(config_path_for center)"
  agent_cfg="$(config_path_for agent)"
  local center_bin="$BIN_DIR/$(binary_name_for center)"
  local agent_bin="$BIN_DIR/$(binary_name_for agent)"
  local center_local="$SCRIPT_DIR/$(binary_name_for center)"
  local agent_local="$SCRIPT_DIR/$(binary_name_for agent)"

  if [ -f "$center_local" ] && [ ! -f "$agent_local" ]; then
    echo "center"
    return 0
  fi
  if [ -f "$agent_local" ] && [ ! -f "$center_local" ]; then
    echo "agent"
    return 0
  fi
  if [ -f "$center_bin" ] && [ ! -f "$agent_bin" ]; then
    echo "center"
    return 0
  fi
  if [ -f "$agent_bin" ] && [ ! -f "$center_bin" ]; then
    echo "agent"
    return 0
  fi
  if [ -f "$center_cfg" ] && [ ! -f "$agent_cfg" ]; then
    echo "center"
    return 0
  fi
  if [ -f "$agent_cfg" ] && [ ! -f "$center_cfg" ]; then
    echo "agent"
    return 0
  fi
  echo "agent"
}

ensure_config_or_init_then_menu() {
  local mode="$1"
  local cfg=""
  cfg="$(config_path_for "$mode")"
  if [ ! -f "$cfg" ]; then
    if [ "$mode" = "center" ]; then
      write_center_config_quick
    else
      write_agent_config_quick
    fi
  fi
  if use_dialog; then menu_loop_dialog "$mode"; else menu_loop "$mode"; fi
  exit 0
}

ensure_config_for_mode() {
  local mode="$1"
  local cfg; cfg="$(config_path_for "$mode")"
  if [ ! -f "$cfg" ]; then
    if [ "$mode" = "center" ]; then
      write_center_config_quick
    else
      write_agent_config_quick
    fi
    return 0
  fi
  if [ "$mode" = "center" ]; then
    if ! validate_center_config >/dev/null 2>&1; then
      echo "$(L "检测到服务器配置不完整，进入初始化/修复。" "Server config looks invalid; entering setup/repair.")"
      write_center_config
    fi
  else
    if ! validate_agent_config >/dev/null 2>&1; then
      echo "$(L "检测到客户端配置不完整，进入初始化/修复。" "Client config looks invalid; entering setup/repair.")"
      write_agent_config
    fi
  fi
}

parse_int_list() {
  # Input: "22,80,443" => "22 80 443"
  local raw="${1-}"
  raw="${raw//,/ }"
  echo "$raw"
}

validate_port() {
  local p="$1"
  case "$p" in
    *[!0-9]*|"") return 1 ;;
  esac
  [ "$p" -ge 1 ] && [ "$p" -le 65535 ]
}

port_in_use_tcp() {
  local port="$1"
  if ! validate_port "$port"; then
    return 1
  fi
  if command -v ss >/dev/null 2>&1; then
    if ss -lnt 2>/dev/null | awk '{print $4}' | grep -Eq ":${port}\$"; then
      return 0
    fi
    return 1
  fi
  if command -v netstat >/dev/null 2>&1; then
    if netstat -lnt 2>/dev/null | awk '{print $4}' | grep -Eq ":${port}\$"; then
      return 0
    fi
    return 1
  fi
  if command -v lsof >/dev/null 2>&1; then
    if lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
    return 1
  fi
  return 1
}

confirm_port_available() {
  # Warn if a TCP port seems occupied; ask user to confirm continuing.
  local port="$1"
  local label="${2:-port}"
  if port_in_use_tcp "$port"; then
    local msg
    msg="$(L \
      "警告：${label} ${port} 似乎已被占用，可能导致无法启动/访问。仍要继续吗？" \
      "Warning: ${label} ${port} appears to be in use and may prevent startup/access. Continue anyway?")"
    if [ "$(prompt_yesno "$msg" "n")" != "true" ]; then
      echo "$(L "已取消。" "Canceled.")" >&2
      return 1
    fi
  fi
  return 0
}

json_get_string() {
  local file="$1"
  local key="$2"
  grep -Eo "\"${key}\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" "$file" 2>/dev/null | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)".*/\1/' || true
}

json_get_bool() {
  local file="$1"
  local key="$2"
  local v
  v="$(grep -Eo "\"${key}\"[[:space:]]*:[[:space:]]*(true|false)" "$file" 2>/dev/null | head -n1 | sed -E 's/.*:[[:space:]]*(true|false).*/\1/' || true)"
  if [ "$v" = "true" ] || [ "$v" = "false" ]; then
    echo "$v"
  else
    echo ""
  fi
}

json_get_int_array_csv() {
  # Best-effort: parse JSON array like: "ports": [22, 80, 443]
  # Returns CSV string: "22,80,443" (empty if missing).
  local file="$1"
  local key="$2"
  local line=""
  line="$(grep -E "\"${key}\"[[:space:]]*:[[:space:]]*\\[" "$file" 2>/dev/null | head -n1 || true)"
  if [ -z "$line" ]; then
    echo ""
    return 0
  fi
  # If array spans multiple lines, join a few lines until ']' is found.
  if ! echo "$line" | grep -q "]"; then
    line="$(
      awk -v k="\"${key}\"" '
        $0 ~ k {
          s=$0
          for(i=0;i<12;i++){
            if (s ~ /]/) { print s; exit }
            if ((getline t) <= 0) { print s; exit }
            s = s t
          }
          print s
          exit
        }' "$file" 2>/dev/null || true
    )"
  fi
  local inner=""
  inner="$(echo "$line" | sed -E 's/.*\\[(.*)\\].*/\\1/' | tr -d '[:space:]')"
  inner="$(echo "$inner" | tr -cd '0-9,' )"
  echo "$inner"
}

parse_port_from_listen_addr() {
  # Accept: ":38088" "0.0.0.0:38088" "[::]:38088" "127.0.0.1:38088"
  local addr="$1"
  addr="${addr//\"/}"
  addr="${addr// /}"
  local port="${addr##*:}"
  if validate_port "$port"; then
    echo "$port"
    return 0
  fi
  return 1
}

read_center_listen_port() {
  local cfg; cfg="$(config_path_for center)"
  if [ ! -f "$cfg" ]; then return 1; fi
  local addr
  addr="$(json_get_string "$cfg" "listen_addr")"
  if [ -z "$addr" ]; then return 1; fi
  parse_port_from_listen_addr "$addr"
}

read_center_data_dir() {
  local cfg; cfg="$(config_path_for center)"
  if [ ! -f "$cfg" ]; then return 1; fi
  local d
  d="$(json_get_string "$cfg" "data_dir")"
  if [ -z "$d" ]; then return 1; fi
  if [ "${d#/}" != "$d" ]; then
    echo "$d"
    return 0
  fi
  echo "$BASE_DIR/$d"
}

validate_center_config() {
  local cfg; cfg="$(config_path_for center)"
  if [ ! -f "$cfg" ]; then echo "$(L "找不到配置：" "Config not found: ")$cfg" >&2; return 1; fi
  local need=("ingest_token" "data_dir" "listen_addr")
  local k
  for k in "${need[@]}"; do
    if ! grep -q "\"$k\"" "$cfg"; then
      echo "$(L "配置缺少字段：" "Missing config field: ")$k" >&2
      return 1
    fi
  done
  if ! grep -q "\"admin_password\"" "$cfg" && ! grep -q "\"admin_token\"" "$cfg"; then
    echo "$(L "配置缺少字段：" "Missing config field: ")admin_password" >&2
    return 1
  fi
  local port
  port="$(read_center_listen_port || true)"
  if [ -z "$port" ]; then
    echo "$(L "listen_addr 端口解析失败（例如 :38088）" "Failed to parse listen_addr port (e.g. :38088)")" >&2
    return 1
  fi
  return 0
}

validate_agent_config() {
  local cfg; cfg="$(config_path_for agent)"
  if [ ! -f "$cfg" ]; then echo "$(L "找不到配置：" "Config not found: ")$cfg" >&2; return 1; fi
  # agent_id/name can be derived by the agent binary (hostname), keep script validation minimal.
  local need=("central_url")
  local k
  for k in "${need[@]}"; do
    if ! grep -q "\"$k\"" "$cfg"; then
      echo "$(L "配置缺少字段：" "Missing config field: ")$k" >&2
      return 1
    fi
  done
  local ingest enroll
  ingest="$(json_get_string "$cfg" "ingest_token")"
  enroll="$(json_get_string "$cfg" "enroll_token")"
  if [ -z "$ingest" ] && [ -z "$enroll" ]; then
    echo "$(L "配置缺少字段：ingest_token 或 enroll_token 至少一个。" "Missing config: need ingest_token or enroll_token.")" >&2
    return 1
  fi
  return 0
}

write_agent_config() {
  local SUDO; SUDO="$(sudo_prefix)"
  ensure_dirs

  local cfg; cfg="$(config_path_for agent)"
  local agent_id name
  agent_id="$(json_get_string "$cfg" "agent_id" || true)"
  if [ -z "$agent_id" ]; then agent_id="$(default_agent_id)"; fi
  name="$(json_get_string "$cfg" "name" || true)"
  if [ -z "$name" ]; then name="$agent_id"; fi

  local def_central
  def_central="$(json_get_string "$cfg" "central_url" || true)"

  local def_collect def_mount def_iface def_tcp def_port_probe def_port_host def_ports_raw
  def_collect="$(json_get_string "$cfg" "collect_interval_seconds" || true)"
  if [ -z "$def_collect" ]; then def_collect="5"; fi
  def_mount="$(json_get_string "$cfg" "disk_mount" || true)"
  if [ -z "$def_mount" ]; then def_mount="/"; fi
  def_iface="$(json_get_string "$cfg" "net_iface" || true)"
  def_tcp="$(json_get_bool "$cfg" "tcp_conn_enabled" || true)"
  if [ -z "$def_tcp" ]; then def_tcp="true"; fi
  def_port_probe="$(json_get_bool "$cfg" "port_probe_enabled" || true)"
  if [ -z "$def_port_probe" ]; then def_port_probe="false"; fi
  def_port_host="$(json_get_string "$cfg" "port_probe_host" || true)"
  if [ -z "$def_port_host" ]; then def_port_host="127.0.0.1"; fi
  def_ports_raw="$(json_get_int_array_csv "$cfg" "ports" || true)"

  local central_url password ingest_token enroll_token collect_interval disk_mount net_iface tcp_conn_enabled
  local port_probe_enabled port_probe_host ports_raw advanced

  central_url="$(prompt "$(L "中心地址（例如 1.2.3.4:38088 或 http://1.2.3.4:38088）" "Central URL (e.g. 1.2.3.4:38088 or http://1.2.3.4:38088)")" "$def_central")"
  central_url="$(normalize_central_url "$central_url")"
  if [ -z "$central_url" ]; then
    echo "$(L "错误：中心地址必填。" "ERROR: central_url required.")" >&2
    exit 1
  fi

  # Password: keep empty by default to avoid echoing secrets in interactive prompts.
  password="$(prompt "$(L "上报密码（留空保持不变）" "Ingest password (leave empty to keep)")" "")"
  if [ -z "$password" ]; then
    ingest_token="$(json_get_string "$cfg" "ingest_token" || true)"
    enroll_token="$(json_get_string "$cfg" "enroll_token" || true)"
  else
    # Simplest mode: treat the entered password as ingest_token and disable enroll.
    ingest_token="$password"
    enroll_token=""
  fi
  if [ -z "${ingest_token:-}" ] && [ -z "${enroll_token:-}" ]; then
    echo "$(L "错误：密码必填。" "ERROR: password required.")" >&2
    exit 1
  fi

  advanced="$(prompt_yesno "$(L "高级设置（可选：采集间隔/磁盘/网卡/端口探活）？" "Advanced settings (optional: interval/disk/iface/port probe)?")" "n")"

  collect_interval="$def_collect"
  disk_mount="$def_mount"
  net_iface="$def_iface"
  tcp_conn_enabled="$def_tcp"
  port_probe_enabled="$def_port_probe"
  port_probe_host="$def_port_host"
  ports_raw="$def_ports_raw"

  if [ "$advanced" = "true" ]; then
    collect_interval="$(prompt "$(L "采集间隔（秒）（通常建议在控制面板设置）" "Collect interval seconds (usually set in admin)")" "$def_collect")"
    disk_mount="$(prompt "$(L "磁盘挂载点" "Disk mount")" "$def_mount")"
    net_iface="$(prompt "$(L "网卡名（留空自动选择）" "Network iface (empty = auto)")" "$def_iface")"
    local def_tcp_yn="y"
    if [ "$def_tcp" = "false" ]; then def_tcp_yn="n"; fi
    tcp_conn_enabled="$(prompt_yesno "$(L "上报 TCP 连接数（通常建议在控制面板设置）" "Report TCP connection counts (usually set in admin)")" "$def_tcp_yn")"
    local def_pp_yn="n"
    if [ "$def_port_probe" = "true" ]; then def_pp_yn="y"; fi
    port_probe_enabled="$(prompt_yesno "$(L "开启端口探活（TCP connect）（通常建议在控制面板设置）" "Enable port probe (TCP connect) (usually set in admin)")" "$def_pp_yn")"
    if [ "$port_probe_enabled" = "true" ]; then
      port_probe_host="$(prompt "$(L "端口探活 Host" "Port probe host")" "$def_port_host")"
      ports_raw="$(prompt "$(L "端口列表（逗号分隔）" "Ports (comma-separated)")" "${def_ports_raw:-22,80,443}")"
    else
      port_probe_host="$def_port_host"
      ports_raw=""
    fi
  fi

  # Build ports JSON array.
  local ports_json="["
  local first=1
  for p in $(parse_int_list "$ports_raw"); do
    [ -z "$p" ] && continue
    case "$p" in
      *[!0-9]*) continue ;;
    esac
    if [ "$first" -eq 1 ]; then first=0; else ports_json="${ports_json}, "; fi
    ports_json="${ports_json}${p}"
  done
  ports_json="${ports_json}]"

  local tmp; tmp="$(mktemp)"
  cat >"$tmp" <<EOF
{
  "agent_id": "$(json_escape "$agent_id")",
  "name": "$(json_escape "$name")",
  "central_url": "$(json_escape "$central_url")",
  "ingest_token": "$(json_escape "${ingest_token:-}")",
  "enroll_token": "$(json_escape "${enroll_token:-}")",
  "collect_interval_seconds": $collect_interval,
  "disk_mount": "$(json_escape "$disk_mount")",
  "net_iface": "$(json_escape "$net_iface")",
  "port_probe_enabled": $port_probe_enabled,
  "port_probe_host": "$(json_escape "$port_probe_host")",
  "ports": $ports_json,
  "tcp_conn_enabled": $tcp_conn_enabled
}
EOF
  $SUDO install -m 0600 "$tmp" "$cfg"
  rm -f "$tmp"
  echo "$(L "已写入配置：" "Wrote config: ")$cfg"
}

write_systemd_unit() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local unit; unit="$(unit_path_for "$svc")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  local cfg; cfg="$(config_path_for "$mode")"
  local work="$BASE_DIR"

  ensure_binary_installed "$mode"
  if [ ! -f "$cfg" ]; then
    echo "$(L "错误：找不到配置：" "ERROR: config not found: ")$cfg $(L "（请先运行：" "(run:") ./run.sh $mode configure$(L "）" ")")" >&2
    exit 1
  fi

  local tmp; tmp="$(mktemp)"
  cat >"$tmp" <<EOF
[Unit]
Description=$svc
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$work
ExecStart=$bin -config $cfg
Restart=always
RestartSec=2
NoNewPrivileges=true
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
  $SUDO install -m 0644 "$tmp" "$unit"
  rm -f "$tmp"
  $SUDO systemctl daemon-reload
  $SUDO systemctl enable "$svc" >/dev/null || true
  echo "$(L "已安装 systemd 服务：" "Installed systemd unit: ")$unit"
}

enable_autostart_nosystemd() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  local cfg; cfg="$(config_path_for "$mode")"
  if [ ! -x "$bin" ] || [ ! -f "$cfg" ]; then
    echo "$(L "请先安装并配置后再启用自启动。" "Install and configure first before enabling autostart.")" >&2
    return 1
  fi
  if ! command -v crontab >/dev/null 2>&1; then
    echo "$(L "缺少 crontab 命令，无法设置 @reboot。" "Missing crontab; cannot set @reboot.")" >&2
    return 1
  fi
  local line="@reboot $bin -config $cfg >>$(log_path_for "$svc") 2>&1"
  local tmp; tmp="$(mktemp)"
  crontab -l 2>/dev/null | grep -vF "$line" >"$tmp" || true
  echo "$line" >>"$tmp"
  $SUDO crontab "$tmp"
  rm -f "$tmp"
  echo "$(L "已设置开机自启动（crontab @reboot）。" "Autostart enabled via crontab @reboot.")"
}

disable_autostart_nosystemd() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  local cfg; cfg="$(config_path_for "$mode")"
  if ! command -v crontab >/dev/null 2>&1; then
    echo "$(L "缺少 crontab 命令。" "Missing crontab.")" >&2
    return 1
  fi
  local line="@reboot $bin -config $cfg >>$(log_path_for "$svc") 2>&1"
  local tmp; tmp="$(mktemp)"
  crontab -l 2>/dev/null | grep -vF "$line" >"$tmp" || true
  $SUDO crontab "$tmp" || true
  rm -f "$tmp"
  echo "$(L "已移除开机自启动（crontab @reboot）。" "Autostart removed from crontab @reboot.")"
}

nohup_start() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  local cfg; cfg="$(config_path_for "$mode")"
  local pidf; pidf="$(pid_path_for "$svc")"
  local logf; logf="$(log_path_for "$svc")"

  if [ ! -f "$cfg" ]; then
    echo "$(L "错误：找不到配置：" "ERROR: config not found: ")$cfg $(L "（请先运行：" "(run:") ./run.sh $mode configure$(L "）" ")")" >&2
    exit 1
  fi
  ensure_binary_installed "$mode"

  if [ -f "$pidf" ] && kill -0 "$(cat "$pidf" 2>/dev/null || echo 0)" 2>/dev/null; then
    echo "$(L "已在运行（pid " "Already running (pid ")$(cat "$pidf")$(L "）。" ").")"
    exit 0
  fi

  $SUDO mkdir -p "$(dirname "$pidf")" "$LOG_DIR_DEFAULT" >/dev/null || true
  $SUDO nohup "$bin" -config "$cfg" >>"$logf" 2>&1 &
  local pid=$!
  echo "$pid" | $SUDO tee "$pidf" >/dev/null
  echo "$(L "已启动（nohup）pid=" "Started (nohup) pid=")$pid $(L "日志：" "log=")$logf"
}

nohup_stop() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local pidf; pidf="$(pid_path_for "$svc")"
  if [ ! -f "$pidf" ]; then
    echo "$(L "未运行。" "Not running.")"
    exit 0
  fi
  local pid; pid="$(cat "$pidf" 2>/dev/null || echo "")"
  if [ -z "$pid" ]; then
    echo "$(L "未运行。" "Not running.")"
    exit 0
  fi
  if kill -0 "$pid" 2>/dev/null; then
    $SUDO kill "$pid" || true
    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
      $SUDO kill -9 "$pid" || true
    fi
    echo "$(L "已停止 pid=" "Stopped pid=")$pid"
  else
    echo "$(L "未运行。" "Not running.")"
  fi
  $SUDO rm -f "$pidf" || true
}

status() {
  local mode="$1"
  local svc; svc="$(service_name_for "$mode")"
  local unit; unit="$(unit_path_for "$svc")"
  if have_systemd && [ -f "$unit" ]; then
    local SUDO; SUDO="$(sudo_prefix)"
    $SUDO systemctl status "$svc" --no-pager || true
    return 0
  fi
  local pidf; pidf="$(pid_path_for "$svc")"
  if [ -f "$pidf" ]; then
    local pid; pid="$(cat "$pidf" 2>/dev/null || echo "")"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      echo "$(L "运行中（pid " "Running (pid ")$pid$(L "），方式：nohup。" ") via nohup.")"
      echo "$(L "日志：" "Log: ")$(log_path_for "$svc")"
      return 0
    fi
  fi
  echo "$(L "未运行。" "Not running.")"
}

is_installed() {
  local mode="$1"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  [ -x "$bin" ]
}

is_running() {
  local mode="$1"
  local svc; svc="$(service_name_for "$mode")"
  local unit; unit="$(unit_path_for "$svc")"
  if have_systemd && [ -f "$unit" ]; then
    # Avoid sudo here to prevent password prompts in menus.
    systemctl is-active --quiet "$svc" 2>/dev/null
    return $?
  fi
  local pidf; pidf="$(pid_path_for "$svc")"
  if [ -f "$pidf" ]; then
    local pid; pid="$(cat "$pidf" 2>/dev/null || echo "")"
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
    return $?
  fi
  return 1
}

print_state_line() {
  local mode="$1"
  local installedText runningText
  if is_installed "$mode"; then
    installedText="$(C_GREEN "$(L "已安装" "installed")")"
  else
    installedText="$(C_RED "$(L "未安装" "not installed")")"
  fi
  if is_running "$mode"; then
    runningText="$(C_GREEN "$(L "已启动" "running")")"
  else
    runningText="$(C_RED "$(L "未启用" "stopped")")"
  fi
  echo ""
  if [ "$TZ_LANG" = "en" ]; then
    echo "Current status: $(mode_label "$mode") is $installedText and $runningText"
  else
    echo "当前状态: $(mode_label "$mode") ${installedText} 并 ${runningText}"
  fi
  local cfg; cfg="$(config_path_for "$mode")"
  if [ -f "$cfg" ]; then
    echo "$(L "配置文件：" "Config: ")$cfg"
  else
    echo "$(L "配置文件：" "Config: ")$cfg $(L "（不存在）" "(missing)")"
  fi
  echo ""
}

show_info() {
  local mode="$1"
  local svc; svc="$(service_name_for "$mode")"
  local bin="$BIN_DIR/$(binary_name_for "$mode")"
  local cfg; cfg="$(config_path_for "$mode")"
  local unit; unit="$(unit_path_for "$svc")"
  local text=""
  text="${text}mode: $mode\n"
  text="${text}service: $svc\n"
  text="${text}binary: $bin\n"
  text="${text}config: $cfg\n"
  if [ -f "$cfg" ]; then
    if [ "$mode" = "center" ]; then
      local listen_addr port
      listen_addr="$(json_get_string "$cfg" "listen_addr" || true)"
      port="$(parse_port_from_listen_addr "$listen_addr" || true)"
      if [ -n "${port:-}" ]; then
        text="${text}dashboard: http://127.0.0.1:${port}/ (replace host for remote)\n"
        text="${text}admin: http://127.0.0.1:${port}/admin\n"
      fi

      local admin_user admin_password ingest_token enroll_token
      admin_user="$(json_get_string "$cfg" "admin_user" || true)"
      if [ -z "$admin_user" ]; then admin_user="admin"; fi
      admin_password="$(json_get_string "$cfg" "admin_password" || true)"
      if [ -z "$admin_password" ]; then
        admin_password="$(json_get_string "$cfg" "admin_token" || true)"
      fi
      ingest_token="$(json_get_string "$cfg" "ingest_token" || true)"
      enroll_token="$(json_get_string "$cfg" "enroll_token" || true)"

      if [ -n "${admin_user:-}" ]; then text="${text}admin_user: $admin_user\n"; fi
      if [ -n "${admin_password:-}" ]; then text="${text}admin_password: $admin_password\n"; fi
      if [ -n "${enroll_token:-}" ]; then text="${text}enroll_token: $enroll_token\n"; fi
      if [ -n "${ingest_token:-}" ]; then text="${text}ingest_token: $ingest_token\n"; fi
    else
      local agent_id name central_url ingest_token enroll_token
      agent_id="$(json_get_string "$cfg" "agent_id" || true)"
      name="$(json_get_string "$cfg" "name" || true)"
      central_url="$(json_get_string "$cfg" "central_url" || true)"
      ingest_token="$(json_get_string "$cfg" "ingest_token" || true)"
      enroll_token="$(json_get_string "$cfg" "enroll_token" || true)"
      if [ -n "${agent_id:-}" ]; then text="${text}agent_id: $agent_id\n"; fi
      if [ -n "${name:-}" ]; then text="${text}name: $name\n"; fi
      if [ -n "${central_url:-}" ]; then text="${text}central_url: $central_url\n"; fi
      if [ -n "${enroll_token:-}" ]; then text="${text}enroll_token: $enroll_token\n"; fi
      if [ -n "${ingest_token:-}" ]; then text="${text}ingest_token: $ingest_token\n"; fi
    fi
  fi
  if have_systemd && [ -f "$unit" ]; then
    text="${text}log: journalctl -u $svc\n"
  else
    text="${text}log: $(log_path_for "$svc")\n"
  fi
  if have_systemd; then
    text="${text}systemd: yes\n"
    if [ -f "$unit" ]; then text="${text}unit: $unit\n"; fi
  else
    text="${text}systemd: no\n"
  fi

  if use_dialog; then
    dialog --clear --title "$(L "信息" "Info")" --msgbox "$(printf "%b" "$text")" 18 80 || true
  else
    echo ""
    echo "$(L "信息：" "Info:")"
    printf "%b" "$text"
  fi
  pause_enter
}

show_logs() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local unit; unit="$(unit_path_for "$svc")"
  if have_systemd && [ -f "$unit" ]; then
    if use_dialog; then
      local tmp; tmp="$(mktemp)"
      trap 'rm -f "$tmp"' RETURN
      $SUDO journalctl -u "$svc" -n 200 --no-pager >"$tmp" 2>&1 || true
      dialog_textbox "$(L "日志" "Logs")" "$tmp"
    else
      $SUDO journalctl -u "$svc" -n 200 --no-pager || true
    fi
  else
    local logf; logf="$(log_path_for "$svc")"
    if [ -f "$logf" ]; then
      if use_dialog; then
        dialog_textbox "$(L "日志" "Logs")" "$logf"
      else
        tail -n 200 "$logf" || true
      fi
    else
      echo "$(L "暂无日志文件：" "No log file: ")$logf"
    fi
  fi
  pause_enter
}

follow_logs() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local unit; unit="$(unit_path_for "$svc")"
  if have_systemd && [ -f "$unit" ]; then
    echo "$(L "按 Ctrl+C 退出日志跟随。" "Press Ctrl+C to stop following logs.")"
    $SUDO journalctl -u "$svc" -n 200 -f --no-pager || true
    return 0
  fi
  local logf; logf="$(log_path_for "$svc")"
  if [ ! -f "$logf" ]; then
    echo "$(L "暂无日志文件：" "No log file: ")$logf"
    return 1
  fi
  echo "$(L "按 Ctrl+C 退出日志跟随。" "Press Ctrl+C to stop following logs.")"
  tail -n 200 -f "$logf" || true
}

install_deps() {
  local mode="$1"
  check_sys
  need_root_for "install dependencies"
  local SUDO; SUDO="$(sudo_prefix)"

  local pkgs=()
  pkgs+=(ca-certificates)
  pkgs+=(curl wget)
  pkgs+=(openssl)
  pkgs+=(dialog)

  if [ "$mode" = "center" ]; then
    pkgs+=(iptables)
  fi

  echo "$(L "系统：" "System: ")$release / $pkg_mgr"

  case "$pkg_mgr" in
    apt)
      $SUDO apt-get update -y
      $SUDO apt-get install -y "${pkgs[@]}" || true
      ;;
    dnf)
      $SUDO dnf makecache -y || true
      $SUDO dnf install -y "${pkgs[@]}" || true
      ;;
    yum)
      $SUDO yum makecache -y || true
      $SUDO yum install -y "${pkgs[@]}" || true
      ;;
    apk)
      $SUDO apk add --no-cache "${pkgs[@]}" || true
      ;;
    *)
      echo "$(L "不支持的包管理器，无法自动安装依赖。" "Unsupported package manager; cannot auto-install dependencies.")" >&2
      return 1
      ;;
  esac

  echo "$(L "依赖安装完成（若有失败请手动安装缺失项）。" "Dependencies installation finished (install missing ones manually if needed).")"
  return 0
}

firewall_menu() {
  local mode="$1"
  if [ "$mode" != "center" ]; then
    echo "$(L "防火墙菜单仅适用于服务器端（接收客户端上报的端口）。" "Firewall menu is only for server mode (inbound port).")" >&2
    return 1
  fi
  need_root_for "firewall rules"
  local port
  port="$(read_center_listen_port || true)"
  if [ -z "$port" ]; then
    port="$(prompt "$(L "请输入要放行的端口" "Enter port to allow")" "38088")"
  fi
  if ! validate_port "$port"; then
    echo "$(L "端口无效。" "Invalid port.")" >&2
    return 1
  fi

  local SUDO; SUDO="$(sudo_prefix)"
  echo "$(L "将放行 TCP 端口：" "Will allow TCP port: ")$port"
  $SUDO iptables -I INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null || true
  if command -v ip6tables >/dev/null 2>&1; then
    $SUDO ip6tables -I INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null || true
  fi

  echo "$(L "已添加规则（持久化请按系统自行保存）。" "Rule added (persist it via your distro tooling if needed).")"
  return 0
}

firewall_persist_hint() {
  local mode="$1"
  if [ "$mode" != "center" ]; then
    echo "$(L "防火墙持久化提示仅适用于服务器端。" "Firewall persistence hints are only for server mode.")" >&2
    return 1
  fi
  check_sys
  local port
  port="$(read_center_listen_port || true)"
  if [ -z "$port" ]; then
    port="$(prompt "$(L "请输入端口" "Enter port")" "38088")"
  fi
  if ! validate_port "$port"; then
    echo "$(L "端口无效。" "Invalid port.")" >&2
    return 1
  fi

  echo ""
  echo "$(L "防火墙持久化建议（示例端口：" "Firewall persistence hints (example port: ")$port$(L "）" "):")"

  if command -v firewall-cmd >/dev/null 2>&1; then
    echo "  - firewalld:"
    echo "    firewall-cmd --add-port=${port}/tcp --permanent"
    echo "    firewall-cmd --reload"
    return 0
  fi

  case "$pkg_mgr" in
    apt)
      echo "  - Debian/Ubuntu (iptables-persistent):"
      echo "    apt-get update && apt-get install -y iptables-persistent"
      echo "    netfilter-persistent save"
      ;;
    yum|dnf)
      echo "  - RHEL/CentOS family:"
      echo "    (if legacy iptables service exists) service iptables save"
      echo "    or use your distro firewall tooling (firewalld/nftables)."
      ;;
    apk)
      echo "  - Alpine:"
      echo "    Use your firewall setup to persist rules (e.g. local scripts or nftables)."
      ;;
    *)
      echo "  - $(L "未知发行版" "Unknown distro"): persist via your firewall tooling."
      ;;
  esac
  echo "  - $(L "提示：新系统可能默认用 nftables；此脚本仅做 iptables 插入。" "Note: newer distros may use nftables by default; this script only inserts iptables rules.")"
  return 0
}

self_check() {
  local mode="$1"
  check_sys

  if [ "$mode" = "center" ]; then
    validate_center_config || return 1
    local port
    port="$(read_center_listen_port || true)"
    echo "$(L "自检（服务器）" "Self-check (server)")"
    echo "  - $(L "端口" "Port"): $port"
    if is_running "$mode"; then
      echo "  - $(L "服务状态" "Service state"): $(C_GREEN "$(L "运行中" "running")")"
    else
      echo "  - $(L "服务状态" "Service state"): $(C_RED "$(L "未运行" "not running")")"
    fi
    if command -v ss >/dev/null 2>&1; then
      ss -lnt 2>/dev/null | grep -q ":${port} " && echo "  - listen: $(C_GREEN OK)" || echo "  - listen: $(C_RED FAIL)"
    elif command -v netstat >/dev/null 2>&1; then
      netstat -lnt 2>/dev/null | grep -q ":${port} " && echo "  - listen: $(C_GREEN OK)" || echo "  - listen: $(C_RED FAIL)"
    else
      echo "  - listen: $(L "跳过（缺少 ss/netstat）" "skipped (missing ss/netstat)")"
    fi
    local base="http://127.0.0.1:${port}"
    if command -v curl >/dev/null 2>&1; then
      if curl -fsS -m 3 "${base}/api/snapshot" | grep -q '"ok"[[:space:]]*:[[:space:]]*true'; then
        echo "  - GET /api/snapshot: $(C_GREEN OK)"
      else
        echo "  - GET /api/snapshot: $(C_RED FAIL)"
      fi
    elif command -v wget >/dev/null 2>&1; then
      if wget -qO- --timeout=3 "${base}/api/snapshot" | grep -q '"ok"[[:space:]]*:[[:space:]]*true'; then
        echo "  - GET /api/snapshot: $(C_GREEN OK)"
      else
        echo "  - GET /api/snapshot: $(C_RED FAIL)"
      fi
    else
      echo "  - GET /api/snapshot: $(L "跳过（缺少 curl/wget）" "skipped (missing curl/wget)")"
    fi
    return 0
  fi

  # client mode
  validate_agent_config || return 1
  local cfg; cfg="$(config_path_for agent)"
  local central ingest enroll agent_id
  central="$(json_get_string "$cfg" "central_url")"
  agent_id="$(json_get_string "$cfg" "agent_id")"
  if [ -z "$agent_id" ]; then agent_id="$(default_agent_id)"; fi
  ingest="$(json_get_string "$cfg" "ingest_token")"
  enroll="$(json_get_string "$cfg" "enroll_token")"
  echo "$(L "自检（客户端）" "Self-check (client)")"
  echo "  - central_url: $central"
  echo "  - agent_id: $agent_id"

  local snapshot_url="${central%/}/api/snapshot"
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS -m 3 "$snapshot_url" | grep -q '"ok"[[:space:]]*:[[:space:]]*true'; then
      echo "  - GET /api/snapshot: $(C_GREEN OK)"
    else
      echo "  - GET /api/snapshot: $(C_RED FAIL)"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if wget -qO- --timeout=3 "$snapshot_url" | grep -q '"ok"[[:space:]]*:[[:space:]]*true'; then
      echo "  - GET /api/snapshot: $(C_GREEN OK)"
    else
      echo "  - GET /api/snapshot: $(C_RED FAIL)"
    fi
  else
    echo "  - GET /api/snapshot: $(L "跳过（缺少 curl/wget）" "skipped (missing curl/wget)")"
  fi

  if [ -n "$ingest" ]; then
    # Password check via ingest (unencrypted, still valid).
    local ts
    ts="$(date +%s 2>/dev/null || echo 0)"
    ts="$((ts * 1000))"
    local payload
    payload="{\"agent_id\":\"${agent_id}\",\"name\":\"${agent_id}\",\"ts_ms\":${ts},\"meta\":{},\"metrics\":{}}"
    local ingest_url="${central%/}/api/ingest"
    if command -v curl >/dev/null 2>&1; then
      if curl -fsS -m 4 -H "X-Ingest-Token: ${ingest}" -H "X-Agent-ID: ${agent_id}" -H "Content-Type: application/json" -d "$payload" "$ingest_url" | grep -q '"ok"[[:space:]]*:[[:space:]]*true'; then
        echo "  - POST /api/ingest (password): $(C_GREEN OK)"
      else
        echo "  - POST /api/ingest (password): $(C_RED FAIL) $(L "（静默模式下密码错通常表现为无响应/EOF）" "(stealth mode may look like EOF if password is wrong)")"
      fi
    elif command -v wget >/dev/null 2>&1; then
      # wget doesn't easily add custom header+post JSON reliably across busybox; skip.
      echo "  - POST /api/ingest: $(L "跳过（建议安装 curl）" "skipped (install curl recommended)")"
    else
      echo "  - POST /api/ingest: $(L "跳过（缺少 curl）" "skipped (missing curl)")"
    fi
  elif [ -n "$enroll" ]; then
    # Enroll password check (will return per-agent ingest_token).
    local enroll_url="${central%/}/api/enroll"
    local body
    body="{\"agent_id\":\"${agent_id}\",\"name\":\"${agent_id}\"}"
    if command -v curl >/dev/null 2>&1; then
      if curl -fsS -m 4 -H "X-Enroll-Token: ${enroll}" -H "Content-Type: application/json" -d "$body" "$enroll_url" | grep -q '"ok"[[:space:]]*:[[:space:]]*true'; then
        echo "  - POST /api/enroll (password): $(C_GREEN OK)"
      else
        echo "  - POST /api/enroll (password): $(C_RED FAIL)"
      fi
    elif command -v wget >/dev/null 2>&1; then
      echo "  - POST /api/enroll: $(L "跳过（建议安装 curl）" "skipped (install curl recommended)")"
    else
      echo "  - POST /api/enroll: $(L "跳过（缺少 curl）" "skipped (missing curl)")"
    fi
  else
    echo "  - $(L "密码校验：跳过（配置中未找到 ingest_token/enroll_token）" "Password check: skipped (no ingest_token/enroll_token in config)")"
  fi
  return 0
}

export_backup() {
  local mode="$1"
  need_root_for "export backup"
  local SUDO; SUDO="$(sudo_prefix)"
  if ! command -v tar >/dev/null 2>&1; then
    echo "$(L "缺少 tar，无法导出。" "Missing tar; cannot export.")" >&2
    return 1
  fi

  local ts out
  ts="$(date +%Y%m%d%H%M%S 2>/dev/null || echo "backup")"
  local mode_name="$mode"
  if [ "$mode" = "agent" ]; then mode_name="probe"; fi
  out="$(prompt "$(L "导出文件路径" "Export file path")" "./tanzhen-${mode_name}-${ts}.tar.gz")"
  if [ -z "$out" ]; then return 1; fi

  local tmp; tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  mkdir -p "$tmp/config"

  if [ "$mode" = "center" ]; then
    local cfg; cfg="$(config_path_for center)"
    if [ -f "$cfg" ]; then $SUDO cp -f "$cfg" "$tmp/config/center.json"; fi
    local data_dir
    data_dir="$(read_center_data_dir || true)"
    if [ -n "$data_dir" ] && [ -d "$data_dir" ]; then
      mkdir -p "$tmp/data"
      $SUDO cp -a "$data_dir/." "$tmp/data/" 2>/dev/null || true
    fi
  else
    local cfg; cfg="$(config_path_for agent)"
    if [ -f "$cfg" ]; then
      $SUDO cp -f "$cfg" "$tmp/config/probe.json"
      # Backward-compatible filename in archive.
      $SUDO cp -f "$cfg" "$tmp/config/agent.json" 2>/dev/null || true
    fi
  fi

  echo "$(L "正在打包..." "Packing...")"
  tar -czf "$out" -C "$tmp" .
  echo "$(L "已导出：" "Exported: ")$out"
  return 0
}

restore_backup() {
  local mode="$1"
  need_root_for "restore backup"
  local SUDO; SUDO="$(sudo_prefix)"
  if ! command -v tar >/dev/null 2>&1; then
    echo "$(L "缺少 tar，无法导入。" "Missing tar; cannot restore.")" >&2
    return 1
  fi

  local in
  in="$(prompt "$(L "备份文件路径" "Backup file path")" "")"
  if [ -z "$in" ] || [ ! -f "$in" ]; then
    echo "$(L "文件不存在。" "File not found.")" >&2
    return 1
  fi

  local tmp; tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  tar -xzf "$in" -C "$tmp"

  stop "$mode" || true

  if [ "$mode" = "center" ]; then
    if [ -f "$tmp/config/center.json" ]; then
      $SUDO install -m 0600 "$tmp/config/center.json" "$(config_path_for center)"
      echo "$(L "已恢复配置：" "Restored config: ")$(config_path_for center)"
    elif [ -f "$tmp/etc/tanzhen/center.json" ]; then
      # Backward compatible: old archive layout
      $SUDO install -m 0600 "$tmp/etc/tanzhen/center.json" "$(config_path_for center)"
      echo "$(L "已恢复配置：" "Restored config: ")$(config_path_for center)"
    fi
    local data_dir
    data_dir="$(read_center_data_dir || true)"
    if [ -n "$data_dir" ] && [ -d "$tmp/data" ]; then
      local yn
      yn="$(prompt_yesno "$(L "是否恢复数据目录（会覆盖现有文件）" "Restore data dir (will overwrite existing files)?")" "n")"
      if [ "$yn" = "true" ]; then
        $SUDO mkdir -p "$data_dir"
        $SUDO cp -a "$tmp/data/." "$data_dir/" 2>/dev/null || true
        echo "$(L "已恢复数据目录：" "Restored data dir: ")$data_dir"
      fi
    fi
  else
    if [ -f "$tmp/config/probe.json" ]; then
      $SUDO install -m 0600 "$tmp/config/probe.json" "$(config_path_for agent)"
      echo "$(L "已恢复配置：" "Restored config: ")$(config_path_for agent)"
    elif [ -f "$tmp/config/agent.json" ]; then
      $SUDO install -m 0600 "$tmp/config/agent.json" "$(config_path_for agent)"
      echo "$(L "已恢复配置：" "Restored config: ")$(config_path_for agent)"
    elif [ -f "$tmp/etc/tanzhen/probe.json" ]; then
      # Backward compatible: old archive layout (probe name)
      $SUDO install -m 0600 "$tmp/etc/tanzhen/probe.json" "$(config_path_for agent)"
      echo "$(L "已恢复配置：" "Restored config: ")$(config_path_for agent)"
    elif [ -f "$tmp/etc/tanzhen/agent.json" ]; then
      # Backward compatible: old archive layout
      $SUDO install -m 0600 "$tmp/etc/tanzhen/agent.json" "$(config_path_for agent)"
      echo "$(L "已恢复配置：" "Restored config: ")$(config_path_for agent)"
    fi
  fi

  start "$mode" || true
  return 0
}

issue_agent_token_center() {
  local mode="${1:-center}"
  if [ "$mode" != "center" ]; then
    echo "$(L "该功能仅适用于服务器端（中心节点）。" "This action is only for server mode (center).")" >&2
    return 1
  fi
  validate_center_config || return 1

  if ! is_running center; then
    if [ "$(prompt_yesno "$(L "服务器未启动，是否现在启动？" "Server is not running. Start it now?")" "y")" = "true" ]; then
      start center || return 1
    else
      echo "$(L "请先启动服务器后再生成 token。" "Please start the server first, then issue a token.")" >&2
      return 1
    fi
  fi

  if ! command -v curl >/dev/null 2>&1; then
    echo "$(L "缺少 curl，无法调用中心节点 API。请在“更多/修复 → 安装依赖”中安装。" "Missing curl; cannot call center API. Install it via Tools/Repair → Install deps.")" >&2
    return 1
  fi

  local agent_id name
  agent_id="$(prompt "$(L "服务器 ID（唯一，例如 la-01）" "Server ID (unique, e.g. la-01)")" "")"
  agent_id="$(echo "${agent_id:-}" | tr -d '[:space:]')"
  if [ -z "$agent_id" ]; then
    echo "$(L "错误：ID 必填。" "ERROR: ID is required.")" >&2
    return 1
  fi
  name="$(prompt "$(L "显示名称（可选）" "Display name (optional)")" "$agent_id")"
  name="${name:-}"

  local cfg port admin_password
  cfg="$(config_path_for center)"
  port="$(read_center_listen_port || true)"
  if [ -z "$port" ]; then port="38088"; fi
  admin_password="$(json_get_string "$cfg" "admin_password" || true)"
  if [ -z "$admin_password" ]; then
    admin_password="$(json_get_string "$cfg" "admin_token" || true)"
  fi
  if [ -z "$admin_password" ]; then
    echo "$(L "错误：无法从配置读取管理密码。" "ERROR: failed to read admin password from config.")" >&2
    return 1
  fi

  local base body resp tok
  local listen_addr host
  listen_addr="$(json_get_string "$cfg" "listen_addr" || true)"
  host="127.0.0.1"
  if [ -n "$listen_addr" ]; then
    local tmp
    tmp="${listen_addr//\"/}"
    tmp="${tmp// /}"
    host="${tmp%:*}"
    if [ "$host" = "$tmp" ]; then host="127.0.0.1"; fi
    case "$host" in
      ""|":"|"0.0.0.0"|"[::]"|"::") host="127.0.0.1" ;;
    esac
  fi
  base="http://${host}:${port}"
  body="{\"agent_id\":\"$(json_escape "$agent_id")\",\"name\":\"$(json_escape "$name")\"}"
  if ! resp="$(curl -fsS -m 6 -H "X-Admin-Token: ${admin_password}" -H "Content-Type: application/json" -d "$body" "${base}/api/admin/issue_agent_token" 2>/dev/null)"; then
    echo "$(L "错误：生成 token 失败（请检查服务器是否可访问、管理密码是否正确）。" "ERROR: failed to issue token (check server reachability and admin password).")" >&2
    return 1
  fi
  tok="$(echo "$resp" | grep -Eo "\"ingest_token\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -n1 | sed -E 's/.*:[[:space:]]*\"([^\"]*)\".*/\\1/' || true)"
  if [ -z "$tok" ]; then
    echo "$(L "错误：返回解析失败。" "ERROR: failed to parse response.")" >&2
    echo "$resp"
    return 1
  fi

  echo ""
  echo "$(L "已添加/预注册服务器：" "Added/pre-registered server: ")$(C_GREEN "$agent_id")"
  echo "$(L "节点密码（Ingest Token，填到客户端）：" "Node password (Ingest Token, put into client): ")$(C_GREEN "$tok")"
  echo ""
  echo "$(L "客户端只需配置：" "Client only needs:")"
  echo "  central_url: http://<center-ip>:${port}"
  echo "  ingest_token: ${tok}"
  return 0
}

tools_menu_text() {
  local mode="$1"
  if use_dialog; then
    # For dialog UI, it's fine to fall back to text tools menu in the same terminal.
    clear || true
  fi
  while true; do
    local cols sep
    cols="$(term_cols)"
    sep="$(C_DIM "$(repeat_char "-" "$cols")")"
    echo ""
    echo "$(L "修复/工具：" "Tools/Repair:")"
    echo "$sep"
    printf "  %s. %s\n" "$(fmt_menu_num_green 1)" "$(L "自检（端口/连通性/密码）" "Self-check (port/reachability/password)")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 2)" "$(L "安装依赖（dialog/curl/openssl...）" "Install deps (dialog/curl/openssl...)")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 3)" "$(L "防火墙放行端口（服务器）" "Allow firewall port (server)")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 4)" "$(L "防火墙持久化提示（服务器）" "Firewall persistence hints (server)")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 5)" "$(L "导出配置/数据备份" "Export config/data backup")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 6)" "$(L "导入备份并恢复" "Restore from backup")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 7)" "$(L "回滚二进制（恢复上一版）" "Rollback binary (restore previous)")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 8)" "$(L "启用开机自启（无 systemd 用 crontab）" "Enable autostart (crontab if no systemd)")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 9)" "$(L "关闭开机自启（无 systemd）" "Disable autostart (no systemd)")"
    printf " %s. %s\n" "$(fmt_menu_num_green 10)" "$(L "校验配置" "Validate config")"
    printf " %s. %s\n" "$(fmt_menu_num_green 11)" "$(L "实时跟随日志（Ctrl+C 退出）" "Follow logs (Ctrl+C to exit)")"
    local max="11"
    if [ "$mode" = "center" ]; then
      printf " %s. %s\n" "$(fmt_menu_num_green 12)" "$(L "添加服务器（生成节点密码）" "Add server (issue node password)")"
      max="12"
    fi
    printf "  %s. %s\n" "$(fmt_menu_num_green 0)" "$(L "返回" "Back")"
    echo "$sep"

    local n
    read -r -p "$(L "请输入数字 [0-"$max"]：" "Enter number [0-"$max"]: ")" n || true
    n="${n:-}"
    case "$n" in
      0) return 0 ;;
      1) self_check "$mode"; pause_enter ;;
      2) install_deps "$mode"; pause_enter ;;
      3) firewall_menu "$mode"; pause_enter ;;
      4) firewall_persist_hint "$mode"; pause_enter ;;
      5) export_backup "$mode"; pause_enter ;;
      6) restore_backup "$mode"; pause_enter ;;
      7) rollback_binary "$mode"; pause_enter ;;
      8) enable_autostart_nosystemd "$mode"; pause_enter ;;
      9) disable_autostart_nosystemd "$mode"; pause_enter ;;
      10)
        if [ "$mode" = "center" ]; then validate_center_config; else validate_agent_config; fi
        pause_enter
        ;;
      11) follow_logs "$mode"; pause_enter ;;
      12) issue_agent_token_center "$mode"; pause_enter ;;
      *) echo "$(L "无效选择。" "Invalid selection.")"; pause_enter ;;
    esac
  done
}

start() {
  local mode="$1"
  if [ "$mode" = "center" ]; then validate_center_config || return 1; else validate_agent_config || return 1; fi
  if have_systemd; then
    write_systemd_unit "$mode"
    local SUDO; SUDO="$(sudo_prefix)"
    local svc; svc="$(service_name_for "$mode")"
    if ! $SUDO systemctl restart "$svc"; then
      echo "$(L "错误：启动失败（systemd）。" "ERROR: failed to start (systemd).")" >&2
      $SUDO systemctl status "$svc" --no-pager || true
      return 1
    fi
    echo "$(L "已启动（systemd）。" "Started via systemd.")"
  else
    nohup_start "$mode"
  fi
}

stop() {
  local mode="$1"
  if have_systemd; then
    local svc; svc="$(service_name_for "$mode")"
    local unit; unit="$(unit_path_for "$svc")"
    if [ -f "$unit" ]; then
      local SUDO; SUDO="$(sudo_prefix)"
      $SUDO systemctl stop "$svc" || true
      echo "$(L "已停止（systemd）。" "Stopped via systemd.")"
      return 0
    fi
  fi
  nohup_stop "$mode"
}

restart() {
  local mode="$1"
  if have_systemd; then
    local svc; svc="$(service_name_for "$mode")"
    local unit; unit="$(unit_path_for "$svc")"
    if [ -f "$unit" ]; then
      ensure_binary_installed "$mode"
      local SUDO; SUDO="$(sudo_prefix)"
      if ! $SUDO systemctl restart "$svc"; then
        echo "$(L "错误：重启失败（systemd）。" "ERROR: failed to restart (systemd).")" >&2
        $SUDO systemctl status "$svc" --no-pager || true
        return 1
      fi
      echo "$(L "已重启（systemd）。" "Restarted via systemd.")"
      return 0
    fi
  fi
  nohup_stop "$mode"
  nohup_start "$mode"
}

show_config() {
  local mode="$1"
  local cfg; cfg="$(config_path_for "$mode")"
  if [ ! -f "$cfg" ]; then
    echo "$(L "找不到配置：" "Config not found: ")$cfg"
    exit 1
  fi
  cat "$cfg"
}

uninstall() {
  local mode="$1"
  local SUDO; SUDO="$(sudo_prefix)"
  local svc; svc="$(service_name_for "$mode")"
  local centerDataDir=""
  if [ "$mode" = "center" ]; then
    centerDataDir="$(read_center_data_dir || true)"
  fi
  stop "$mode" || true
  disable_autostart_nosystemd "$mode" >/dev/null 2>&1 || true

  if have_systemd; then
    local unit; unit="$(unit_path_for "$svc")"
    if [ -f "$unit" ]; then
      $SUDO systemctl disable "$svc" >/dev/null 2>&1 || true
      $SUDO rm -f "$unit" || true
      $SUDO systemctl daemon-reload || true
    fi
  fi

  $SUDO rm -f "$BIN_DIR/$(binary_name_for "$mode")" || true
  $SUDO rm -f "$(config_path_for "$mode")" || true
  $SUDO rm -f "$(pid_path_for "$svc")" || true

  local purge="${TZ_UNINSTALL_PURGE:-0}"
  local purgeData="false"
  local purgeLogs="false"
  local purgeBackups="false"

  if [ "$purge" = "1" ]; then
    purgeData="true"
    purgeLogs="true"
    purgeBackups="true"
  fi

  if [ "$mode" = "center" ] && [ "$purge" != "1" ]; then
    purgeData="$(prompt_yesno "$(L "是否删除服务器数据目录（历史/配置）？" "Delete server data dir (history/config)?")" "n")"
    purgeLogs="$(prompt_yesno "$(L "是否删除日志文件？" "Delete log file?")" "n")"
    purgeBackups="$(prompt_yesno "$(L "是否删除本脚本备份目录？" "Delete script backup dir?")" "n")"
  elif [ "$mode" = "agent" ] && [ "$purge" != "1" ]; then
    purgeLogs="$(prompt_yesno "$(L "是否删除日志文件？" "Delete log file?")" "n")"
    purgeBackups="$(prompt_yesno "$(L "是否删除本脚本备份目录？" "Delete script backup dir?")" "n")"
  fi

  if [ "$mode" = "center" ] && [ "$purgeData" = "true" ]; then
    if [ -n "$centerDataDir" ]; then
      $SUDO rm -rf "$centerDataDir" || true
      echo "$(L "已删除数据目录：" "Deleted data dir: ")$centerDataDir"
    fi
  fi
  if [ "$purgeLogs" = "true" ]; then
    local logf; logf="$(log_path_for "$svc")"
    $SUDO rm -f "$logf" || true
    echo "$(L "已删除日志：" "Deleted log: ")$logf"
  fi
  if [ "$purgeBackups" = "true" ]; then
    local bdir; bdir="$(backup_dir_for "$svc")"
    $SUDO rm -rf "$bdir" || true
    echo "$(L "已删除备份目录：" "Deleted backup dir: ")$bdir"
  fi

  echo "$(L "已卸载 " "Uninstalled ")$(mode_label "$mode")$(L "。" ".")"
}

banner() {
  local mode="${1:-agent}"
  local cols; cols="$(term_cols)"
  local border; border="$(repeat_char "=" "$cols")"

  local statusShort
  if is_running "$mode"; then
    statusShort="$(C_GREEN "$(L "已启动" "running")")"
  else
    statusShort="$(C_RED "$(L "未启动" "stopped")")"
  fi

  echo "$border"
  echo "  ${SCRIPT_NAME} $(L "一键安装管理脚本" "one-click install & manage script") [$(C_RED "${SCRIPT_VERSION}")]"
  echo "  $(L "当前菜单:" "Menu:") $(mode_label "$mode")$(L "菜单" " menu") | $(mode_label "$mode") ${statusShort}"
  echo "$border"
}

mode_label() {
  local mode="$1"
  if [ "$mode" = "agent" ]; then
    echo "$(L "客户端" "Client")"
  else
    echo "$(L "服务器" "Server")"
  fi
}

menu_loop() {
  local mode="$1"
  while true; do
    ensure_config_for_mode "$mode"
    banner "$mode"

    local cols sep
    cols="$(term_cols)"
    sep="$(C_DIM "$(repeat_char "-" "$cols")")"

    printf "  %s. %s\n" "$(fmt_menu_num_green 0)" "$(L "升级脚本" "Upgrade script")"
    echo "$sep"
    printf "  %s. %s%s\n" "$(fmt_menu_num_green 1)" "$(L "安装 " "Install ")" "$(mode_label "$mode")"
    printf "  %s. %s%s\n" "$(fmt_menu_num_green 2)" "$(L "更新 " "Update ")" "$(mode_label "$mode")"
    printf "  %s. %s%s\n" "$(fmt_menu_num_green 3)" "$(L "卸载 " "Uninstall ")" "$(mode_label "$mode")"
    echo "$sep"
    printf "  %s. %s%s\n" "$(fmt_menu_num_green 4)" "$(L "启动 " "Start ")" "$(mode_label "$mode")"
    printf "  %s. %s%s\n" "$(fmt_menu_num_green 5)" "$(L "停止 " "Stop ")" "$(mode_label "$mode")"
    printf "  %s. %s%s\n" "$(fmt_menu_num_green 6)" "$(L "重启 " "Restart ")" "$(mode_label "$mode")"
    echo "$sep"
    printf "  %s. %s%s%s\n" "$(fmt_menu_num_green 7)" "$(L "设置 " "Configure ")" "$(mode_label "$mode")" "$(L "配置" " config")"
    printf "  %s. %s%s%s\n" "$(fmt_menu_num_green 8)" "$(L "查看 " "Show ")" "$(mode_label "$mode")" "$(L "信息" " info")"
    printf "  %s. %s%s%s\n" "$(fmt_menu_num_green 9)" "$(L "查看 " "View ")" "$(mode_label "$mode")" "$(L "日志" " logs")"
    printf "  %s. %s\n" "$(fmt_menu_num_green 10)" "$(L "更多/修复" "Tools/Repair")"
    echo "$sep"
    if [ "$mode" = "agent" ]; then
      printf "  %s. %s\n" "$(fmt_menu_num_green 11)" "$(L "切换为 服务器菜单" "Switch to Server menu")"
    else
      printf "  %s. %s\n" "$(fmt_menu_num_green 11)" "$(L "切换为 客户端菜单" "Switch to Client menu")"
    fi

    print_state_line "$mode"

    local n
    if [ "$TZ_LANG" = "en" ]; then
      read -r -p "Enter number [0-11]: " n || true
    else
      read -r -p "请输入数字 [0-11]：" n || true
    fi
    n="${n:-}"

    case "$n" in
      0) upgrade_script || true; pause_enter ;;
      1) install_local "$mode"; pause_enter ;;
      2) update_binary "$mode" || true; pause_enter ;;
      3) uninstall "$mode"; pause_enter ;;
      4) start "$mode"; pause_enter ;;
      5) stop "$mode"; pause_enter ;;
      6) restart "$mode"; pause_enter ;;
      7)
        if [ "$mode" = "center" ]; then write_center_config; else write_agent_config; fi
        pause_enter
        ;;
      8) show_info "$mode" ;;
      9) show_logs "$mode" ;;
      10) tools_menu_text "$mode" ;;
      11)
        if [ "$mode" = "agent" ]; then mode="center"; else mode="agent"; fi
        ;;
      *) echo "$(L "无效选择。" "Invalid selection.")"; pause_enter ;;
    esac
  done
}

menu_loop_dialog() {
  local mode="$1"
  while true; do
    ensure_config_for_mode "$mode"
    local state
    if is_installed "$mode"; then
      if is_running "$mode"; then
        state="$(L "当前状态：$(mode_label "$mode") 已安装 并 已启动" "Current status: $(mode_label "$mode") is installed and running")"
      else
        state="$(L "当前状态：$(mode_label "$mode") 已安装 但 未启用" "Current status: $(mode_label "$mode") is installed but stopped")"
      fi
    else
      state="$(L "当前状态：$(mode_label "$mode") 未安装" "Current status: $(mode_label "$mode") is not installed")"
    fi

    local title="${SCRIPT_NAME} ${SCRIPT_VERSION} - $(mode_label "$mode")$(L "菜单" " menu")"
    local choice
    local switchText
    if [ "$mode" = "agent" ]; then
      switchText="$(L "切换为 服务器菜单" "Switch to Server menu")"
    else
      switchText="$(L "切换为 客户端菜单" "Switch to Client menu")"
    fi
    local toolsText
    toolsText="$(L "更多/修复" "Tools/Repair")"
    choice="$(
      dialog --clear --stdout --title "$title" \
        --menu "$state\n\n$(L "请选择操作：" "Select an action:")" 22 90 12 \
        0 "$(L "升级脚本（占位）" "Upgrade script (placeholder)")" \
        1 "$(L "安装" "Install") $(mode_label "$mode")" \
        2 "$(L "更新（占位下载）" "Update (download placeholder)") $(mode_label "$mode")" \
        3 "$(L "卸载" "Uninstall") $(mode_label "$mode")" \
        4 "$(L "启动" "Start") $(mode_label "$mode")" \
        5 "$(L "停止" "Stop") $(mode_label "$mode")" \
        6 "$(L "重启" "Restart") $(mode_label "$mode")" \
        7 "$(L "设置配置" "Configure") $(mode_label "$mode")" \
        8 "$(L "查看信息" "Show info") $(mode_label "$mode")" \
        9 "$(L "查看日志" "View logs") $(mode_label "$mode")" \
        10 "$toolsText" \
        11 "$switchText"
    )" || {
      clear || true
      exit 0
    }

    case "$choice" in
      0) dialog_action upgrade_script || true ;;
      1) dialog_action install_local "$mode" || true ;;
      2) dialog_action update_binary "$mode" || true ;;
      3) dialog_action uninstall "$mode" || true ;;
      4) dialog_action start "$mode" || true ;;
      5) dialog_action stop "$mode" || true ;;
      6) dialog_action restart "$mode" || true ;;
      7)
        if [ "$mode" = "center" ]; then dialog_action write_center_config || true; else dialog_action write_agent_config || true; fi
        ;;
      8) show_info "$mode" ;;
      9) show_logs "$mode" ;;
      10) dialog_action tools_menu_text "$mode" || true ;;
      11)
        if [ "$mode" = "agent" ]; then mode="center"; else mode="agent"; fi
        ;;
      *) ;;
    esac
  done
}

run_action() {
  local mode="$1"
  local act="$2"
  case "$act" in
    install) install_local "$mode" ;;
    download) download_binary "$mode" ;;
    configure)
      if [ "$mode" = "center" ]; then write_center_config; else write_agent_config; fi
      ;;
    start) start "$mode" ;;
    stop) stop "$mode" ;;
    restart) restart "$mode" ;;
    status) status "$mode" ;;
    show-config) show_config "$mode" ;;
    uninstall) uninstall "$mode" ;;
    *) usage; exit 1 ;;
  esac
}

main() {
  if [ "${1-}" = "-h" ] || [ "${1-}" = "--help" ]; then
    usage
    exit 0
  fi

  if [ $# -eq 0 ]; then
    if [ "$TZ_UI" = "dialog" ] && ! use_dialog; then
      echo "$(L "错误：已指定 TZ_UI=dialog，但当前环境无法使用 dialog（未安装/非 TTY/TERM=dumb）。" "ERROR: TZ_UI=dialog is set but dialog UI is not available (missing dialog / not a TTY / TERM=dumb).")" >&2
      exit 1
    fi
    # First-time experience: if no configs exist, guide the user through init.
    local cc ca
    cc="$(config_path_for center)"
    ca="$(config_path_for agent)"
    if [ ! -f "$cc" ] && [ ! -f "$ca" ]; then
      first_run_wizard
    fi
    local defm
    defm="$(default_mode_for_noargs)"
    ensure_config_or_init_then_menu "$defm"
  fi

  local mode="${1-}"
  local act="${2-}"

  # Convenience: allow "c"/"s" to open the numbered menu.
  if [ $# -eq 1 ]; then
    case "$mode" in
      c|client|agent|probe)
        if [ "$TZ_UI" = "dialog" ] && ! use_dialog; then
          echo "$(L "错误：已指定 TZ_UI=dialog，但当前环境无法使用 dialog（未安装/非 TTY/TERM=dumb）。" "ERROR: TZ_UI=dialog is set but dialog UI is not available (missing dialog / not a TTY / TERM=dumb).")" >&2
          exit 1
        fi
        ensure_config_for_mode "agent"
        if use_dialog; then menu_loop_dialog "agent"; else menu_loop "agent"; fi
        exit 0
        ;;
      s|server|center)
        if [ "$TZ_UI" = "dialog" ] && ! use_dialog; then
          echo "$(L "错误：已指定 TZ_UI=dialog，但当前环境无法使用 dialog（未安装/非 TTY/TERM=dumb）。" "ERROR: TZ_UI=dialog is set but dialog UI is not available (missing dialog / not a TTY / TERM=dumb).")" >&2
          exit 1
        fi
        ensure_config_for_mode "center"
        if use_dialog; then menu_loop_dialog "center"; else menu_loop "center"; fi
        exit 0
        ;;
    esac
  fi

  case "$mode" in
    client|c|agent|probe|客户端|探针) mode="agent" ;;
    server|s|center|服务端|服务器) mode="center" ;;
  esac

  if [ -z "$mode" ] || [ -z "$act" ]; then
    usage
    exit 1
  fi
  if [ "$mode" != "center" ] && [ "$mode" != "agent" ]; then
    usage
    exit 1
  fi
  run_action "$mode" "$act"
}

main "$@"
