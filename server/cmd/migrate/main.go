package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	if len(os.Args) > 1 && os.Args[1] == "reset" {
		drop, _ := os.ReadFile("migrations/000_drop.sql")
		if _, err := conn.Exec(ctx, string(drop)); err != nil {
			fmt.Fprintf(os.Stderr, "drop failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Tables dropped")
	}

	// Track applied migrations
	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT now()
	)`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating migrations table: %v\n", err)
		os.Exit(1)
	}

	// Bootstrap: if tables exist but no migration tracking, mark 001 as applied
	var usersExist bool
	conn.QueryRow(ctx, "SELECT true FROM information_schema.tables WHERE table_name = 'users'").Scan(&usersExist)
	if usersExist {
		conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ('001_init.sql') ON CONFLICT DO NOTHING")
	}

	files, err := filepath.Glob("migrations/[0-9]*.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listing migrations: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(files)

	for _, file := range files {
		name := filepath.Base(file)
		if name == "000_drop.sql" {
			continue
		}

		var exists bool
		err := conn.QueryRow(ctx, "SELECT true FROM schema_migrations WHERE version = $1", name).Scan(&exists)
		if err == nil && exists {
			continue
		}

		sql, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "reading %s: %v\n", name, err)
			os.Exit(1)
		}

		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			fmt.Fprintf(os.Stderr, "applying %s: %v\n", name, err)
			os.Exit(1)
		}

		_, err = conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "recording %s: %v\n", name, err)
			os.Exit(1)
		}

		fmt.Printf("Applied: %s\n", name)
	}

	fmt.Println("Migration complete")
}
