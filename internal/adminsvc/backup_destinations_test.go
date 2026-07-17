package adminsvc

import (
	"encoding/json"
	"testing"
)

func TestBackupDestinationInputValidatesProviderAndConfig(t *testing.T) {
	good := backupDestinationInput{Name: "primary-r2", Provider: "s3", Config: json.RawMessage(`{"endpoint":"https://x","bucket":"ddag","prefix":"prod"}`)}
	if err := good.validate(); err != nil {
		t.Fatalf("valid destination rejected: %v", err)
	}
	bad := good
	bad.Provider = "dropbox"
	if err := bad.validate(); err == nil {
		t.Fatal("unknown provider accepted")
	}
	bad = good
	bad.Config = json.RawMessage(`{"token":"plaintext-secret"}`)
	if err := bad.validate(); err == nil {
		t.Fatal("secret-like config field accepted")
	}
}

func TestBackupDestinationPublicViewNeverContainsCredential(t *testing.T) {
	in := backupDestinationInput{Name: "remote", Provider: "sftp", Config: json.RawMessage(`{"host":"backup.example","path":"/ddag"}`), Credential: json.RawMessage(`{"private_key":"secret"}`)}
	out := in.publicConfig()
	if string(out) != `{"host":"backup.example","path":"/ddag"}` {
		t.Fatalf("public config=%s", out)
	}
}

func TestBackupDestinationProviderRequirements(t *testing.T) {
	if err := validateBackupDestinationCredential("local", nil); err != nil {
		t.Fatalf("local should not require credential: %v", err)
	}
	if err := validateBackupDestinationCredential("s3", nil); err == nil {
		t.Fatal("s3 without credential accepted")
	}
	if err := validateBackupDestinationCredential("s3", json.RawMessage(`{"access_key_id":"x","secret_access_key":"y"}`)); err != nil {
		t.Fatalf("s3 credential rejected: %v", err)
	}
}
