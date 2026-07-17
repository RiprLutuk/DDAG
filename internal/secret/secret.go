// Package secret implements encrypted-at-rest secret storage (PRD §13.4).
//
// EnvelopeStore uses envelope encryption: each secret gets a fresh random 32-byte
// data-encryption key (DEK); the plaintext is sealed with the DEK using
// AES-256-GCM, and the DEK itself is wrapped with a master key (also AES-256-GCM)
// loaded from configuration/KMS. Only ciphertext + wrapped DEK + nonces are
// persisted — plaintext never touches disk or logs. The Store interface lets a
// Vault/KMS-backed implementation drop in later without touching callers.
package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a secret id does not exist.
var ErrNotFound = errors.New("secret not found")

// Store is the secret persistence abstraction.
type Store interface {
	Put(ctx context.Context, plaintext []byte, purpose string) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) ([]byte, error)
	Update(ctx context.Context, id uuid.UUID, plaintext []byte) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// EnvelopeStore is the AES-256-GCM envelope-encryption Store backed by the
// metadata `secrets` table.
type EnvelopeStore struct {
	pool       *pgxpool.Pool
	masterGCM  cipher.AEAD
	keyVersion int
}

const aadKeyVersion = 2

func secretAAD(id uuid.UUID, purpose string, version int) []byte {
	return []byte(fmt.Sprintf("%s|%s|%d", id.String(), purpose, version))
}

// NewEnvelopeStore builds a store from a base64-encoded 32-byte master key.
func NewEnvelopeStore(pool *pgxpool.Pool, masterKeyB64 string) (*EnvelopeStore, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode master key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (got %d) — set DDAG_MASTER_KEY", len(key))
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	return &EnvelopeStore{pool: pool, masterGCM: gcm, keyVersion: 1}, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return gcm, nil
}

// seal encrypts plaintext under a fresh DEK and returns all envelope parts.
func (s *EnvelopeStore) seal(plaintext []byte) (ciphertext, nonce, wrappedDEK, dekNonce []byte, err error) {
	dek := make([]byte, 32)
	if _, err = rand.Read(dek); err != nil {
		return
	}
	dataGCM, err := newGCM(dek)
	if err != nil {
		return
	}
	nonce = make([]byte, dataGCM.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return
	}
	ciphertext = dataGCM.Seal(nil, nonce, plaintext, nil)

	dekNonce = make([]byte, s.masterGCM.NonceSize())
	if _, err = rand.Read(dekNonce); err != nil {
		return
	}
	wrappedDEK = s.masterGCM.Seal(nil, dekNonce, dek, nil)
	return
}

// open reverses seal using the provided AAD for verification.
func (s *EnvelopeStore) open(ciphertext, nonce, wrappedDEK, dekNonce []byte) ([]byte, error) {
	return s.openWithAAD(nil, ciphertext, nonce, wrappedDEK, dekNonce)
}

func (s *EnvelopeStore) openWithAAD(aad, ciphertext, nonce, wrappedDEK, dekNonce []byte) ([]byte, error) {
	dek, err := s.masterGCM.Open(nil, dekNonce, wrappedDEK, nil)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	dataGCM, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := dataGCM.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plaintext, nil
}

func (s *EnvelopeStore) sealV2(id uuid.UUID, purpose string, plaintext []byte) (ciphertext, nonce, wrappedDEK, dekNonce []byte, err error) {
	dek := make([]byte, 32)
	if _, err = rand.Read(dek); err != nil {
		return
	}
	dataGCM, err := newGCM(dek)
	if err != nil {
		return
	}
	nonce = make([]byte, dataGCM.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return
	}
	aad := secretAAD(id, purpose, aadKeyVersion)
	ciphertext = dataGCM.Seal(nil, nonce, plaintext, aad)

	dekNonce = make([]byte, s.masterGCM.NonceSize())
	if _, err = rand.Read(dekNonce); err != nil {
		return
	}
	wrappedDEK = s.masterGCM.Seal(nil, dekNonce, dek, aad)
	return
}

func (s *EnvelopeStore) openV2(id uuid.UUID, purpose string, version int, ciphertext, nonce, wrappedDEK, dekNonce []byte) ([]byte, error) {
	aad := secretAAD(id, purpose, version)
	dek, err := s.masterGCM.Open(nil, dekNonce, wrappedDEK, aad)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	dataGCM, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	plaintext, err := dataGCM.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plaintext, nil
}

// Put encrypts and stores a new secret, returning its id.
func (s *EnvelopeStore) Put(ctx context.Context, plaintext []byte, purpose string) (uuid.UUID, error) {
	if purpose == "" {
		purpose = "generic"
	}
	id := uuid.New()
	ct, nonce, wdek, dnonce, err := s.sealV2(id, purpose, plaintext)
	if err != nil {
		return uuid.Nil, err
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO secrets (id, ciphertext, nonce, wrapped_dek, dek_nonce, key_version, purpose)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		id, ct, nonce, wdek, dnonce, aadKeyVersion, purpose).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert secret: %w", err)
	}
	return id, nil
}

// Get decrypts and returns a secret's plaintext.
func (s *EnvelopeStore) Get(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var ct, nonce, wdek, dnonce []byte
	var version int
	var purpose string
	err := s.pool.QueryRow(ctx,
		`SELECT ciphertext, nonce, wrapped_dek, dek_nonce, key_version, purpose FROM secrets WHERE id=$1`, id).
		Scan(&ct, &nonce, &wdek, &dnonce, &version, &purpose)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get secret: %w", err)
	}
	if version >= aadKeyVersion {
		return s.openV2(id, purpose, version, ct, nonce, wdek, dnonce)
	}
	return s.open(ct, nonce, wdek, dnonce)
}

// Update re-seals an existing secret with new plaintext (value rotation).
func (s *EnvelopeStore) Update(ctx context.Context, id uuid.UUID, plaintext []byte) error {
	var purpose string
	err := s.pool.QueryRow(ctx, `SELECT purpose FROM secrets WHERE id=$1`, id).Scan(&purpose)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("update secret check: %w", err)
	}
	ct, nonce, wdek, dnonce, err := s.sealV2(id, purpose, plaintext)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE secrets SET ciphertext=$2, nonce=$3, wrapped_dek=$4, dek_nonce=$5,
		       key_version=$6, updated_at=now() WHERE id=$1`,
		id, ct, nonce, wdek, dnonce, aadKeyVersion)
	if err != nil {
		return fmt.Errorf("update secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a secret.
func (s *EnvelopeStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM secrets WHERE id=$1`, id)
	return err
}
