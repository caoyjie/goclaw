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

SUPPORT_SERVICES=(
  postgres
  chrome
  jaeger
  redis
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

ensure_support_services() {
  GOCLAW_VERSION="$(cat VERSION)" compose up -d "${SUPPORT_SERVICES[@]}"
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
  local mode="${1:-}"
  if [ -z "$mode" ]; then
    if [ -t 0 ]; then
      cat <<'EOM'
请选择部署模式:
  1) backend  只重建后端镜像，跳过 embedded UI（更快）
  2) frontend 重建完整镜像，包含 embedded UI（推荐）
  3) all      构建所有服务镜像并重新拉起完整部署
EOM
      read -r -p "请输入 [2]: " mode
      case "${mode:-2}" in
        1|backend|--backend|后端) mode="backend" ;;
        2|frontend|--frontend|前端|"") mode="frontend" ;;
        3|all|--all|全部) mode="all" ;;
        *) echo "无效选择: $mode"; return 1 ;;
      esac
    else
      mode="frontend"
    fi
  fi

  set_version_file
  ensure_support_services

  case "$mode" in
    backend)
      echo "Deploy mode: backend (embedded UI disabled)"
      GOCLAW_VERSION="$(cat VERSION)" compose build --build-arg ENABLE_EMBEDUI=false goclaw
      ;;
    frontend)
      echo "Deploy mode: frontend (embedded UI enabled)"
      GOCLAW_VERSION="$(cat VERSION)" compose build goclaw
      ;;
    all)
      echo "Deploy mode: all (full compose build + up)"
      GOCLAW_VERSION="$(cat VERSION)" compose build
      GOCLAW_VERSION="$(cat VERSION)" compose up -d
      migrate
      health_check
      status
      return
      ;;
    *)
      echo "用法: ./scripts/dev-docker-local.sh deploy [backend|frontend|all]"
      return 1
      ;;
  esac

  GOCLAW_VERSION="$(cat VERSION)" compose up -d --force-recreate --no-deps goclaw
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
  ./scripts/dev-docker-local.sh deploy [backend|frontend|all]


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
    deploy "${2:-}"
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
