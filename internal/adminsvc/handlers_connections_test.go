package adminsvc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

type fakeSecretStore struct {
	putErr    error
	updateErr error
	putID     uuid.UUID
}

func (f fakeSecretStore) Put(context.Context, []byte, string) (uuid.UUID, error) {
	if f.putErr != nil {
		return uuid.Nil, f.putErr
	}
	return f.putID, nil
}

func (f fakeSecretStore) Get(context.Context, uuid.UUID) ([]byte, error) { return nil, nil }

func (f fakeSecretStore) Update(context.Context, uuid.UUID, []byte) error {
	return f.updateErr
}

func (f fakeSecretStore) Delete(context.Context, uuid.UUID) error { return nil }

func TestStoreConnectionPasswordReturnsUpdateError(t *testing.T) {
	ref := uuid.New()
	wantErr := errors.New("seal failed")
	_, err := storeConnectionPassword(context.Background(), fakeSecretStore{updateErr: wantErr}, &models.DatabaseConnection{SecretRef: &ref}, "new-password")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestRoutesAcceptAdminPrefix(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	(&service{}).routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for routed invalid JSON request", rec.Code, http.StatusBadRequest)
	}
}
