# DDAG Backup, Recovery, and PITR Runbook

This runbook covers the **metadata PostgreSQL** database (`ddag`), which holds DDAG configuration, users, API definitions, audit data, and encrypted source credentials. It does not replace backups of external source databases managed through DDAG connectors.

## Recovery design

| Control | Schedule / retention | Evidence | Purpose |
|---|---:|---|---|
| Encrypted logical backup | daily at 02:15 WIB (up to 10-minute random delay); 30 days | AES-256-GCM artifact, SHA-256 manifest, `backup_runs`, systemd journal | Recover a complete metadata database |
| Isolated restore drill | Sunday 03:15 WIB (up to 10-minute random delay) | temporary DB restore, application-table count, cleanup, `backup_runs` | Prove backups are decryptable and restorable |
| PostgreSQL WAL archive | continuous; 30-day archive pruning at 04:15 WIB | `pg_stat_archiver`, private WAL archive, Prometheus metrics | Point-in-time recovery within retained archive window |
| Prometheus evaluation | every 15 seconds | backup/WAL/connection rules | Detect stale recovery controls and PostgreSQL pressure |

All schedules use `Persistent=true`: a missed timer is run after the host returns.

## Security boundaries

- Backup artifacts are AES-256-GCM encrypted before remaining on disk.
- Backup scheduler credentials live only in `/etc/ddag/backup-runner.env`, root-readable (`0600`); no credential is stored in the repository or systemd unit.
- Artifacts in `/var/www/DDAG/var/backups` are private to the DDAG service account.
- WAL archive is private to `postgres` at `/var/lib/postgresql/18/wal-archive`.
- The recovery key is the DDAG master-key material. Keep an independent, access-controlled escrow of it. **An encrypted backup without this key is not recoverable.**

## Installed services and timers

```bash
sudo systemctl list-timers --all | grep ddag-
sudo systemctl status ddag-logical-backup.service
sudo systemctl status ddag-restore-drill.service
sudo systemctl status ddag-wal-retention.service
sudo systemctl status ddag-backup-metrics.service
```

| Unit | Function |
|---|---|
| `ddag-logical-backup.timer` | Starts encrypted logical backup daily |
| `ddag-restore-drill.timer` | Starts weekly isolated restore drill |
| `ddag-wal-retention.timer` | Prunes archived WAL older than 30 days |
| `ddag-backup-metrics.timer` | Refreshes Prometheus textfile evidence every minute |

`Type=oneshot` jobs show `inactive (dead)` after success. Use `status=0/SUCCESS` and journal output, not the active state alone, as success evidence.

## Reinstall and replacement-host readiness

The repository contains the backup code and unit templates, but deliberately
excludes backup artifacts and environment files. A fresh clone therefore does
**not** contain historical backups or the key required to decrypt them.

| Item | In Git | Required on a replacement host |
|---|---:|---|
| Backup runner source and systemd templates | Yes | Build/install from the selected release |
| Scheduler environment and encryption key | No | Restore through the approved secret-management process |
| Encrypted artifacts and manifests | No | Copy from an independently protected destination |
| Local WAL archive | No | Restore only when performing a validated PITR recovery |

Before declaring a replacement host recoverable:

1. Install the selected DDAG release, PostgreSQL tooling, and the backup systemd
   units/timers.
2. Provision the scheduler environment with owner-only permissions; never place
   it in Git, tickets, shell history, or chat.
3. Restore encrypted artifacts and manifests from the independent backup
   destination. A local-only copy does not survive loss of the host or disk.
4. Enable the timers, run an on-demand backup, then run an isolated restore
   drill.
5. Record the artifact hash, release SHA, drill timestamp, and result in the
   change or incident record.

The verification commands below are the acceptance gate; a timer being enabled
is not evidence that recovery works.

## Operator procedures

### Run an on-demand backup

```bash
sudo systemctl start ddag-logical-backup.service
sudo journalctl -u ddag-logical-backup.service -n 40 --no-pager
```

Expected journal marker:

```text
BACKUP_OK artifact=...dump.aes sha256=... bytes=...
```

### Run an on-demand restore drill

```bash
sudo systemctl start ddag-restore-drill.service
sudo journalctl -u ddag-restore-drill.service -n 40 --no-pager
sudo -u postgres psql -X -d postgres -Atc "SELECT count(*) FROM pg_database WHERE datname LIKE 'ddag_restore_drill_%';"
```

Expected journal marker:

```text
RESTORE_DRILL_OK artifact=... database=ddag_restore_drill_... tables=N
```

The final query must return `0`: the temporary drill database is removed after verification. No plaintext `.restore-drill-*.dump` should remain under the backup root.

### Inspect PITR/WAL status

```bash
sudo -u postgres psql -X -d postgres -P pager=off -c \
  "SELECT archived_count, failed_count, last_archived_wal, last_archived_time FROM pg_stat_archiver;"
sudo systemctl start ddag-wal-retention.service
sudo journalctl -u ddag-wal-retention.service -n 20 --no-pager
```

`failed_count` must remain `0`. The archive directory alone is not a full PITR recovery environment: an incident recovery also needs a clean PostgreSQL cluster/data directory, a matching PostgreSQL major version, the retained WAL, and the recovery target time.

### Restore procedure (incident only)

1. Declare the incident; stop DDAG writers and preserve affected evidence.
2. Restore onto an isolated host/cluster first. Do **not** overwrite production before validation.
3. Obtain the encrypted artifact and the recovery key through the approved secret-management process.
4. Decrypt only in a protected temporary filesystem, then run `pg_restore --exit-on-error --no-owner --no-privileges` into a new database.
5. Validate schema/table counts and an application smoke test before cutover.
6. For a point-in-time target, configure `restore_command` to read the retained WAL archive and set `recovery_target_time` on the isolated recovery cluster.
7. Record exact artifact SHA-256, target time, validation result, and cutover decision in the incident record.

Never place the recovery key, database password, or decrypted dump in tickets, chat, shell history, or source control.

## Monitoring and alert rules

Prometheus obtains custom metrics from node-exporter's local textfile collector. Verify with:

```bash
curl -fsS http://127.0.0.1:9100/metrics | grep '^ddag_\(backup\|postgres_wal\|postgres_active\)'
curl -fsS http://127.0.0.1:9090/api/v1/rules
```

| Alert | Meaning | First action |
|---|---|---|
| `DDAGLogicalBackupStale` | no successful logical backup for 26h | inspect logical-backup journal and artifact storage |
| `DDAGRestoreDrillStale` | no successful drill for 8d | run isolated drill and investigate failure |
| `DDAGWALArchiveFailure` | WAL archive failures increased | inspect PostgreSQL archive command, permissions, capacity |
| `DDAGWALArchivingStale` | no archive for >1h | confirm write activity; inspect archiver if writes occurred |
| `DDAGPostgresConnectionPressure` | >85% max connections for 10m | inspect client pools/long sessions before changing limits |

These rules are evaluated by Prometheus. This host currently has no configured Alertmanager receiver, SMTP relay, or Telegram webhook; configure a receiver before treating this as out-of-band paging.

## Offsite copy status

DDAG supports encrypted destinations in the dashboard (S3-compatible, B2, Google Drive, SFTP, and local). A verified destination is a prerequisite, but no offsite destination is configured in this environment yet. Therefore the current recovery copy is **encrypted local disk only** and does not survive loss of the VPS/disk.

To finish the DR control, create a dedicated least-privilege offsite bucket/folder, add it in **Backup & Recovery**, run its verification, then implement/test artifact delivery and a restore from that offsite copy. Do not claim offsite recovery readiness until that end-to-end test has passed.

## PgBouncer decision

PgBouncer is intentionally **not installed** on this host at this time:

- metadata PostgreSQL is configured for 50 connections and observed usage is low;
- the VPS has approximately 0.9 GB RAM with limited headroom;
- DDAG already uses application connection pools, and another proxy adds memory, failure modes, and transaction/session-pooling compatibility work.

Reassess when connection utilization stays above 70%, connection churn causes CPU/latency pressure, or DDAG services are scaled to multiple replicas. If adopted, bind PgBouncer to localhost, use `auth_file`/SCRAM and least-privilege DB users, choose pooling mode only after validating session semantics, and load-test before cutover.

## Deployment verification

```bash
sudo systemd-analyze verify /etc/systemd/system/ddag-*.service /etc/systemd/system/ddag-*.timer
promtool check config /etc/prometheus/prometheus.yml
sudo systemctl list-timers --all | grep ddag-
sudo -u postgres psql -X -d postgres -P pager=off -c "SELECT archived_count, failed_count, last_archived_time FROM pg_stat_archiver;"
```

The DDAG dashboard is operational evidence, but the systemd journal, PostgreSQL statistics, artifact manifest/hash, and Prometheus rule state are the authoritative host-level verification sources.
