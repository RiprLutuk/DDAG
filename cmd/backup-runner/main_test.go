package main

import (
	"os"
	"reflect"
	"testing"
)

func TestCreateDBArgsUsesConnectionFlagsNotUnsupportedDBNameURI(t *testing.T) {
	t.Setenv("DDAG_DB_HOST", "127.0.0.1")
	t.Setenv("DDAG_DB_PORT", "5432")
	t.Setenv("DDAG_DB_USER", "postgres")

	want := []string{"-h", "127.0.0.1", "-p", "5432", "-U", "postgres", "ddag_restore_drill_test"}
	if got := createDBArgs("ddag_restore_drill_test"); !reflect.DeepEqual(got, want) {
		t.Fatalf("createDBArgs() = %#v, want %#v", got, want)
	}
}

func TestCleanupRemovesTransientRestorePlaintext(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.restore-drill-test.dump"
	if err := os.WriteFile(path, []byte("plaintext"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupTransient(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("plaintext restore artifact remains: %v", err)
	}
}
