#!/bin/bash
set -e

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$APP_DIR/cc-switch"
DATA_DIR="$APP_DIR/.ccswitch"
PIDFILE="$DATA_DIR/server.pid"
LOGFILE="$DATA_DIR/server.log"
ENVFILE="$APP_DIR/.env"

cd "$APP_DIR"

load_env() {
  if [ -f "$ENVFILE" ]; then
    set -a
    source "$ENVFILE"
    set +a
  fi
}

stop_existing() {
  if [ -f "$PIDFILE" ]; then
    PID=$(cat "$PIDFILE")
    if kill -0 "$PID" 2>/dev/null; then
      echo "Stopping existing process $PID ..."
      kill "$PID" || true
      sleep 1
    fi
    rm -f "$PIDFILE"
  fi
  # 也查找同名进程
  pkill -f "cc-switch" 2>/dev/null || true
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
    echo "Visit http://localhost:18080/ccswitch/"
  else
    echo "Failed to start cc-switch. Check $LOGFILE"
    exit 1
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
  status)
    if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
      echo "Running. PID: $(cat "$PIDFILE")"
    else
      echo "Not running."
    fi
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
    echo "Usage: $0 {start|stop|restart|status|import-current}"
    exit 1
    ;;
esac
