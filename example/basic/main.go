package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/retailancer/pgkit"
	"github.com/retailancer/pgkit/internal/identifier"
	"github.com/retailancer/pgkit/query"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	fmt.Println("Connecting to PostgreSQL...")
	db, err := pgkit.New(ctx, dsn, pgkit.Options{
		MaxConns:         10,
		MinConns:         2,
		IDGenerator:      identifier.NewCUID2Generator(),
		SoftDeleteColumn: "deleted_at",
	})
	if err != nil {
		log.Printf("Could not connect to database (is it running?): %v", err)
		log.Println("Exiting example gracefully since database is offline.")
		return
	}
	defer db.Close()

	client := db.Client()
	defer client.Close()

	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("Created table 'users'.")

	defer func() {
		_, err = db.Exec(ctx, `DROP TABLE users;`)
		if err != nil {
			log.Fatalf("Failed to drop table: %v", err)
		}
		fmt.Println("Dropped table 'users'.")
	}()

	// insert
	fmt.Println("\n--- Inserting user 'Alice' ---")
	aliceID, err := client.Insert(ctx, &query.Insert{
		Into: "users",
		Data: map[string]any{
			"email": "alice@example.com",
			"name":  "Alice Smith",
		},
	})
	if err != nil {
		log.Fatalf("Insert failed: %v", err)
	}
	fmt.Printf("Alice inserted successfully! Generated ID: %s\n", aliceID)

	// fetch single
	fmt.Println("\n--- Fetching user 'Alice' by ID ---")
	var user User
	err = client.One(ctx, &query.Get{
		From:      "users",
		Selection: []string{"id", "email", "name", "created_at"},
		Where: &query.Filter{
			Eq: map[string]any{"id": aliceID},
		},
	}, &user)
	if err != nil {
		log.Fatalf("Get user failed: %v", err)
	}
	fmt.Printf("Fetched User: %+v\n", user)

	// atomic upsert
	fmt.Println("\n--- Upserting user 'Alice' (Updating Name) ---")
	_, err = client.Upsert(ctx, &query.Upsert{
		Into:       "users",
		ConflictOn: []string{"email"},
		Data: map[string]any{
			"email": "alice@example.com",
			"name":  "Alice J. Smith",
		},
	})
	if err != nil {
		log.Fatalf("Upsert failed: %v", err)
	}

	var updatedUser User
	err = client.One(ctx, &query.Get{
		From: "users",
		Where: &query.Filter{
			Eq: map[string]any{"email": "alice@example.com"},
		},
	}, &updatedUser)
	if err != nil {
		log.Fatalf("Verification query failed: %v", err)
	}
	fmt.Printf("Fetched User after Upsert: %+v\n", updatedUser)

	// stateful transaction
	fmt.Println("\n--- Running Stateful Transaction ---")
	err = client.WithTx(ctx, func(tx *pgkit.Tx) error {
		_, err := tx.Insert(ctx, &query.Insert{
			Into: "users",
			Data: map[string]any{
				"email": "bob@example.com",
				"name":  "Bob",
			},
		})
		return err
	})
	if err != nil {
		log.Fatalf("Transaction failed: %v", err)
	}
	fmt.Println("Transaction committed successfully (Bob created).")
}
