// Command backup-runner creates an encrypted PostgreSQL custom-format logical backup.
// It is designed for systemd: stdout/stderr are the audit trail and non-zero exits
// make timer failures visible. It never prints database credentials.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type manifest struct {
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	Database   string    `json:"database"`
	Artifact   string    `json:"artifact"`
	SHA256     string    `json:"sha256"`
	Bytes      int64     `json:"bytes"`
	Encryption string    `json:"encryption"`
	Nonce      string    `json:"nonce"`
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	drill := flag.Bool("drill", false, "restore latest encrypted artifact into isolated temporary database")
	flag.Parse()
	root := env("DDAG_BACKUP_ROOT", "/var/www/DDAG/var/backups")
	retention, err := strconv.Atoi(env("DDAG_BACKUP_RETENTION_DAYS", "30"))
	if err != nil || retention < 1 || retention > 3650 {
		fatal("invalid DDAG_BACKUP_RETENTION_DAYS")
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("DDAG_BACKUP_KEY"))
	if err != nil || len(key) != 32 {
		fatal("DDAG_BACKUP_KEY must be a base64-encoded 32-byte key")
	}
	if *drill {
		if err := runDrill(root, key); err != nil {
			fatalErr(err)
		}
		return
	}
	runBackup(root, key, retention)
}
func runBackup(root string, key []byte, retention int) {
	if err := os.MkdirAll(root, 0700); err != nil {
		fatalErr(err)
	}
	db := env("DDAG_DB_NAME", "ddag")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	base := fmt.Sprintf("%s-%s.dump.aes", db, stamp)
	dest := filepath.Join(root, base)
	tmp, err := os.CreateTemp(root, ".ddag-backup-*.tmp")
	if err != nil {
		fatalErr(err)
	}
	defer os.Remove(tmp.Name())
	cmd := exec.Command("pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--dbname="+dsn())
	cmd.Stdout = tmp
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatalErr(fmt.Errorf("pg_dump: %w", err))
	}
	if err := tmp.Close(); err != nil {
		fatalErr(err)
	}
	if err := encryptFile(tmp.Name(), dest, key); err != nil {
		fatalErr(err)
	}
	sum, n, err := fileHash(dest)
	if err != nil {
		fatalErr(err)
	}
	_, nonce, err := decryptHeader(dest, key)
	if err != nil {
		fatalErr(err)
	}
	m := manifest{1, time.Now().UTC(), db, base, sum, n, "AES-256-GCM", base64.StdEncoding.EncodeToString(nonce)}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(dest+".manifest.json", b, 0600); err != nil {
		fatalErr(err)
	}
	cleanup(root, time.Duration(retention)*24*time.Hour)
	fmt.Printf("BACKUP_OK artifact=%s sha256=%s bytes=%d\n", dest, sum, n)
}
func runDrill(root string, key []byte) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var newest string
	var newestT time.Time
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".dump.aes") {
			info, _ := e.Info()
			if info != nil && info.ModTime().After(newestT) {
				newestT = info.ModTime()
				newest = filepath.Join(root, e.Name())
			}
		}
	}
	if newest == "" {
		return errors.New("no encrypted backup artifact found")
	}
	plain, err := os.CreateTemp(root, ".restore-drill-*.dump")
	if err != nil {
		return err
	}
	if err := plain.Close(); err != nil {
		return err
	}
	defer cleanupTransient(plain.Name())
	if err := decryptFile(newest, plain.Name(), key); err != nil {
		return err
	}

	name := "ddag_restore_drill_" + time.Now().UTC().Format("20060102150405")
	createdb := exec.Command("createdb", createDBArgs(name)...)
	if output, err := createdb.CombinedOutput(); err != nil {
		return fmt.Errorf("create isolated drill database: %w: %s", err, strings.TrimSpace(string(output)))
	}
	defer func() {
		if output, err := exec.Command("dropdb", dropDBArgs(name)...).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "restore drill cleanup failed for %s: %v: %s\n", name, err, strings.TrimSpace(string(output)))
		}
	}()

	restore := exec.Command("pg_restore", "--exit-on-error", "--no-owner", "--no-privileges", "--dbname="+dbDSN(name), plain.Name())
	if output, err := restore.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_restore: %w: %s", err, strings.TrimSpace(string(output)))
	}
	out, err := exec.Command("psql", "--no-psqlrc", "--tuples-only", "--no-align", "--dbname="+dbDSN(name), "-c", "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) == "0" {
		return errors.New("restore drill completed but no application tables were found")
	}
	fmt.Printf("RESTORE_DRILL_OK artifact=%s database=%s tables=%s\n", newest, name, strings.TrimSpace(string(out)))
	return nil
}

func createDBArgs(name string) []string {
	return []string{"-h", env("DDAG_DB_HOST", "127.0.0.1"), "-p", env("DDAG_DB_PORT", "5432"), "-U", env("DDAG_DB_USER", "postgres"), name}
}

func dropDBArgs(name string) []string {
	return append([]string{"--if-exists"}, createDBArgs(name)...)
}

func cleanupTransient(path string) error { return os.Remove(path) }
func dsn() string                        { return dbDSN(env("DDAG_DB_NAME", "ddag")) }
func adminDSN() string                   { return dbDSN("postgres") }
func dbDSN(db string) string {
	host := env("DDAG_DB_HOST", "127.0.0.1")
	port := env("DDAG_DB_PORT", "5432")
	user := env("DDAG_DB_USER", "postgres")
	ssl := env("DDAG_DB_SSLMODE", "disable")
	return fmt.Sprintf("postgresql://%s@%s:%s/%s?sslmode=%s", user, host, port, db, ssl)
}
func encryptFile(src, dst string, key []byte) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer out.Close()
	plain, e := io.ReadAll(in)
	if e != nil {
		return e
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return e
	}
	if _, e = out.Write([]byte("DDAGBKP1")); e != nil {
		return e
	}
	if _, e = out.Write(nonce); e != nil {
		return e
	}
	_, e = out.Write(g.Seal(nil, nonce, plain, nil))
	return e
}
func decryptHeader(src string, key []byte) ([]byte, []byte, error) {
	b, e := os.ReadFile(src)
	if e != nil {
		return nil, nil, e
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, nil, e
	}
	if len(b) < 8+g.NonceSize() || string(b[:8]) != "DDAGBKP1" {
		return nil, nil, errors.New("invalid backup artifact")
	}
	p, e := g.Open(nil, b[8:8+g.NonceSize()], b[8+g.NonceSize():], nil)
	return p, b[8 : 8+g.NonceSize()], e
}
func decryptFile(src, dst string, key []byte) error {
	p, _, e := decryptHeader(src, key)
	if e != nil {
		return e
	}
	return os.WriteFile(dst, p, 0600)
}
func fileHash(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, e
}
func cleanup(root string, age time.Duration) {
	entries, _ := os.ReadDir(root)
	cut := time.Now().Add(-age)
	for _, e := range entries {
		if !(strings.HasSuffix(e.Name(), ".dump.aes") || strings.HasSuffix(e.Name(), ".manifest.json")) {
			continue
		}
		info, _ := e.Info()
		if info != nil && info.ModTime().Before(cut) {
			_ = os.Remove(filepath.Join(root, e.Name()))
		}
	}
}
func fatal(s string)   { fmt.Fprintln(os.Stderr, "BACKUP_FAILED", s); os.Exit(1) }
func fatalErr(e error) { fatal(e.Error()) }
