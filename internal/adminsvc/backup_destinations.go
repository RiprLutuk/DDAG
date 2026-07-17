package adminsvc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type backupDestinationInput struct {
	Name       string          `json:"name"`
	Provider   string          `json:"provider"`
	Config     json.RawMessage `json:"config"`
	Credential json.RawMessage `json:"credential,omitempty"` // write-only; never returned
	Enabled    bool            `json:"enabled"`
}

func (in backupDestinationInput) publicConfig() json.RawMessage { return in.Config }
func (in backupDestinationInput) validate() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return fmt.Errorf("name is required")
	}
	if err := validateBackupProvider(in.Provider); err != nil {
		return err
	}
	if len(in.Config) == 0 {
		in.Config = json.RawMessage(`{}`)
	}
	var cfg map[string]any
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return fmt.Errorf("config must be a JSON object")
	}
	if cfg == nil {
		return fmt.Errorf("config must be a JSON object")
	}
	if hasSensitiveConfigKey(cfg) {
		return fmt.Errorf("credentials must be supplied in credential, not config")
	}
	return nil
}
func hasSensitiveConfigKey(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			if sensitiveBackupKey(k) || hasSensitiveConfigKey(vv) {
				return true
			}
		}
	case []any:
		for _, vv := range x {
			if hasSensitiveConfigKey(vv) {
				return true
			}
		}
	}
	return false
}
func validateBackupDestinationCredential(provider string, raw json.RawMessage) error {
	if provider == "local" {
		return nil
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || bytes.Equal(bytes.TrimSpace(raw), []byte("{}")) {
		return fmt.Errorf("credential is required for %s", provider)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || len(value) == 0 {
		return fmt.Errorf("credential must be a non-empty JSON object")
	}
	return nil
}
func (s *service) listBackupDestinations(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.ListBackupDestinations(r.Context())
	if err != nil {
		storeErr(w, r, err)
		return
	}
	ok(w, r, out)
}
func (s *service) createBackupDestination(w http.ResponseWriter, r *http.Request) {
	var in backupDestinationInput
	if !decode(w, r, &in) {
		return
	}
	if err := in.validate(); err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, err.Error())
		return
	}
	if err := validateBackupDestinationCredential(in.Provider, in.Credential); err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, err.Error())
		return
	}
	var ref *uuid.UUID
	if len(in.Credential) > 0 && in.Provider != "local" {
		id, err := s.secrets.Put(r.Context(), in.Credential, "backup_destination_credential")
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to store destination credential")
			return
		}
		ref = &id
	}
	status := "draft"
	if in.Provider == "local" {
		status = "enabled"
	}
	actor := principalOf(r).UserID
	id, err := s.store.CreateBackupDestination(r.Context(), &models.BackupDestination{Name: strings.TrimSpace(in.Name), Provider: in.Provider, Config: in.publicConfig(), CredentialSecretRef: ref, Status: status, CreatedBy: &actor})
	if err != nil {
		if ref != nil {
			_ = s.secrets.Delete(r.Context(), *ref)
		}
		httpx.ErrorCode(w, r, httpx.CodeConflict, "destination name may already exist")
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "create_backup_destination", "backup_destination", id.String(), map[string]any{"name": in.Name, "provider": in.Provider}))
	out, _ := s.store.GetBackupDestination(r.Context(), id)
	httpx.Created(w, r, out)
}
func (s *service) updateBackupDestination(w http.ResponseWriter, r *http.Request) {
	id, okID := idParam(w, r)
	if !okID {
		return
	}
	var in backupDestinationInput
	if !decode(w, r, &in) {
		return
	}
	if err := in.validate(); err != nil {
		httpx.ErrorCode(w, r, httpx.CodeValidation, err.Error())
		return
	}
	existing, err := s.store.GetBackupDestination(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	if in.Provider != "local" && existing.CredentialSecretRef == nil && len(in.Credential) == 0 {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "credential is required for non-local destination")
		return
	}
	status := "draft"
	if in.Provider == "local" {
		status = "enabled"
	}
	if err := s.store.UpdateBackupDestination(r.Context(), &models.BackupDestination{ID: id, Name: strings.TrimSpace(in.Name), Provider: in.Provider, Config: in.publicConfig(), Status: status}); err != nil {
		storeErr(w, r, err)
		return
	}
	if len(in.Credential) > 0 {
		if err := validateBackupDestinationCredential(in.Provider, in.Credential); err != nil {
			httpx.ErrorCode(w, r, httpx.CodeValidation, err.Error())
			return
		}
		if existing.CredentialSecretRef != nil {
			err = s.secrets.Update(r.Context(), *existing.CredentialSecretRef, in.Credential)
		} else {
			var ref uuid.UUID
			ref, err = s.secrets.Put(r.Context(), in.Credential, "backup_destination_credential")
			if err == nil {
				err = s.store.SetBackupDestinationCredential(r.Context(), id, ref)
			}
		}
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to store destination credential")
			return
		}
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "update_backup_destination", "backup_destination", id.String(), map[string]any{"name": in.Name, "provider": in.Provider}))
	out, _ := s.store.GetBackupDestination(r.Context(), id)
	ok(w, r, out)
}
func (s *service) deleteBackupDestination(w http.ResponseWriter, r *http.Request) {
	id, okID := idParam(w, r)
	if !okID {
		return
	}
	if err := s.store.DeleteBackupDestination(r.Context(), id); err != nil {
		storeErr(w, r, err)
		return
	}
	s.audit.Write(r.Context(), r, s.actorEvent(r, "delete_backup_destination", "backup_destination", id.String(), nil))
	ok(w, r, map[string]bool{"ok": true})
}
func (s *service) verifyBackupDestination(w http.ResponseWriter, r *http.Request) {
	id, okID := idParam(w, r)
	if !okID {
		return
	}
	d, err := s.store.GetBackupDestination(r.Context(), id)
	if err != nil {
		storeErr(w, r, err)
		return
	}
	if d.Provider == "local" {
		_ = s.store.SetBackupDestinationVerification(r.Context(), id, "verified", "")
		d, _ = s.store.GetBackupDestination(r.Context(), id)
		ok(w, r, map[string]any{"destination": d, "verified": true, "message": "local destination configuration accepted"})
		return
	}
	if !d.HasCredential {
		httpx.ErrorCode(w, r, httpx.CodeValidation, "configure credentials before verification")
		return
	}
	_ = s.store.SetBackupDestinationVerification(r.Context(), id, "failed", "provider adapter is not installed in this DDAG build")
	httpx.ErrorCode(w, r, httpx.CodeValidation, "provider adapter is not installed in this DDAG build")
}

var _ = store.ErrNotFound
var _ = chi.URLParam
