#!/bin/bash
set -e

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$APP_DIR/cc-switch"
DATA_DIR="$APP_DIR/.ccswitch"
PIDFILE="$DATA_DIR/server.pid"
LOGFILE="$DATA_DIR/server.log"
ENVFILE="$APP_DIR/.env"
DEFAULT_PUBLIC_ADDR=":18080"
DEFAULT_VSCODE_ADDR="127.0.0.1:18080"

cd "$APP_DIR"

load_env() {
  if [ -f "$ENVFILE" ]; then
    set -a
    source "$ENVFILE"
    set +a
  fi
  : "${CCSWITCH_ADDR:=$DEFAULT_PUBLIC_ADDR}"
  export CCSWITCH_ADDR
}

write_env_addr() {
  ADDR="$1"
  printf 'CCSWITCH_ADDR=%s\n' "$ADDR" > "$ENVFILE"
  CCSWITCH_ADDR="$ADDR"
  export CCSWITCH_ADDR
}

is_public_listen() {
  [ "${CCSWITCH_ADDR#:}" != "$CCSWITCH_ADDR" ] || [ "${CCSWITCH_ADDR#0.0.0.0:}" != "$CCSWITCH_ADDR" ]
}

local_check_addr() {
  ADDR="${CCSWITCH_ADDR:-:18080}"
  if [ "${ADDR#:}" != "$ADDR" ]; then
    echo "127.0.0.1:${ADDR#:}"
  elif [ "${ADDR#0.0.0.0:}" != "$ADDR" ]; then
    echo "127.0.0.1:${ADDR#0.0.0.0:}"
  else
    echo "$ADDR"
  fi
}

listen_port() {
  ADDR="${CCSWITCH_ADDR:-:18080}"
  if [ "${ADDR#:}" != "$ADDR" ]; then
    echo "${ADDR#:}"
  else
    echo "${ADDR##*:}"
  fi
}

primary_host_ip() {
  set -- $(hostname -I 2>/dev/null || true)
  echo "${1:-SERVER_PUBLIC_IP}"
}

show_access_hint() {
  CHECK_ADDR="$(local_check_addr)"
  PORT="$(listen_port)"
  echo "Local check: http://$CHECK_ADDR/ccswitch/"
  if is_public_listen; then
    echo "Public listen enabled: http://$(primary_host_ip):$PORT/ccswitch/"
    echo "WARNING: No authentication is enabled. Restrict access with firewall/security group if needed."
  else
    echo "VSCode forward remote port $PORT to local port $PORT."
    echo "Local browser URL: http://127.0.0.1:$PORT/ccswitch/"
  fi
}

stop_existing() {
  STOPPED=""
  if [ -f "$PIDFILE" ]; then
    PID=$(cat "$PIDFILE")
    if kill -0 "$PID" 2>/dev/null; then
      PROC_CWD=$(readlink "/proc/$PID/cwd" 2>/dev/null || true)
      if [ "$PROC_CWD" = "$APP_DIR" ]; then
        echo "Stopping existing process $PID ..."
        kill "$PID" || true
        STOPPED="$STOPPED $PID"
        sleep 1
      else
        echo "Stale pidfile points to another process $PID; removing pidfile."
      fi
    fi
    rm -f "$PIDFILE"
  fi

  # PID files can be stale after a failed start or manual process handling.
  # Also stop cc-switch processes that are running from this project directory.
  for PROC in /proc/[0-9]*; do
    [ -d "$PROC" ] || continue
    PID=${PROC##*/}
    case " $STOPPED " in
      *" $PID "*) continue ;;
    esac
    if ! kill -0 "$PID" 2>/dev/null; then
      continue
    fi
    PROC_CWD=$(readlink "$PROC/cwd" 2>/dev/null || true)
    if [ "$PROC_CWD" != "$APP_DIR" ]; then
      continue
    fi
    COMM=$(cat "$PROC/comm" 2>/dev/null || true)
    CMDLINE=$(tr '\0' ' ' < "$PROC/cmdline" 2>/dev/null || true)
    if [ "$COMM" = "cc-switch" ] || echo "$CMDLINE" | grep -q "$BINARY"; then
      echo "Stopping existing process $PID ..."
      kill "$PID" || true
      STOPPED="$STOPPED $PID"
    fi
  done

  if [ -n "$STOPPED" ]; then
    sleep 1
  fi
}

start_server() {
  load_env
  mkdir -p "$DATA_DIR"
  if [ -f "$BINARY" ]; then
    setsid "$BINARY" >> "$LOGFILE" 2>&1 < /dev/null &
  else
    echo "Binary not found, fallback to 'go run .'"
    setsid go run . >> "$LOGFILE" 2>&1 < /dev/null &
  fi
  echo $! > "$PIDFILE"
  sleep 1
  if kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "cc-switch started. PID: $(cat "$PIDFILE")"
    show_access_hint
  else
    echo "Failed to start cc-switch. Check $LOGFILE"
    exit 1
  fi
}

status_server() {
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "Running. PID: $(cat "$PIDFILE")"
  else
    echo "Not running."
    return 1
  fi

  load_env
  CHECK_ADDR="$(local_check_addr)"
  if command -v curl >/dev/null 2>&1; then
    if curl -fsS --max-time 2 "http://$CHECK_ADDR/ccswitch/" >/dev/null; then
      echo "HTTP OK: http://$CHECK_ADDR/ccswitch/"
    else
      echo "HTTP check failed: http://$CHECK_ADDR/ccswitch/"
    fi
  fi
  if is_public_listen; then
    echo "Public listen enabled on port $(listen_port)."
  else
    echo "VSCode/local mode enabled. Forward port $(listen_port), then open http://127.0.0.1:$(listen_port)/ccswitch/"
  fi
}

case "${1:-start}" in
  start)
    stop_existing
    start_server
    ;;
  stop)
    stop_existing
    echo "Stopped."
    ;;
  restart)
    stop_existing
    start_server
    ;;
  vscode|local)
    write_env_addr "$DEFAULT_VSCODE_ADDR"
    echo "Configured .env for VSCode/local forwarding: CCSWITCH_ADDR=$DEFAULT_VSCODE_ADDR"
    stop_existing
    start_server
    ;;
  public)
    write_env_addr "$DEFAULT_PUBLIC_ADDR"
    echo "Configured .env for public listen: CCSWITCH_ADDR=$DEFAULT_PUBLIC_ADDR"
    stop_existing
    start_server
    ;;
  status)
    status_server
    ;;
  import-current)
    if [ -z "$2" ] || [ -z "$3" ]; then
      echo "Usage: $0 import-current <claude|codex|all> <name>"
      exit 1
    fi
    if [ -f "$BINARY" ]; then
      "$BINARY" import-current "$2" "$3"
    else
      go run . import-current "$2" "$3"
    fi
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|status|vscode|local|public|import-current}"
    exit 1
    ;;
esac
