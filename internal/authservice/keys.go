// Package authservice implements the OAuth2 token service (PRD §11.5): token,
// refresh, revoke, introspect endpoints, plus JWKS publication and persistent,
// rotatable RS256 signing keys.
package authservice

import (
	"context"
	"crypto/rsa"
	"sync"

	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/secret"
	"github.com/ddag/ddag/internal/store"
)

// keyManager holds the in-memory signing/verification keys loaded from the
// metadata DB. The newest active key signs; all keys (active + retired) verify.
type keyManager struct {
	mu      sync.RWMutex
	signKID string
	signKey *rsa.PrivateKey
	verify  map[string]*rsa.PublicKey
}

// loadKeyManager ensures at least one active signing key exists (generating one
// on first boot), then loads all keys into memory.
func loadKeyManager(ctx context.Context, st *store.Store, sec secret.Store) (*keyManager, error) {
	if _, err := st.CurrentSigningKey(ctx); err != nil {
		if err := generateSigningKey(ctx, st, sec); err != nil {
			return nil, err
		}
	}
	km := &keyManager{verify: map[string]*rsa.PublicKey{}}
	if err := km.reload(ctx, st, sec); err != nil {
		return nil, err
	}
	return km, nil
}

// generateSigningKey creates a new RSA keypair, stores the private key encrypted
// via the secret store, and records the key as active.
func generateSigningKey(ctx context.Context, st *store.Store, sec secret.Store) error {
	priv, err := auth.GenerateRSAKey()
	if err != nil {
		return err
	}
	privPEM, err := auth.MarshalPrivatePEM(priv)
	if err != nil {
		return err
	}
	pubPEM, err := auth.MarshalPublicPEM(&priv.PublicKey)
	if err != nil {
		return err
	}
	ref, err := sec.Put(ctx, []byte(privPEM), "jwt_private_key")
	if err != nil {
		return err
	}
	kid := "ddag-" + auth.HashToken(pubPEM)[:12]
	_, err = st.CreateSigningKey(ctx, kid, pubPEM, ref, "RS256")
	return err
}

// reload refreshes the in-memory keys from the database.
func (km *keyManager) reload(ctx context.Context, st *store.Store, sec secret.Store) error {
	current, err := st.CurrentSigningKey(ctx)
	if err != nil {
		return err
	}
	privPEM, err := sec.Get(ctx, current.PrivateSecretRef)
	if err != nil {
		return err
	}
	priv, err := auth.ParsePrivatePEM(string(privPEM))
	if err != nil {
		return err
	}

	all, err := st.AllSigningKeys(ctx)
	if err != nil {
		return err
	}
	verify := make(map[string]*rsa.PublicKey, len(all))
	for _, k := range all {
		pub, err := auth.ParsePublicPEM(k.PublicKeyPEM)
		if err == nil {
			verify[k.KID] = pub
		}
	}

	km.mu.Lock()
	km.signKID = current.KID
	km.signKey = priv
	km.verify = verify
	km.mu.Unlock()
	return nil
}

func (km *keyManager) signer() (string, *rsa.PrivateKey) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.signKID, km.signKey
}

// keyfunc resolves a verification key by kid (implements auth.Keyfunc).
func (km *keyManager) keyfunc(kid string) (*rsa.PublicKey, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	k, ok := km.verify[kid]
	return k, ok
}

// jwks builds the JWK set for publication.
func (km *keyManager) jwks() auth.JWKS {
	km.mu.RLock()
	defer km.mu.RUnlock()
	set := auth.JWKS{Keys: make([]auth.JWK, 0, len(km.verify))}
	for kid, pub := range km.verify {
		set.Keys = append(set.Keys, auth.PublicToJWK(kid, pub))
	}
	return set
}
