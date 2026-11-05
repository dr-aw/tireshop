package runtimeapp

import (
	"crypto/tls"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionTLS(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.pem")
	password := filepath.Join(dir, "password")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(password, []byte("synthetic-test-password"), 0600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"APP_ENV": "production", "PGHOST": "example.com", "PGPORT": "22201", "PGDATABASE": "defaultdb", "PGUSER": "defaultdb_requests_login", "PGSSLROOTCERT": ca, "REQUESTS_PASSWORD_FILE": password, "APP_ORIGINS": "https://shop.example.com", "ADMIN_LOCAL_HTTP": "", "PGSSLMODE": "disable"} {
		t.Setenv(key, value)
	}
	cfg, err := databaseConfig("requests")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxConns != 4 || cfg.ConnConfig.TLSConfig == nil || cfg.ConnConfig.TLSConfig.InsecureSkipVerify || len(cfg.ConnConfig.Fallbacks) != 0 {
		t.Fatal("production must verify TLS without plaintext fallback")
	}
	conn, err := tls.Dial("tcp", strings.TrimPrefix(server.URL, "https://"), cfg.ConnConfig.TLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	t.Setenv("PGHOST", "wrong.invalid")
	cfg, err = databaseConfig("requests")
	if err != nil {
		t.Fatal(err)
	}
	if conn, err = tls.Dial("tcp", strings.TrimPrefix(server.URL, "https://"), cfg.ConnConfig.TLSConfig); err == nil {
		conn.Close()
		t.Fatal("wrong hostname accepted")
	}
	t.Setenv("PGSSLROOTCERT", filepath.Join(dir, "missing.pem"))
	if _, err = databaseConfig("requests"); err == nil {
		t.Fatal("missing CA accepted")
	}
}

func TestDevelopmentGuard(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PGDATABASE", "defaultdb")
	if _, err := databaseConfig("catalog"); err == nil {
		t.Fatal("external database accepted in development")
	}
	t.Setenv("APP_ENV", "typo")
	if _, err := databaseConfig("catalog"); err == nil {
		t.Fatal("unknown mode accepted")
	}
}
