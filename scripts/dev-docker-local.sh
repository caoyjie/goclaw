#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILES=(
  -f docker-compose.yml
  -f docker-compose.postgres.yml
  -f docker-compose.browser.yml
  -f docker-compose.otel.yml
  -f docker-compose.redis.yml
  -f docker-compose.sandbox-ceo.yml
)

UPGRADE_FILES=(
  -f docker-compose.yml
  -f docker-compose.postgres.yml
  -f docker-compose.upgrade.yml
)

compose() {
  docker compose "${COMPOSE_FILES[@]}" "$@"
}

upgrade_compose() {
  docker compose "${UPGRADE_FILES[@]}" "$@"
}

set_version_file() {
  local version
  version="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || echo dev)"
  echo "$version" > VERSION
  echo "VERSION=$version"
}

health_check() {
  curl -fsS http://localhost:18790/health >/dev/null
  echo "Health check OK: http://localhost:18790/health"
}

prepare_env() {
  chmod +x prepare-env.sh
  ./prepare-env.sh
}

build() {
  set_version_file
  GOCLAW_VERSION="$(cat VERSION)" compose build --pull
}

up() {
  set_version_file
  GOCLAW_VERSION="$(cat VERSION)" compose up -d
}

migrate() {
  upgrade_compose run --rm upgrade
}

start() {
  build
  up
  migrate
  health_check
  status
  endpoints
}

deploy() {
  set_version_file
  GOCLAW_VERSION="$(cat VERSION)" compose build goclaw
  GOCLAW_VERSION="$(cat VERSION)" compose up -d --no-deps goclaw
  # Ensure ComfyUI is running for embedded media studio page.
  GOCLAW_VERSION="$(cat VERSION)" compose up -d comfyui
  migrate
  health_check
  status
}

quick() {
  local build_args=()
  if [ "${1:-}" = "--no-ui" ]; then
    build_args+=(--build-arg ENABLE_EMBEDUI=false)
  fi
  set_version_file
  GOCLAW_VERSION="$(cat VERSION)" compose build --no-cache=false "${build_args[@]}" goclaw
  GOCLAW_VERSION="$(cat VERSION)" compose up -d --no-deps goclaw
  migrate
  health_check
  status
}

rollback() {
  local target_ref="${1:-}"
  if [ -z "$target_ref" ]; then
    echo "用法: ./scripts/dev-docker-local.sh rollback <git-ref>"
    return 1
  fi

  git rev-parse --verify "$target_ref" >/dev/null
  git checkout "$target_ref"
  deploy
}

status() {
  compose ps
}

logs() {
  compose logs -f goclaw
}

down() {
  compose down
}

reset() {
  set_version_file
  compose down -v
  GOCLAW_VERSION="$(cat VERSION)" compose up -d --pull always
  migrate
  health_check
}

endpoints() {
  cat <<EOM
GoClaw WebUI:   http://localhost:18790
Health Check:   http://localhost:18790/health
Jaeger (OTEL):  http://localhost:16686
EOM
}

usage() {
  cat <<'EOM'
Usage:
  ./scripts/dev-docker-local.sh <command>


EOM
}

cmd="${1:-}"
case "$cmd" in
  prepare-env)
    prepare_env
    ;;
  build)
    build
    ;;
  up)
    up
    ;;
  migrate)
    migrate
    ;;
  start)
    start
    ;;
  deploy)
    deploy
    ;;
  quick)
    quick "${2:-}"
    ;;
  rollback)
    rollback "${2:-}"
    ;;
  status)
    status
    ;;
  logs)
    logs
    ;;
  down)
    down
    ;;
  reset)
    reset
    ;;
  endpoints)
    endpoints
    ;;
  *)
    usage
    exit 1
    ;;
esac
