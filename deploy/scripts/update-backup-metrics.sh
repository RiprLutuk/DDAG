#!/usr/bin/env bash
# Expose backup/restore and PostgreSQL recovery evidence through node-exporter's
# textfile collector. This is intentionally read-only against PostgreSQL.
set -euo pipefail

output_file="${DDAG_PROM_METRICS_FILE:-/var/lib/prometheus/node-exporter/ddag-backup.prom}"
db_name="${DDAG_DB_NAME:-ddag}"
tmp_file=$(mktemp)
trap 'rm -f "$tmp_file"' EXIT

# Metadata PostgreSQL is local. Running psql as postgres uses peer authentication
# over the Unix socket, avoiding passwords in the monitoring path.
query() { runuser -u postgres -- psql -X -d "$db_name" -At -F '|' -c "$1"; }
query_postgres() { runuser -u postgres -- psql -X -d postgres -At -F '|' -c "$1"; }

{
  echo '# HELP ddag_backup_runs_total Total count of backup and restore drill runs.'
  echo '# TYPE ddag_backup_runs_total counter'
  while IFS='|' read -r kind status count; do
    [[ -n "$kind" ]] || continue
    printf 'ddag_backup_runs_total{kind="%s",status="%s"} %s\n' "$kind" "$status" "$count"
  done < <(query 'SELECT kind, status, count(*) FROM backup_runs GROUP BY kind, status ORDER BY kind, status')

  echo '# HELP ddag_backup_last_success_timestamp_seconds Epoch timestamp of the latest successful run by kind.'
  echo '# TYPE ddag_backup_last_success_timestamp_seconds gauge'
  echo '# HELP ddag_backup_last_size_bytes Artifact bytes from the latest successful logical backup.'
  echo '# TYPE ddag_backup_last_size_bytes gauge'
  while IFS='|' read -r kind completed bytes; do
    [[ -n "$kind" ]] || continue
    printf 'ddag_backup_last_success_timestamp_seconds{kind="%s"} %s\n' "$kind" "$completed"
    printf 'ddag_backup_last_size_bytes{kind="%s"} %s\n' "$kind" "${bytes:-0}"
  done < <(query "SELECT DISTINCT ON (kind) kind, extract(epoch FROM completed_at)::bigint, bytes FROM backup_runs WHERE status = 'succeeded' AND completed_at IS NOT NULL ORDER BY kind, completed_at DESC")

  echo '# HELP ddag_postgres_wal_archived_total PostgreSQL WAL segments archived successfully.'
  echo '# TYPE ddag_postgres_wal_archived_total counter'
  echo '# HELP ddag_postgres_wal_archive_failures_total PostgreSQL WAL archive failures.'
  echo '# TYPE ddag_postgres_wal_archive_failures_total counter'
  echo '# HELP ddag_postgres_wal_last_archived_timestamp_seconds Epoch timestamp of last archived WAL segment; 0 if none.'
  echo '# TYPE ddag_postgres_wal_last_archived_timestamp_seconds gauge'
  IFS='|' read -r archived failed last_archived < <(query_postgres "SELECT archived_count, failed_count, COALESCE(extract(epoch FROM last_archived_time)::bigint,0) FROM pg_stat_archiver")
  printf 'ddag_postgres_wal_archived_total %s\n' "${archived:-0}"
  printf 'ddag_postgres_wal_archive_failures_total %s\n' "${failed:-0}"
  printf 'ddag_postgres_wal_last_archived_timestamp_seconds %s\n' "${last_archived:-0}"

  echo '# HELP ddag_postgres_active_connections Current active connections to metadata database.'
  echo '# TYPE ddag_postgres_active_connections gauge'
  echo '# HELP ddag_postgres_max_connections PostgreSQL configured maximum connections.'
  echo '# TYPE ddag_postgres_max_connections gauge'
  active=$(query "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()")
  max=$(query 'SHOW max_connections')
  printf 'ddag_postgres_active_connections %s\n' "${active:-0}"
  printf 'ddag_postgres_max_connections %s\n' "${max:-0}"
} >"$tmp_file"

install -o prometheus -g prometheus -m 0644 "$tmp_file" "$output_file"
