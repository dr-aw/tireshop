package runtimeapp

import (
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Production is explicit. Local tooling retains its independent local-only guards.
func databaseConfig(service string) (*pgxpool.Config, error) {
	if service != "catalog" && service != "requests" {
		return nil, errors.New("unknown service")
	}
	mode := env("APP_ENV", "development")
	database := env("PGDATABASE", "tireshop_dev")
	host, port, user := "postgres", "5432", database+"_"+service+"_login"
	q := url.Values{"sslmode": {"disable"}}
	maxConns := 8
	switch mode {
	case "development":
		if database != "tireshop_dev" && !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(database) {
			return nil, errors.New("development only permits local dev/test databases")
		}
	case "production":
		host, port, database, user = os.Getenv("PGHOST"), os.Getenv("PGPORT"), os.Getenv("PGDATABASE"), os.Getenv("PGUSER")
		n, err := strconv.Atoi(port)
		if host == "" || !regexp.MustCompile(`^[a-zA-Z0-9.-]+$`).MatchString(host) || net.ParseIP(host) != nil || err != nil || n < 1 || n > 65535 || database == "" || user == "" {
			return nil, errors.New("production requires explicit DNS host, port, database and runtime user")
		}
		if user == "avnadmin" || user == "postgres" {
			return nil, errors.New("production requires a restricted runtime login")
		}
		if os.Getenv("ADMIN_LOCAL_HTTP") == "1" {
			return nil, errors.New("production requires secure admin cookies")
		}
		if service == "requests" {
			origins := strings.Split(os.Getenv("APP_ORIGINS"), ",")
			for _, origin := range origins {
				u, e := url.Parse(origin)
				if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
					return nil, errors.New("production requires explicit HTTPS origins")
				}
			}
		}
		ca := os.Getenv("PGSSLROOTCERT")
		if ca == "" {
			return nil, errors.New("production requires a CA certificate file")
		}
		q.Set("sslmode", "verify-full")
		q.Set("sslrootcert", ca)
		maxConns = 4
		if value := os.Getenv("PGPOOL_MAX_CONNS"); value != "" {
			maxConns, err = strconv.Atoi(value)
			if err != nil || maxConns < 1 || maxConns > 16 {
				return nil, errors.New("invalid production pool limit (1-16)")
			}
		}
	default:
		return nil, errors.New("APP_ENV must be development or production")
	}
	// Explicit URI fields override inherited PGHOST/PGUSER/PGSSLMODE settings.
	u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port), User: url.User(user), Path: "/" + database, RawQuery: q.Encode()}
	config, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		return nil, errors.New("invalid database or TLS configuration")
	}
	passwordPath := env(strings.ToUpper(service)+"_PASSWORD_FILE", "/run/secrets/"+service+"_password")
	password, err := os.ReadFile(passwordPath)
	if err != nil || len(strings.TrimSpace(string(password))) == 0 {
		return nil, errors.New("runtime password file missing or empty")
	}
	config.ConnConfig.Password = strings.TrimSpace(string(password))
	config.ConnConfig.RuntimeParams = map[string]string{"application_name": "tireshop-" + service}
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	config.MaxConns = int32(maxConns)
	config.MaxConnIdleTime = 5 * time.Minute
	return config, nil
}
