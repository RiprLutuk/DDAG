package adminsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/go-chi/chi/v5"
)

// listJobs and runJob preserve the existing safe maintenance-job API.
func (s *service) listJobs(w http.ResponseWriter, r *http.Request) {
	if err := s.ensureSelfManagementDefaults(r.Context()); err != nil {
		storeErr(w, r, err)
		return
	}
	rows, err := s.store.ListMaintenanceJobs(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, rows)
}

func (s *service) runJob(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if err := s.ensureSelfManagementDefaults(r.Context()); err != nil {
		storeErr(w, r, err)
		return
	}
	start := time.Now()
	runner := newSafeJobRunner(map[string]jobFunc{
		"cleanup_expired_tokens": s.noopJob,
		"purge_old_request_logs": s.purgeOldLogsJob,
		"service_health_sweep":   s.healthSweepJob,
		"metadata_backup":        s.metadataBackupJob,
	})
	result, err := runner.run(r.Context(), key)
	status, msg := "success", ""
	if err != nil {
		status, msg, result = "failed", err.Error(), json.RawMessage(`{}`)
	}
	_ = s.store.RecordMaintenanceJobRun(r.Context(), key, status, int(time.Since(start).Milliseconds()), msg, result)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "run_maintenance_job", "maintenance_job", key, nil))
	ok(w, r, map[string]any{"key": key, "status": status, "result": json.RawMessage(result)})
}

func (s *service) listBackupRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListBackupRuns(r.Context(), 50)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, runs)
}

func (s *service) runLogicalBackup(w http.ResponseWriter, r *http.Request) {
	s.triggerBackupOrDrill(w, r, "logical_backup")
}

func (s *service) runRestoreDrill(w http.ResponseWriter, r *http.Request) {
	s.triggerBackupOrDrill(w, r, "restore_drill")
}

func (s *service) triggerBackupOrDrill(w http.ResponseWriter, r *http.Request, key string) {
	runID, err := s.store.CreateBackupRun(r.Context(), models.BackupRunCreate{
		Kind:         key,
		DatabaseName: s.cfg.Metadata.Database,
	})
	if err != nil {
		httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to record run kickoff")
		return
	}

	go func() {
		ctx := context.Background()
		runner := "/var/www/DDAG/bin/backup-runner"
		if _, err := os.Stat(runner); err != nil {
			_ = s.store.CompleteBackupRun(ctx, runID, "failed", "", "", "", 0, nil, fmt.Sprintf("runner executable not found: %s", err))
			return
		}

		cmd := exec.Command(runner)
		if key == "restore_drill" {
			cmd = exec.Command(runner, "-drill")
		}

		cmd.Env = append(os.Environ(),
			fmt.Sprintf("DDAG_BACKUP_KEY=%s", s.cfg.Secret.MasterKeyB64),
			fmt.Sprintf("DDAG_DB_HOST=%s", s.cfg.Metadata.Host),
			fmt.Sprintf("DDAG_DB_PORT=%d", s.cfg.Metadata.Port),
			fmt.Sprintf("DDAG_DB_USER=%s", s.cfg.Metadata.User),
			fmt.Sprintf("DDAG_DB_NAME=%s", s.cfg.Metadata.Database),
			fmt.Sprintf("DDAG_DB_SSLMODE=%s", s.cfg.Metadata.SSLMode),
		)
		if s.cfg.Metadata.Password != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", s.cfg.Metadata.Password))
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errStr := stderr.String()
			if errStr == "" {
				errStr = err.Error()
			}
			_ = s.store.CompleteBackupRun(ctx, runID, "failed", "", "", "", 0, nil, strings.TrimSpace(errStr))
			return
		}

		out := strings.TrimSpace(stdout.String())
		if key == "logical_backup" && strings.Contains(out, "BACKUP_OK") {
			parts := strings.Split(out, " ")
			var art, sum string
			var sz int64
			for _, p := range parts {
				if strings.HasPrefix(p, "artifact=") {
					art = strings.TrimPrefix(p, "artifact=")
				}
				if strings.HasPrefix(p, "sha256=") {
					sum = strings.TrimPrefix(p, "sha256=")
				}
				if strings.HasPrefix(p, "bytes=") {
					sz, _ = strconv.ParseInt(strings.TrimPrefix(p, "bytes="), 10, 64)
				}
			}
			_ = s.store.CompleteBackupRun(ctx, runID, "succeeded", art, art+".manifest.json", sum, sz, nil, "")
		} else if key == "restore_drill" && strings.Contains(out, "RESTORE_DRILL_OK") {
			parts := strings.Split(out, " ")
			var art, dbname, tbls string
			for _, p := range parts {
				if strings.HasPrefix(p, "artifact=") {
					art = strings.TrimPrefix(p, "artifact=")
				}
				if strings.HasPrefix(p, "database=") {
					dbname = strings.TrimPrefix(p, "database=")
				}
				if strings.HasPrefix(p, "tables=") {
					tbls = strings.TrimPrefix(p, "tables=")
				}
			}
			det, _ := json.Marshal(map[string]any{"temporary_database": dbname, "restored_tables": tbls})
			_ = s.store.CompleteBackupRun(ctx, runID, "succeeded", art, "", "", 0, det, "")
		} else {
			_ = s.store.CompleteBackupRun(ctx, runID, "failed", "", "", "", 0, nil, fmt.Sprintf("invalid runner output: %s", out))
		}
	}()

	ok(w, r, map[string]any{"ok": true, "run_id": runID})
}
