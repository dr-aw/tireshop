// admin-user is a local provisioning tool, never an HTTP registration endpoint.
package main

import (
	"context"
	"fmt"
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
	if len(os.Args) != 3 || !regexp.MustCompile(`^[a-z0-9_.-]{1,80}$`).MatchString(os.Args[1]) {
		return fmt.Errorf("usage: admin-user USERNAME PASSWORD_FILE (12–1024 bytes, file mode 0600)")
	}
	info, err := os.Stat(os.Args[2])
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("password file must be a private regular file (0600)")
	}
	raw, err := os.ReadFile(os.Args[2])
	if err != nil {
		return fmt.Errorf("cannot read password file")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if len(password) < 12 || len(password) > 1024 {
		return fmt.Errorf("password must contain 12–1024 bytes")
	}
	database := os.Getenv("PGDATABASE")
	if database != "tireshop_dev" && !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(database) {
		return fmt.Errorf("only local dev/test databases are permitted")
	}
	if os.Getenv("PGHOST") != "postgres" || os.Getenv("PGPORT") != "5432" {
		return fmt.Errorf("only postgres:5432 is permitted")
	}
	hash := adminauth.Hash(password)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, "")
	if err != nil {
		return fmt.Errorf("database connection failed")
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction failed")
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "admin:"+os.Args[1]); err != nil {
		return fmt.Errorf("account lock failed")
	}
	_, err = tx.Exec(ctx, `INSERT INTO requests.admin_users(username,password_hash) VALUES($1,$2)
 ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash,active=true`, os.Args[1], hash)
	if err != nil {
		return fmt.Errorf("account update failed; apply migrations first")
	}
	_, err = tx.Exec(ctx, `DELETE FROM requests.admin_sessions WHERE username=$1`, os.Args[1])
	if err != nil {
		return fmt.Errorf("session revocation failed")
	}
	if tx.Commit(ctx) != nil {
		return fmt.Errorf("commit could not be confirmed")
	}
	fmt.Println("Account saved; previous sessions revoked.")
	return nil
}
