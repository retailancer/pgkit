# pgkit

[![Go Reference](https://pkg.go.dev/badge/github.com/retailancer/pgkit.svg)](https://pkg.go.dev/github.com/retailancer/pgkit)

`pgkit` is a PostgreSQL toolkit for Go, built natively on top of [`pgx/v5`](https://github.com/jackc/pgx). It provides a type-safe, ergonomic query builder, automatic JSON/JSONB scanning, savepoint-based nested transactions, and a resilient self-healing `LISTEN/NOTIFY` client, without an ORM or code generation.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration & Options](#configuration--options)
- [Client & Lifecycle](#client--lifecycle)
- [Reading Data](#reading-data)
  - [One — fetch single record](#one--fetch-single-record)
  - [Many — fetch list with pagination](#many--fetch-list-with-pagination)
  - [Count — count matching records](#count--count-matching-records)
  - [Exec — generic query runner](#exec--generic-query-runner)
- [Writing Data](#writing-data)
  - [Insert](#insert)
  - [InsertMany — batch insert](#insertmany--batch-insert)
  - [Upsert](#upsert)
  - [Update](#update)
  - [Delete](#delete)
  - [Soft Delete](#soft-delete)
- [Filtering — query.Filter](#filtering--queryfilter)
  - [Operators](#operators)
  - [Logical operators (Op)](#logical-operators-op)
  - [Filter Groups — nested logic](#filter-groups--nested-logic)
- [Joins & Relations](#joins--relations)
  - [One-to-One join](#one-to-one-join)
  - [One-to-Many join](#one-to-many-join)
  - [Join-level filters & ordering](#join-level-filters--ordering)
- [Aggregates](#aggregates)
- [Advanced Get Options](#advanced-get-options)
  - [Ordering](#ordering)
  - [Pagination (Limit & Offset)](#pagination-limit--offset)
  - [Group By](#group-by)
  - [Distinct On](#distinct-on)
  - [Shuffle (random order)](#shuffle-random-order)
  - [FOR UPDATE locking](#for-update-locking)
  - [Include deleted rows](#include-deleted-rows)
  - [Query logging](#query-logging)
- [Type Casting](#type-casting)
- [Transactions](#transactions)
  - [WithTx — automatic commit/rollback](#withtx--automatic-commitrollback)
  - [Manual transaction control](#manual-transaction-control)
  - [Nested transactions (Savepoints)](#nested-transactions-savepoints)
- [LISTEN / NOTIFY](#listen--notify)
- [Raw SQL Escape Hatch](#raw-sql-escape-hatch)
- [ID Generation](#id-generation)
- [Errors](#errors)
- [Result Scanning](#result-scanning)

---

## Installation

```bash
go get github.com/retailancer/pgkit
```

Requires Go 1.21+ and PostgreSQL 13+.

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/retailancer/pgkit"
    "github.com/retailancer/pgkit/query"
)

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    ctx := context.Background()

    db, err := pgkit.New(ctx, "postgres://user:pass@localhost:5432/mydb", pgkit.Options{
        SoftDeleteColumn: "deleted_at",
        AutoUpdatedAt:    true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    client := db.Client()
    defer client.Close()

    id, err := client.Insert(ctx, &query.Insert{
        Into: "users",
        Data: map[string]any{"name": "Alice", "email": "alice@example.com"},
    })

    var user User
    err = client.One(ctx, &query.Get{
        From:  "users",
        Where: &query.Filter{Eq: map[string]any{"id": id}},
    }, &user)

    fmt.Printf("User: %+v\n", user)
}
```

---

## Configuration & Options

```go
db, err := pgkit.New(ctx, dsn, pgkit.Options{
    // Connection pool
    MaxConns:          25,
    MinConns:          2,
    MaxConnLifetime:   5 * time.Minute,
    MaxConnIdleTime:   10 * time.Minute,
    HealthCheckPeriod: 30 * time.Second,

    // Database conventions
    Schema:           "public",        // default: "public"
    SoftDeleteColumn: "deleted_at",    // enables soft-delete support globally
    AutoUpdatedAt:    true,            // auto-sets updated_at on writes

    // Observability
    Logger: myLogger, // implements pgkit.Logger interface

    // ID generation
    IDGenerator: myIDGen, // implements identifier.Generator; default: IgnoreGenerator
})
```

**`pgkit.Logger` interface:**

```go
type Logger interface {
    Info(msg string, args ...any)
    Error(msg string, args ...any)
}
```

---

## Client & Lifecycle

A `Client` is the primary handle for executing queries. Obtain one per request/goroutine.

```go
client := db.Client()
defer client.Close() // rolls back any uncommitted tx and releases the client
```

`client.Close()` is always safe to call. `client.CloseSilently()` is a convenience wrapper that discards the error.

---

## Reading Data

### One — fetch single record

Fetches exactly one row and scans it into `dest`. Returns `pgkit.ErrNotFound` if no rows match, and `pgkit.ErrMultipleRecords` if more than one row is returned.

```go
var user User
err := client.One(ctx, &query.Get{
    From:  "users",
    Where: &query.Filter{Eq: map[string]any{"id": "abc123"}},
}, &user)
```

### Many — fetch list with pagination

Fetches all matching rows and returns the total record count (ignoring limit/offset) for pagination. Set `SkipCount: true` to bypass the total record count query for optimal performance when counting is not needed.

```go
var users []User
total, err := client.Many(ctx, &query.Get{
    From:      "users",
    Order:     map[string]string{"created_at": "DESC"},
    Limit:     20,
    Offset:    0,
    SkipCount: true, // Optional: bypasses the COUNT(*) query for optimal performance
}, &users)

fmt.Printf("page has %d users, total is %d\n", len(users), total)
```

### Count — count matching records

Counts records matching the filter without fetching rows.

```go
count, err := client.Count(ctx, &query.Get{
    From:  "users",
    Where: &query.Filter{Eq: map[string]any{"status": "active"}},
})
```

### Exec — generic query runner

Runs any `query.Query` type and returns a raw `*query.Result`.

```go
result, err := client.Exec(ctx, &query.Get{From: "users"})
```

---

## Writing Data

### Insert

Inserts a single row. The `id` field is auto-generated (CUID2) if not provided. Returns the inserted record's ID.

```go
id, err := client.Insert(ctx, &query.Insert{
    Into: "products",
    Data: map[string]any{
        "name":  "Widget",
        "price": 9.99,
    },
})
```

To supply your own ID, include `"id"` in `Data`:

```go
id, err := client.Insert(ctx, &query.Insert{
    Into: "products",
    Data: map[string]any{"id": "my-custom-id", "name": "Widget"},
})
```

`SetUpdatedAt` overrides the global `AutoUpdatedAt` setting for this query only:

```go
&query.Insert{
    Into:         "events",
    Data:         map[string]any{"name": "signup"},
    SetUpdatedAt: pgkit.Bool(false), // suppress updated_at for this insert
}
```

### InsertMany — batch insert

Inserts multiple rows in a single round trip. IDs are auto-generated per row. Returns a slice of generated IDs.

```go
ids, err := client.InsertMany(ctx, &query.InsertMany{
    Into:   "tags",
    Fields: []string{"name", "color"},
    Values: [][]any{
        {"Go",     "#00ADD8"},
        {"Rust",   "#CE422B"},
        {"Python", "#3776AB"},
    },
})
```

If `"id"` is listed in `Fields`, you may include it per row. Omit it from a row's values to auto-generate.

### Upsert

Atomically inserts or updates via `INSERT ... ON CONFLICT DO UPDATE`. All fields in `ConflictOn` must correspond to a unique constraint. The `id` field and conflict columns are never overwritten on conflict.

```go
id, err := client.Upsert(ctx, &query.Upsert{
    Into:       "users",
    ConflictOn: []string{"email"},
    Data: map[string]any{
        "id":    "abc123",
        "email": "alice@example.com",
        "name":  "Alice Updated",
    },
})
```

**Composite unique constraint:**

```go
ConflictOn: []string{"user_id", "organization_id"},
```

### Update

Updates rows matching the filter. Returns `pgkit.ErrNotFound` if no rows were affected. Requires at least one field in `Data`.

```go
err := client.Update(ctx, &query.Update{
    Table: "users",
    Data:  map[string]any{"name": "Bob"},
    Where: &query.Filter{Eq: map[string]any{"id": "abc123"}},
})
```

### Delete

Hard deletes rows matching the filter:

```go
err := client.Delete(ctx, &query.Delete{
    From:  "users",
    Where: &query.Filter{Eq: map[string]any{"id": "abc123"}},
    Soft:  false,
})
```

### Soft Delete

Sets `deleted_at` to the current timestamp instead of removing the row. Requires `SoftDeleteColumn` to be configured in `Options`.

```go
err := client.Delete(ctx, &query.Delete{
    From:  "users",
    Where: &query.Filter{Eq: map[string]any{"id": "abc123"}},
    Soft:  true,
})
```

By default, all `Get` and `Aggregate` queries automatically exclude soft-deleted rows (`WHERE deleted_at IS NULL`). See [Include deleted rows](#include-deleted-rows) to override.

---

## Filtering — query.Filter

`query.Filter` builds the `WHERE` clause. All map keys are sorted alphabetically at build time for deterministic SQL and prepared-statement reuse.

### Operators

```go
&query.Filter{
    Eq:        map[string]any{"status": "active"},          // col = $1
    Neq:       map[string]any{"role": "guest"},             // col != $1
    Gt:        map[string]any{"age": 18},                   // col > $1
    Gte:       map[string]any{"score": 100},                // col >= $1
    Lt:        map[string]any{"attempts": 5},               // col < $1
    Lte:       map[string]any{"price": 99.99},              // col <= $1
    In:        map[string][]any{"id": {"a", "b", "c"}},    // col IN ($1,$2,$3)
    NotIn:     map[string][]any{"status": {"banned"}},      // col NOT IN ($1)
    Like:      map[string]string{"name": "%Alice%"},        // col LIKE $1
    ILike:     map[string]string{"name": "%alice%"},        // col ILIKE $1
    Regexp:    map[string]string{"email": "@example\\.com$"}, // col ~* $1
    IsNull:    []string{"deleted_at"},                      // col IS NULL
    IsNotNull: []string{"verified_at"},                     // col IS NOT NULL
}
```

**Notes:**

- `Eq` with a `nil` value generates `col IS NULL`.
- `Neq` with a `nil` value generates `col IS NOT NULL`.
- `In` with an empty slice generates `FALSE` (safe no-op preventing invalid SQL).
- `NotIn` with an empty slice generates `TRUE`.
- `Like` uses case-sensitive `LIKE`.
- `ILike` uses case-insensitive `ILIKE`.
- `Regexp` uses PostgreSQL's case-insensitive `~*` operator.

### Logical operators (Op)

By default, all conditions in a single `Filter` are joined with `AND`. Use `Op` to switch to `OR`:

```go
&query.Filter{
    Op:    query.Or,
    ILike: map[string]string{"name": "%alice%", "email": "%alice%"},
}
// → name ILIKE $1 OR email ILIKE $2
```

Constants: `query.And`, `query.Or`.

### Filter Groups — nested logic

`Groups` allows composing arbitrarily nested AND/OR logic. Each group is wrapped in parentheses and appended to the parent filter's conditions.

```go
// WHERE organization_id = $1 AND (email = $2 OR name = $3)
&query.Filter{
    Op: query.And,
    Eq: map[string]any{"organization_id": orgID},
    Groups: []query.FilterGroup{
        {
            Name: "search",
            Filter: &query.Filter{
                Op:   query.Or,
                Like: map[string]string{
                    "email": "%alice%",
                    "name":  "%alice%",
                },
            },
        },
    },
}
```

Groups can be nested recursively to any depth. The parent filter's `Op` controls how the group result is joined to the rest of the conditions.

---

## Joins & Relations

Joins are declared via `Include []query.Join` on a `Get` query. By default, joins are compiled as `LEFT JOIN`, but you can specify custom join types using the `Type` field (e.g., `query.InnerJoin`, `query.RightJoin`, `query.FullJoin`).

### One-to-One join

```go
var order OrderWithCustomer

err := client.One(ctx, &query.Get{
    From:      "orders",
    Selection: []string{"id", "total", "status"},
    Where:     &query.Filter{Eq: map[string]any{"id": orderID}},
    Include: []query.Join{
        {
            Type:      query.InnerJoin, // Optional: defaults to LeftJoin
            From:      "users",
            Alias:     "customer",
            Selection: []string{"name", "email"},
            On:        map[string]string{"customer_id": "id"},
        },
    },
}, &order)
```

The `On` map keys are columns on the **parent** table (or `"parentTable.col"` for explicit qualification), and values are columns on the **joined** table's alias. Joined columns are returned as `alias__column` and automatically nested into `{"customer": {"name": "...", "email": "..."}}` by the row scanner.

At least one `On` condition is required or the build will error.

### One-to-Many join

Set `Many: true` on a join to aggregate child rows into a slice per parent row.

```go
type PostWithComments struct {
    ID       string    `json:"id"`
    Title    string    `json:"title"`
    Comments []Comment `json:"comments"`
}

var posts []PostWithComments
total, err := client.Many(ctx, &query.Get{
    From: "posts",
    Include: []query.Join{
        {
            From:  "comments",
            Alias: "comments",
            On:    map[string]string{"id": "post_id"},
            Many:  true,
        },
    },
}, &posts)
```

When `Many: true`, duplicate parent rows from the join are deduplicated and children are aggregated by matching `id`.

### Join-level filters & ordering

Each join supports its own `Where`, `Order`, and `GroupBy`:

```go
Include: []query.Join{
    {
        From:  "comments",
        Alias: "comments",
        On:    map[string]string{"id": "post_id"},
        Many:  true,
        Where: &query.Filter{
            Eq: map[string]any{"approved": true},
        },
        Order: map[string]string{"created_at": "ASC"},
    },
},
```

---

## Aggregates

`query.Aggregate` computes aggregate functions across rows with optional grouping and joins.

```go
type Stats struct {
    Category    string  `json:"category"`
    PriceAvg    float64 `json:"price__avg"`
    PriceMax    float64 `json:"price__max"`
    PriceMin    float64 `json:"price__min"`
    TotalSum    float64 `json:"total__sum"`
    OrdersCount float64 `json:"id__count"`
}

result, err := client.Exec(ctx, &query.Aggregate{
    From:    "orders",
    Fields:  []string{"category"},          // plain SELECT columns
    Avg:     []string{"price"},             // → COALESCE(AVG(price), 0)::float AS price__avg
    Max:     []string{"price"},             // → COALESCE(MAX(price), 0)::float AS price__max
    Min:     []string{"price"},             // → COALESCE(MIN(price), 0)::float AS price__min
    Sum:     []string{"total"},             // → COALESCE(SUM(total), 0)::float AS total__sum
    Count:   []string{"id"},               // → COALESCE(COUNT(id), 0)::float AS id__count
    GroupBy: []string{"category"},
    Order:   map[string]string{"category": "ASC"},
    Where:   &query.Filter{Eq: map[string]any{"status": "completed"}},
})

var stats []Stats
err = result.Scan(&stats)
```

`Aggregate` also supports `Include` joins with the same `Join` struct, `Limit`, `Offset`, `IncludeDeleted`, and `Log`.

When a joined table has no `Selection`, it automatically computes `COUNT(alias.id)::float AS alias__count`.

---

## Advanced Get Options

### Ordering

```go
Order: map[string]string{
    "created_at": "DESC",
    "name":       "ASC",
},
```

### Pagination (Limit & Offset)

```go
Limit:  25,
Offset: 50, // page 3
```

### Group By

```go
GroupBy: []string{"status", "role"},
```

### Distinct On

Selects only the first row from each group of `DISTINCT ON` expressions. The leftmost `ORDER BY` key should match `DistinctOn` per PostgreSQL rules.

```go
DistinctOn: []string{"user_id"},
Order:       map[string]string{"user_id": "ASC", "created_at": "DESC"},
```

### Shuffle (random order)

Produces a daily-stable pseudo-random order, useful for discovery feeds. Uses `md5(col || 'YYYY-MM-DD-HH')` as the sort key.

```go
ShuffleOn: "id", // randomised ORDER BY seeded on the current hour
```

`ShuffleOn` takes precedence over `Order`. Both cannot be used together.

### FOR UPDATE locking

Locks selected rows for the duration of the enclosing transaction, preventing concurrent updates.

```go
ForUpdate: true,
```

### Include deleted rows

Override the global soft-delete filter for a specific query. By default all queries with a `SoftDeleteColumn` configured will exclude soft-deleted rows.

```go
IncludeDeleted: pgkit.Bool(true),  // include soft-deleted rows
IncludeDeleted: pgkit.Bool(false), // force-exclude even if global default changes
IncludeDeleted: nil,               // inherit global default (default behaviour)
```

`pgkit.Bool(v bool) *bool` is a convenience helper.

### Query logging

Log the interpolated SQL for a specific query to the configured `Logger`:

```go
Log: true,
```

---

## Type Casting

All write query types (`Insert`, `InsertMany`, `Upsert`, `Update`) and `Filter` accept a `Types map[string]string` for appending PostgreSQL type casts to parameter placeholders.

```go
&query.Insert{
    Into: "events",
    Data: map[string]any{
        "metadata": `{"key":"value"}`,
        "tags":     []string{"a", "b"},
    },
    Types: map[string]string{
        "metadata": "jsonb",
        "tags":     "text[]",
    },
}
// → INSERT INTO ... VALUES ($1::jsonb, $2::text[])
```

The same `Types` map works on `Filter` conditions:

```go
&query.Get{
    From:  "events",
    Where: &query.Filter{Eq: map[string]any{"status": "active"}},
    Types: map[string]string{"status": "text"},
}
// → WHERE "status" = $1::text
```

---

## Transactions

### WithTx — automatic commit/rollback

The cleanest pattern. Commits on success, rolls back automatically if the function returns an error.

**On `DB` (creates its own client internally):**

```go
err := db.WithTx(ctx, func(tx *pgkit.Tx) error {
    id, err := tx.Insert(ctx, &query.Insert{
        Into: "orders",
        Data: map[string]any{"user_id": userID, "total": 99.99},
    })
    if err != nil {
        return err
    }
    return tx.Update(ctx, &query.Update{
        Table: "users",
        Data:  map[string]any{"last_order_id": id},
        Where: &query.Filter{Eq: map[string]any{"id": userID}},
    })
})
```

**On an existing `Client` (shares the client's lifecycle):**

```go
err := client.WithTx(ctx, func(tx *pgkit.Tx) error {
    // ...
})
```

`Tx` exposes the same methods as `Client`: `One`, `Many`, `Count`, `Insert`, `InsertMany`, `Upsert`, `Update`, `Delete`, `Exec`.

### Manual transaction control

For cases where you need explicit control:

```go
client := db.Client()
defer client.Close()

if err := client.StartTx(ctx); err != nil {
    return err
}

_, err := client.Insert(ctx, /* ... */)
if err != nil {
    _ = client.RollbackTx(ctx)
    return err
}

return client.CommitTx(ctx)
```

### Nested transactions (Savepoints)

`Tx.WithTx` creates a child transaction using a PostgreSQL `SAVEPOINT`. Rollback on the child rolls back to the savepoint without affecting the outer transaction.

```go
err := db.WithTx(ctx, func(tx *pgkit.Tx) error {
    // outer transaction work...
    _, err := tx.Insert(ctx, &query.Insert{Into: "orders", Data: orderData})
    if err != nil {
        return err
    }

    // nested transaction — uses SAVEPOINT
    nestedErr := tx.WithTx(ctx, func(inner *pgkit.Tx) error {
        return inner.Insert(ctx, &query.Insert{Into: "audit_log", Data: logData})
    })
    if nestedErr != nil {
        // only the nested savepoint is rolled back, outer tx continues
        log.Println("audit log failed, continuing:", nestedErr)
    }

    return nil
})
```

Nesting depth is tracked automatically; savepoint names are `pgkit_sp_1`, `pgkit_sp_2`, etc.

---

## LISTEN / NOTIFY

`pgkit` provides a resilient, self-healing listener that automatically reconnects and re-subscribes on connection loss.

```go
listener, err := db.Listen(ctx, "order_created")
if err != nil {
    log.Fatal(err)
}
defer listener.Close()

for {
    select {
    case n, ok := <-listener.C():
        if !ok {
            return // listener was closed
        }
        fmt.Printf("channel=%s payload=%s\n", n.Channel, n.Payload)
    case <-ctx.Done():
        return
    }
}
```

`listener.C()` returns a `<-chan pgkit.Notification`. The internal loop:

- Blocks on `WaitForNotification` with a cancellable context.
- On connection drop, closes the old connection and reconnects with exponential backoff.
- On `Close()`, cancels the context, drains the connection, and closes the channel cleanly.

```go
type Notification struct {
    Channel string
    Payload string
}
```

---

## Raw SQL Escape Hatch

For queries that fall outside the builder's scope, you can execute raw SQL.

### Execute Raw SQL on Client/Transaction

To execute raw SQL queries that participate in client transactions and nested savepoints, use `client.Query`. It returns a raw `pgx.Rows` result:

```go
rows, err := client.Query(ctx, "SELECT id, name FROM users WHERE age > $1", 18)
if err != nil {
    return err
}
defer rows.Close()
for rows.Next() {
    // scan rows using pgx.Rows standard scanning
}
```

### Execute Raw DDL/SQL directly on the Pool

To run schema migrations or DDL statements directly on the database pool (bypassing any transaction state):

```go
tag, err := db.Exec(ctx, `
    CREATE TABLE IF NOT EXISTS sessions (
        id TEXT PRIMARY KEY,
        user_id TEXT NOT NULL,
        expires_at TIMESTAMPTZ NOT NULL
    )
`)
```

This runs directly against the `pgxpool.Pool` and bypasses the query builder entirely.

---

## ID Generation

By default, `pgkit` delegates ID generation entirely to the database (e.g., `SERIAL`, `BIGSERIAL`, `IDENTITY`, or database-level defaults). It does not inject an `"id"` column on the client side, allowing the database to assign it natively.

You can configure client-side ID generation or plug in a custom generator via the `IDGenerator` option:

### 1. Database-Side ID Generation (Default)

Rely on database-side sequences, identity columns, or defaults. No client-side values are injected:

```go
db, err := pgkit.New(ctx, dsn, pgkit.Options{
    IDGenerator: identifier.NewIgnoreGenerator(), // default: ignore client-side ID generation
})
```

### 2. Client-Side CUID2 Generation

Generate CUID2 identifiers on the client side before inserting records:

```go
db, err := pgkit.New(ctx, dsn, pgkit.Options{
    IDGenerator: identifier.NewCUID2Generator(),
})
```

### 3. Custom ID Generator

Plug in any generator by implementing the `identifier.Generator` interface:

```go
// internal/identifier/id.go
type Generator interface {
    Generate() string
}
```

Example — UUID v4:

```go
import "github.com/google/uuid"

type uuidGen struct{}
func (u uuidGen) Generate() string { return uuid.NewString() }

db, err := pgkit.New(ctx, dsn, pgkit.Options{
    IDGenerator: uuidGen{},
})
```

---

## Errors

All sentinel errors can be tested with `errors.Is`:

| Error                          | When returned                                |
| ------------------------------ | -------------------------------------------- |
| `pgkit.ErrNotFound`            | `One` finds no matching rows                 |
| `pgkit.ErrMultipleRecords`     | `One` finds more than one row                |
| `pgkit.ErrUniqueViolation`     | PostgreSQL code `23505`                      |
| `pgkit.ErrForeignKeyViolation` | PostgreSQL code `23503`                      |
| `pgkit.ErrCheckViolation`      | PostgreSQL code `23514`                      |
| `pgkit.ErrNullViolation`       | PostgreSQL code `23502`                      |
| `pgkit.ErrTxClosed`            | Operation on a committed/rolled-back `Tx`    |
| `pgkit.ErrTxAlreadyStarted`    | `StartTx` called when a tx is already active |
| `pgkit.ErrConnectionFailed`    | Initial pool connection fails                |

`MapError` wraps `*pgconn.PgError` using `%w` chaining, so both the sentinel **and** the original database error are inspectable:

```go
if errors.Is(err, pgkit.ErrUniqueViolation) {
    // high-level check
}

var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
    fmt.Println("constraint:", pgErr.ConstraintName)
}
```

`pgkit.IsPgErrorCode(err, "23505")` checks for a specific PostgreSQL error code directly.

---

## Result Scanning

`client.One` and `client.Many` scan results directly via JSON round-trip mapping. For raw `*query.Result` from `client.Exec`, use:

| Method                                      | Purpose                                                     |
| ------------------------------------------- | ----------------------------------------------------------- |
| `result.Scan(dest)`                         | Scan entire result into dest (struct or slice)              |
| `result.ScanAt(dest, index)`                | Scan row at index from a multi-row result                   |
| `result.ScanIncluded(dest, alias)`          | Scan a joined relation from a single-row result             |
| `result.ScanIncludedAt(dest, index, alias)` | Scan a joined relation at row index from a multi-row result |
| `result.LastInsertID()`                     | Get the string ID from an Insert result                     |
| `result.LastInsertIDs()`                    | Get all string IDs from an InsertMany result                |
| `result.Total`                              | Total count from a Many query (ignoring limit/offset)       |

`dest` must always be a non-nil pointer. For collections it must be a pointer to a slice.

---

## License

MIT © Retailancer.
