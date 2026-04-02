package main

import (
	"context"
	"fmt"
	"os"
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

	sql, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading migration: %v\n", err)
		os.Exit(1)
	}

	if _, err := conn.Exec(ctx, string(sql)); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migration complete")
}
