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
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Order struct {
	ID       string  `json:"id"`
	Total    float64 `json:"total"`
	Customer *User   `json:"customer"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:54360/postgres?sslmode=disable"
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
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("Created table 'users'.")

	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			total NUMERIC(10,2) NOT NULL,
			customer_id TEXT NOT NULL REFERENCES users(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
	`)
	if err != nil {
		log.Fatalf("Failed to create orders table: %v", err)
	}
	fmt.Println("Created table 'orders'.")

	defer func() {
		_, _ = db.Exec(ctx, `DROP TABLE orders;`)
		_, _ = db.Exec(ctx, `DROP TABLE users;`)
		fmt.Println("Dropped tables.")
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

	// conditional upsert
	fmt.Println("\n--- Conditional Upsert (Skipping update since user is banned) ---")
	err = client.Update(ctx, &query.Update{
		Table: "users",
		Data: map[string]any{
			"status": "banned",
		},
		Where: &query.Filter{
			Eq: map[string]any{"email": "alice@example.com"},
		},
	})
	if err != nil {
		log.Fatalf("Failed to update status to banned: %v", err)
	}

	skippedID, err := client.Upsert(ctx, &query.Upsert{
		Into:       "users",
		ConflictOn: []string{"email"},
		Data: map[string]any{
			"email":  "alice@example.com",
			"name":   "Alice Banned",
			"status": "active",
		},
		Where: &query.Filter{
			Neq: map[string]any{"status": "banned"},
		},
	})
	if err != nil {
		log.Fatalf("Conditional Upsert failed: %v", err)
	}
	fmt.Printf("Conditional Upsert returned ID: %q (expected empty because user is banned)\n", skippedID)

	var checkedUser User
	err = client.One(ctx, &query.Get{
		From: "users",
		Where: &query.Filter{
			Eq: map[string]any{"email": "alice@example.com"},
		},
	}, &checkedUser)
	if err != nil {
		log.Fatalf("Verification query failed: %v", err)
	}
	fmt.Printf("Fetched User after skipped Conditional Upsert: %+v\n", checkedUser)

	// reset Alice's status for the join example
	err = client.Update(ctx, &query.Update{
		Table: "users",
		Data:  map[string]any{"status": "active"},
		Where: &query.Filter{Eq: map[string]any{"id": aliceID}},
	})
	if err != nil {
		log.Fatalf("Failed to reset Alice's status: %v", err)
	}

	// insert orders for Alice
	fmt.Println("\n--- Inserting orders for Alice ---")
	order1ID, err := client.Insert(ctx, &query.Insert{
		Into: "orders",
		Data: map[string]any{
			"total":       49.99,
			"customer_id": aliceID,
		},
	})
	if err != nil {
		log.Fatalf("Insert order 1 failed: %v", err)
	}
	_, err = client.Insert(ctx, &query.Insert{
		Into: "orders",
		Data: map[string]any{
			"total":       129.50,
			"customer_id": aliceID,
		},
	})
	if err != nil {
		log.Fatalf("Insert order 2 failed: %v", err)
	}
	fmt.Printf("Inserted 2 orders for Alice (first order ID: %s)\n", order1ID)

	// join with auto-selection: omit Selection on the Join to get all user fields
	fmt.Println("\n--- Fetching order with joined customer (auto-select all user columns) ---")
	var order Order
	err = client.One(ctx, &query.Get{
		From:      "orders",
		Selection: []string{"id", "total"},
		Where:     &query.Filter{Eq: map[string]any{"id": order1ID}},
		Include: []query.Join{
			{
				From:  "users",
				Alias: "customer",
				On:    map[string]string{"customer_id": "id"},
			},
		},
	}, &order)
	if err != nil {
		log.Fatalf("Get order with join failed: %v", err)
	}
	fmt.Printf("Order: id=%s total=%.2f\n", order.ID, order.Total)
	if order.Customer != nil {
		fmt.Printf("Customer: id=%s name=%s email=%s status=%s\n",
			order.Customer.ID, order.Customer.Name, order.Customer.Email, order.Customer.Status)
	}

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
