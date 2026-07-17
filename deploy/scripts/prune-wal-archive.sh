#!/usr/bin/env bash
# Keep local WAL files for the same 30-day recovery window as logical backups.
# WAL archive is private to postgres; this script must run as that account.
set -euo pipefail

archive_dir="${DDAG_WAL_ARCHIVE_DIR:-/var/lib/postgresql/18/wal-archive}"
retention_days="${DDAG_WAL_RETENTION_DAYS:-30}"

[[ "$retention_days" =~ ^[1-9][0-9]*$ ]] || { echo "invalid DDAG_WAL_RETENTION_DAYS" >&2; exit 2; }
[[ -d "$archive_dir" ]] || { echo "missing WAL archive: $archive_dir" >&2; exit 2; }

# PostgreSQL never reads archived WAL from this directory while running; recovery
# uses a separate copy of this archive. Prune only files older than the agreed RPO.
find "$archive_dir" -maxdepth 1 -type f -mtime "+$retention_days" -print -delete
printf 'WAL_RETENTION_OK archive=%s days=%s\n' "$archive_dir" "$retention_days"
