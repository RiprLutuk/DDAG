#!/usr/bin/env bash
# Start/stop the DDAG core services locally for development. Each service runs
# as its own process (mirroring one-pod-per-service in production). Metadata
# PostgreSQL (:1921) and Redis (:6379) are expected to be running already.
# Bash 3.2 compatible (macOS default shell) — no associative arrays.
set -eu

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin"
RUN="$ROOT/.run"
LOGS="$RUN/logs"
mkdir -p "$RUN" "$LOGS"

# Core services to run locally (service:port pairs).
SERVICES="admin-backend:8080 auth-service:8081 api-gateway:8082 connector-postgres:8090"

start() {
  for pair in $SERVICES; do
    s="${pair%%:*}"; port="${pair##*:}"
    lsof -ti tcp:"$port" | xargs kill -9 2>/dev/null || true
    if [ ! -x "$BIN/ddag-$s" ]; then
      echo "missing $BIN/ddag-$s — run 'make build' first"; exit 1
    fi
    nohup "$BIN/ddag-$s" >"$LOGS/$s.log" 2>&1 &
    echo $! > "$RUN/$s.pid"
    echo "started $s (pid $(cat "$RUN/$s.pid"), :$port)"
  done
  echo
  echo "DDAG core services started. Logs: $LOGS"
  echo "  admin-backend  http://localhost:8080  (dashboard API)"
  echo "  auth-service   http://localhost:8081  (OAuth2)"
  echo "  api-gateway    http://localhost:8082  (dynamic APIs)"
  echo "  connector-pg   http://localhost:8090"
}

stop() {
  for pair in $SERVICES; do
    s="${pair%%:*}"; port="${pair##*:}"
    if [ -f "$RUN/$s.pid" ]; then
      kill -9 "$(cat "$RUN/$s.pid")" 2>/dev/null || true
      rm -f "$RUN/$s.pid"
    fi
    lsof -ti tcp:"$port" | xargs kill -9 2>/dev/null || true
    echo "stopped $s"
  done
}

logs() { tail -n 50 -f "$LOGS"/*.log; }

case "${1:-start}" in
  start) start ;;
  stop)  stop ;;
  logs)  logs ;;
  restart) stop; start ;;
  *) echo "usage: dev.sh {start|stop|restart|logs}"; exit 1 ;;
esac
