#!/usr/bin/env bash
# Start an isolated DDAG demo without requiring a host PostgreSQL installation.
# This script only operates on the "ddag-demo" Compose project.
set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly ENV_FILE="$ROOT/.env.demo"
readonly COMPOSE=(docker compose --project-name ddag-demo --env-file "$ENV_FILE" -f "$ROOT/docker-compose.yml" -f "$ROOT/docker-compose.demo.yml" --profile demo)

usage() {
  cat <<'EOF'
Usage: ./scripts/quickstart-demo.sh [--start|--status|--stop]

  --start   Generate .env.demo if needed, build/start the local demo, then seed it.
            This is the default.
  --status  Show the status of the isolated demo Compose project.
  --stop    Stop demo containers without deleting the demo database volume.

The demo binds DDAG's dashboard and service ports to localhost as defined in
 docker-compose.yml. Its PostgreSQL database has no host port and credentials
are generated locally in .env.demo (mode 600). Do not use this setup for
production.
EOF
}

require_docker() {
  command -v docker >/dev/null 2>&1 || {
    echo "Error: Docker CLI is required. Install Docker Compose v2 and retry." >&2
    exit 1
  }
  docker compose version >/dev/null 2>&1 || {
    echo "Error: Docker Compose v2 is required (docker compose ...)." >&2
    exit 1
  }
}

random_b64() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -d '\n'
  else
    python3 -c 'import base64, secrets; print(base64.b64encode(secrets.token_bytes(32)).decode())'
  fi
}

random_hex() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    python3 -c 'import secrets; print(secrets.token_hex(32))'
  fi
}

create_env_file() {
  if [[ -e "$ENV_FILE" ]]; then
    return
  fi

  umask 077
  {
    printf '%s\n' '# Generated for the disposable local demo. Do not commit this file.'
    printf 'DDAG_ENV=demo\n'
    printf 'DDAG_LOG_LEVEL=info\n'
    printf 'DDAG_DB_USER=ddag\n'
    printf 'DDAG_DB_PASSWORD=%s\n' "$(random_hex)"
    printf 'DDAG_DB_NAME=ddag\n'
    printf 'DDAG_DB_SSLMODE=disable\n'
    printf 'DDAG_MASTER_KEY=%s\n' "$(random_b64)"
    printf 'DDAG_SESSION_SECRET=%s\n' "$(random_hex)"
    printf 'DDAG_SESSION_COOKIE_SECURE=false\n'
    printf 'DDAG_DASHBOARD_ORIGINS=http://localhost:3000\n'
    printf 'DDAG_SUPERADMIN_PASSWORD=%s\n' "$(random_hex)"
    printf 'DDAG_TOKEN_ISSUER=http://localhost:8081\n'
    printf 'DDAG_TOKEN_AUDIENCE=ddag-api\n'
    printf 'DDAG_RATE_LIMIT_FAIL_MODE=closed\n'
    printf 'GRAFANA_PASSWORD=%s\n' "$(random_hex)"
  } > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  echo "Created $ENV_FILE with generated local-only secrets (mode 600)."
}

start() {
  create_env_file
  "${COMPOSE[@]}" up --build --detach
  "${COMPOSE[@]}" run --rm migrate --demo
  cat <<'EOF'

DDAG demo is ready:
  Dashboard:  http://localhost:3000
  Gateway:    http://localhost:8082
  Prometheus: http://localhost:9090
  Grafana:    http://localhost:3001

Demo sign-in (local only): superadmin
The generated password is in .env.demo as DDAG_SUPERADMIN_PASSWORD.
Stop containers (preserves demo data): ./scripts/quickstart-demo.sh --stop
EOF
}

main() {
  case "${1:---start}" in
    --start) require_docker; start ;;
    --status) require_docker; [[ -f "$ENV_FILE" ]] || { echo "Demo has not been initialized."; exit 0; }; "${COMPOSE[@]}" ps ;;
    --stop) require_docker; [[ -f "$ENV_FILE" ]] || { echo "Demo has not been initialized."; exit 0; }; "${COMPOSE[@]}" stop ;;
    -h|--help) usage ;;
    *) usage >&2; exit 2 ;;
  esac
}

main "$@"
