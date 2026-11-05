// admin-user is an operator provisioning tool, never an HTTP registration endpoint.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"tireshop/internal/adminauth"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	args := os.Args[1:]
	production := len(args) > 0 && args[0] == "--production"
	if production {
		args = args[1:]
	}
	if len(args) != 2 || !regexp.MustCompile(`^[a-z0-9_.-]{1,80}$`).MatchString(args[0]) {
		return fmt.Errorf("usage: admin-user [--production] USERNAME PASSWORD_FILE (12–1024 bytes, file mode 0600)")
	}
	info, err := os.Stat(args[1])
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("password file must be a private regular file (0600)")
	}
	raw, err := os.ReadFile(args[1])
	if err != nil {
		return fmt.Errorf("cannot read password file")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if len(password) < 12 || len(password) > 1024 {
		return fmt.Errorf("password must contain 12–1024 bytes")
	}
	config, err := connectionConfig(production)
	if err != nil {
		return err
	}
	hash := adminauth.Hash(password)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("database connection failed")
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction failed")
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "admin:"+args[0]); err != nil {
		return fmt.Errorf("account lock failed")
	}
	_, err = tx.Exec(ctx, `INSERT INTO requests.admin_users(username,password_hash) VALUES($1,$2)
 ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash,active=true`, args[0], hash)
	if err != nil {
		return fmt.Errorf("account update failed; apply migrations first")
	}
	_, err = tx.Exec(ctx, `DELETE FROM requests.admin_sessions WHERE username=$1`, args[0])
	if err != nil {
		return fmt.Errorf("session revocation failed")
	}
	if tx.Commit(ctx) != nil {
		return fmt.Errorf("commit could not be confirmed")
	}
	fmt.Println("Account saved; previous sessions revoked.")
	return nil
}

func connectionConfig(production bool) (*pgx.ConnConfig, error) {
	if production {
		ca := os.Getenv("PGSSLROOTCERT")
		if !strings.HasPrefix(ca, "/") {
			return nil, fmt.Errorf("absolute PGSSLROOTCERT path required")
		}
		file := os.Getenv("MIGRATION_PASSWORD_FILE")
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("MIGRATION_PASSWORD_FILE must be a private regular file (0600)")
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("cannot read migration password file")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		if password == "" {
			return nil, fmt.Errorf("empty migration password file")
		}
		u := url.URL{Scheme: "postgres", Host: "pg-2f7b3aad-snorchello-5783.g.aivencloud.com:22201", Path: "/defaultdb", User: url.User("defaultdb_migration_login")}
		q := url.Values{"sslmode": {"verify-full"}, "sslrootcert": {ca}, "connect_timeout": {"10"}}
		u.RawQuery = q.Encode()
		// Ignore inherited connection routing and credentials in production.
		for _, key := range []string{"PGSERVICE", "PGSERVICEFILE", "PGHOSTADDR", "PGOPTIONS", "PGPASSWORD", "PGPASSFILE"} {
			os.Unsetenv(key)
		}
		cfg, err := pgx.ParseConfig(u.String())
		if err != nil {
			return nil, fmt.Errorf("invalid production database configuration or CA")
		}
		cfg.Password = password
		cfg.RuntimeParams["role"] = "defaultdb_migrator"
		return cfg, nil
	}
	database := os.Getenv("PGDATABASE")
	if database != "tireshop_dev" && !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(database) {
		return nil, fmt.Errorf("only local dev/test databases are permitted")
	}
	if os.Getenv("PGHOST") != "postgres" || os.Getenv("PGPORT") != "5432" {
		return nil, fmt.Errorf("only postgres:5432 is permitted")
	}
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("invalid local database configuration")
	}
	return cfg, nil
}
