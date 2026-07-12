package pgkit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/retailancer/pgkit/internal/identifier"
	"github.com/retailancer/pgkit/query"
)

func TestClientIDUniquenessWithIgnoreGenerator(t *testing.T) {
	db := &DB{
		opts: Options{
			IDGenerator: identifier.NewIgnoreGenerator(),
		},
		clients: make(map[string]*Client),
	}

	cl1 := db.Client()
	cl2 := db.Client()

	if cl1.id == "" || cl2.id == "" {
		t.Error("expected client IDs to be non-empty strings")
	}

	if cl1.id == cl2.id {
		t.Errorf("expected client IDs to be unique, got matching ID %q", cl1.id)
	}

	if err := cl1.Close(); err != nil {
		t.Errorf("unexpected error closing client 1: %v", err)
	}
	if err := cl2.Close(); err != nil {
		t.Errorf("unexpected error closing client 2: %v", err)
	}
}

func TestSkipCountIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	db, err := New(ctx, dsn, Options{
		IDGenerator: identifier.NewCUID2Generator(),
	})
	if err != nil {
		t.Skipf("skipping integration test, postgres offline: %v", err)
		return
	}
	defer db.Close()

	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_skip_count (
			id TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE test_skip_count;")
	}()

	client := db.Client()
	defer client.Close()

	_, err = client.Insert(ctx, &query.Insert{
		Into: "test_skip_count",
		Data: map[string]any{"value": "row1"},
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	_, err = client.Insert(ctx, &query.Insert{
		Into: "test_skip_count",
		Data: map[string]any{"value": "row2"},
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	type Row struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}

	var rowsWithSkip []Row
	totalWithSkip, err := client.Many(ctx, &query.Get{
		From:      "test_skip_count",
		SkipCount: true,
	}, &rowsWithSkip)
	if err != nil {
		t.Fatalf("Many with SkipCount failed: %v", err)
	}

	if len(rowsWithSkip) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rowsWithSkip))
	}
	if totalWithSkip != 0 {
		t.Errorf("expected count to be 0 (skipped), got %d", totalWithSkip)
	}

	var rowsWithCount []Row
	totalWithCount, err := client.Many(ctx, &query.Get{
		From:      "test_skip_count",
		SkipCount: false,
	}, &rowsWithCount)
	if err != nil {
		t.Fatalf("Many with counting failed: %v", err)
	}

	if len(rowsWithCount) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rowsWithCount))
	}
	if totalWithCount != 2 {
		t.Errorf("expected count to be 2, got %d", totalWithCount)
	}
}
