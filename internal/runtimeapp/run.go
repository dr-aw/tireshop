// Package runtimeapp wires configuration, restricted DB credentials and HTTP.
// It is shared wiring, not another business service or a migration runner.
package runtimeapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"tireshop/internal/httpapi"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run owns the server and pool lifetime. Runtime logins inherit only their
// business role; migrations are never executed by the serving process.
func Run(service string) error {
	if service != "catalog" && service != "requests" {
		return errors.New("unknown service")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database := env("PGDATABASE", "tireshop_dev")
	if database != "tireshop_dev" && !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(database) {
		return errors.New("this development setup only permits local dev/test databases")
	}
	passwordPath := env(strings.ToUpper(service)+"_PASSWORD_FILE", "/run/secrets/"+service+"_password")
	password, err := os.ReadFile(passwordPath)
	if err != nil || len(strings.TrimSpace(string(password))) == 0 {
		return errors.New("runtime password file missing or empty; provision Compose secrets first")
	}

	// Configure a pool
	config, err := pgxpool.ParseConfig("host=postgres port=5432 sslmode=disable")
	if err != nil {
		return errors.New("invalid database configuration")
	}
	config.ConnConfig.Database = database
	config.ConnConfig.User = database + "_" + service + "_login"
	config.ConnConfig.Password = strings.TrimSpace(string(password))
	config.ConnConfig.RuntimeParams = map[string]string{"application_name": "tireshop-" + service}
	config.ConnConfig.ConnectTimeout = 3 * time.Second
	config.MaxConns = 8
	config.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("could not configure database pool")
	}
	defer pool.Close()
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	var current string
	err = pool.QueryRow(checkCtx, "SELECT current_user").Scan(&current)
	cancel()
	if err != nil || current != config.ConnConfig.User {
		return errors.New("runtime database login failed; run db:runtime after mounting secrets")
	}
	var handler http.Handler
	addr := "127.0.0.1:8081"
	if service == "catalog" {
		handler = httpapi.Catalog(pool)
	} else {
		addr = "127.0.0.1:8082"
		handler = httpapi.Requests(pool, strings.Split(env("APP_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), ","))
	}
	server := &http.Server{Addr: env("HTTP_ADDR", addr), Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}

	// buffered to avoid goroutine leak if shutdown is immediate
	finished := make(chan error, 1)

	go func() {
		finished <- server.ListenAndServe()
	}()

	log.Printf("%s listening on %s", service, server.Addr)
	select {
	case err := <-finished:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}
	case <-ctx.Done():
		// Stop accepting connections, let in-flight requests finish, then release
		// the pool. A fresh context allows cleanup after the signal context ends.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return errors.New("HTTP shutdown deadline exceeded")
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
