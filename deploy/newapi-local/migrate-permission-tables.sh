#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
MIGRATION_FILE="${SCRIPT_DIR}/migrations/20260731_create_user_management_permissions.postgres.sql"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-new-api-postgres}"
APP_CONTAINER="${APP_CONTAINER:-new-api}"
BACKUP_DIR="${BACKUP_DIR:-/root/new-api-db-backups}"

usage() {
  cat <<'EOF'
Usage:
  deploy/newapi-local/migrate-permission-tables.sh check
  deploy/newapi-local/migrate-permission-tables.sh apply

Environment overrides:
  POSTGRES_CONTAINER  PostgreSQL container name (default: new-api-postgres)
  APP_CONTAINER       application container name (default: new-api)
  BACKUP_DIR          host backup directory (default: /root/new-api-db-backups)

The apply command:
  1. verifies the PostgreSQL container and database identity;
  2. creates and validates a pg_dump backup;
  3. stops the application container;
  4. applies the idempotent permission-table migration;
  5. restarts the application and verifies all permission tables.
EOF
}

require_container() {
  local container=$1
  if ! docker inspect "${container}" >/dev/null 2>&1; then
    echo "ERROR: container not found: ${container}" >&2
    exit 1
  fi
  if [[ "$(docker inspect -f '{{.State.Running}}' "${container}")" != "true" ]]; then
    echo "ERROR: container is not running: ${container}" >&2
    exit 1
  fi
}

postgres_setting() {
  local variable=$1
  docker exec "${POSTGRES_CONTAINER}" sh -ec \
    'value=$(printenv "$1"); if [ -z "$value" ]; then exit 1; fi; printf "%s" "$value"' \
    sh "${variable}"
}

query_permission_tables() {
  docker exec "${POSTGRES_CONTAINER}" \
    psql -X -v ON_ERROR_STOP=1 \
    -U "${POSTGRES_USER}" \
    -d "${POSTGRES_DB}" \
    -Atc "
      SELECT expected.table_name || '=' ||
             CASE WHEN to_regclass('public.' || expected.table_name) IS NULL
                  THEN 'missing' ELSE 'present' END
      FROM (VALUES
        ('casbin_rule'),
        ('authz_roles'),
        ('user_management_permissions')
      ) AS expected(table_name)
      ORDER BY expected.table_name;
    "
}

check() {
  require_container "${POSTGRES_CONTAINER}"
  POSTGRES_USER=$(postgres_setting POSTGRES_USER)
  POSTGRES_DB=$(postgres_setting POSTGRES_DB)
  export POSTGRES_USER POSTGRES_DB

  echo "PostgreSQL container: ${POSTGRES_CONTAINER}"
  echo "Database: ${POSTGRES_DB}"
  echo "User: ${POSTGRES_USER}"
  query_permission_tables
}

apply() {
  require_container "${POSTGRES_CONTAINER}"
  require_container "${APP_CONTAINER}"
  if [[ ! -r "${MIGRATION_FILE}" ]]; then
    echo "ERROR: migration file is not readable: ${MIGRATION_FILE}" >&2
    exit 1
  fi

  POSTGRES_USER=$(postgres_setting POSTGRES_USER)
  POSTGRES_DB=$(postgres_setting POSTGRES_DB)
  export POSTGRES_USER POSTGRES_DB

  mkdir -p "${BACKUP_DIR}"
  local timestamp
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)
  local backup_file="${BACKUP_DIR}/${POSTGRES_DB}-before-permission-migration-${timestamp}.dump"

  echo "Creating backup: ${backup_file}"
  docker exec "${POSTGRES_CONTAINER}" \
    pg_dump -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" --format=custom \
    >"${backup_file}"
  if [[ ! -s "${backup_file}" ]]; then
    echo "ERROR: backup is empty: ${backup_file}" >&2
    exit 1
  fi
  docker exec -i "${POSTGRES_CONTAINER}" pg_restore --list <"${backup_file}" >/dev/null

  local app_was_running
  app_was_running=$(docker inspect -f '{{.State.Running}}' "${APP_CONTAINER}")
  if [[ "${app_was_running}" == "true" ]]; then
    docker stop "${APP_CONTAINER}" >/dev/null
  fi

  restore_app() {
    if [[ "${app_was_running}" == "true" ]]; then
      docker start "${APP_CONTAINER}" >/dev/null
    fi
  }
  trap restore_app EXIT

  docker exec -i "${POSTGRES_CONTAINER}" \
    psql -X -v ON_ERROR_STOP=1 \
    -U "${POSTGRES_USER}" \
    -d "${POSTGRES_DB}" \
    <"${MIGRATION_FILE}"

  restore_app
  trap - EXIT

  echo "Permission-table status:"
  query_permission_tables
  echo "Migration completed. Backup retained at: ${backup_file}"
}

main() {
  case "${1:-check}" in
    check)
      check
      ;;
    apply)
      apply
      ;;
    help|-h|--help)
      usage
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
}

main "$@"
