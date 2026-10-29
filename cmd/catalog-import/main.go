// catalog-import is a bounded standalone catalog worker, never part of HTTP.
package main

import (
	"context"
	"encoding/json/v2"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"regexp"
	"strings"
	"time"
	"tireshop/internal/catalogsource"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	source := flag.String("source", "", "JSON produced by catalog-audit")
	mediaFile := flag.String("media", "", "validated media manifest")
	publish := flag.Bool("publish", false, "publish to local dev database")
	flag.Parse()
	raw, err := os.ReadFile(*source)
	if err != nil {
		return err
	}
	var books []catalogsource.Workbook
	if err = json.Unmarshal(raw, &books); err != nil {
		return err
	}
	batch, err := catalogsource.Normalize(books)
	if err != nil {
		return err
	}
	if !*publish {
		b, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(b, '\n'))
		return err
	}
	if *mediaFile == "" {
		return fmt.Errorf("media manifest required; prepare and validate before publication")
	}
	raw, err = os.ReadFile(*mediaFile)
	if err != nil {
		return err
	}
	var media []catalogsource.Media
	if err = json.Unmarshal(raw, &media); err != nil {
		return err
	}
	database := os.Getenv("PGDATABASE")
	if database == "" {
		database = "tireshop_dev"
	}
	if database != "tireshop_dev" && !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(database) {
		return fmt.Errorf("only local dev/test permitted")
	}
	passwordFile := os.Getenv("CATALOG_PASSWORD_FILE")
	if passwordFile == "" {
		passwordFile = "/run/secrets/catalog_password"
	}
	password, err := os.ReadFile(passwordFile)
	if err != nil {
		return fmt.Errorf("catalog credential unavailable")
	}
	config, err := pgxpool.ParseConfig("host=postgres port=5432 sslmode=disable")
	if err != nil {
		return err
	}
	config.ConnConfig.Database = database
	config.ConnConfig.User = database + "_catalog_login"
	config.ConnConfig.Password = strings.TrimSpace(string(password))
	config.MaxConns = 1
	config.ConnConfig.ConnectTimeout = 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("pool setup failed")
	}
	defer pool.Close()
	changed, err := catalogsource.Publish(ctx, pool, batch, media)
	if err != nil {
		return err
	}
	fmt.Printf("catalog import: published=%t products=%d images=%d\n", changed, len(batch.Products), len(media))
	return nil
}
