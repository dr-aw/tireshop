package main

import (
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProductionConnection(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	dir := t.TempDir()
	ca, password := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "password")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(password, []byte("synthetic-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"PGSSLROOTCERT": ca, "MIGRATION_PASSWORD_FILE": password, "PGHOST": "wrong.invalid", "PGPORT": "1", "PGDATABASE": "wrong", "PGUSER": "wrong", "PGSSLMODE": "disable", "PGSERVICE": "", "PGSERVICEFILE": "", "PGHOSTADDR": "", "PGOPTIONS": "", "PGPASSWORD": "", "PGPASSFILE": ""} {
		t.Setenv(key, value)
	}
	cfg, err := connectionConfig(true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "pg-2f7b3aad-snorchello-5783.g.aivencloud.com" || cfg.Port != 22201 || cfg.Database != "defaultdb" || cfg.User != "defaultdb_migration_login" || cfg.Password != "synthetic-password" {
		t.Fatal("production target or credentials not fixed")
	}
	if cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify || cfg.TLSConfig.ServerName != cfg.Host || len(cfg.Fallbacks) > 0 {
		t.Fatal("verify-full TLS required without fallbacks")
	}
	if cfg.RuntimeParams["role"] != "defaultdb_migrator" {
		t.Fatalf("migration role required: %v", cfg.RuntimeParams)
	}
	if err := os.Chmod(password, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := connectionConfig(true); err == nil {
		t.Fatal("public password file accepted")
	}
}

func TestLocalGuard(t *testing.T) {
	t.Setenv("PGDATABASE", "defaultdb")
	if _, err := connectionConfig(false); err == nil {
		t.Fatal("production database accepted without explicit flag")
	}
}
