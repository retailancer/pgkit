package builder

import (
	"testing"

	"github.com/retailancer/pgkit/internal/sqlutil"
	"github.com/retailancer/pgkit/query"
)

func TestBuildInsertDeterministic(t *testing.T) {
	q := &query.Insert{
		Into: "users",
		Data: map[string]any{
			"email": "test@example.com",
			"name":  "Test User",
			"age":   30,
		},
		Types: map[string]string{
			"age": "integer",
		},
	}

	var expectedSQL string
	for i := 0; i < 20; i++ {
		pt := &ParamTracker{}
		sqlStr, err := Build(q, pt, "public", "", false)
		if err != nil {
			t.Fatalf("failed to build SQL: %v", err)
		}

		if expectedSQL == "" {
			expectedSQL = sqlStr
			expectedPattern := `INSERT INTO "public"."users" ("age", "email", "name") VALUES ($1::integer, $2, $3) RETURNING *`
			if sqlStr != expectedPattern {
				t.Errorf("unexpected SQL string: got %q, want %q", sqlStr, expectedPattern)
			}
			if len(pt.Params) != 3 {
				t.Errorf("unexpected parameters length: got %d, want 3", len(pt.Params))
			}
		} else if sqlStr != expectedSQL {
			t.Errorf("non-deterministic SQL generated at iteration %d:\nrun 1: %s\nrun 2: %s", i, expectedSQL, sqlStr)
		}
	}
}

func TestBuildGetWithJoins(t *testing.T) {
	q := &query.Get{
		From:      "orders",
		Selection: []string{"id", "total"},
		Where: &query.Filter{
			Eq: map[string]any{"status": "pending"},
		},
		Include: []query.Join{
			{
				From:  "users",
				Alias: "customer",
				On: map[string]string{
					"customer_id": "id",
				},
				Selection: []string{"name", "email"},
			},
		},
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "deleted_at", false)
	if err != nil {
		t.Fatalf("failed to build SELECT: %v", err)
	}

	expectedSQL := `SELECT "orders"."id", "orders"."total", "customer"."name" AS "customer__name", "customer"."email" AS "customer__email" FROM "public"."orders" LEFT JOIN "public"."users" AS "customer" ON "orders"."customer_id" = "customer"."id" WHERE "public"."orders"."deleted_at" IS NULL AND ("orders"."status" = $1)`
	if sqlStr != expectedSQL {
		t.Errorf("unexpected SQL output:\ngot:  %s\nwant: %s", sqlStr, expectedSQL)
	}
}

func TestBuildGetWithInnerJoin(t *testing.T) {
	q := &query.Get{
		From:      "orders",
		Selection: []string{"id", "total"},
		Include: []query.Join{
			{
				Type:  query.InnerJoin,
				From:  "users",
				Alias: "customer",
				On: map[string]string{
					"customer_id": "id",
				},
				Selection: []string{"name"},
			},
		},
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "", false)
	if err != nil {
		t.Fatalf("failed to build SELECT: %v", err)
	}

	expectedSQL := `SELECT "orders"."id", "orders"."total", "customer"."name" AS "customer__name" FROM "public"."orders" INNER JOIN "public"."users" AS "customer" ON "orders"."customer_id" = "customer"."id"`
	if sqlStr != expectedSQL {
		t.Errorf("unexpected SQL output:\ngot:  %s\nwant: %s", sqlStr, expectedSQL)
	}
}

func TestBuildUpsert(t *testing.T) {
	q := &query.Upsert{
		Into:       "users",
		ConflictOn: []string{"email"},
		Data: map[string]any{
			"id":    "123",
			"email": "test@example.com",
			"name":  "Alice",
		},
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "", false)
	if err != nil {
		t.Fatalf("failed to build UPSERT: %v", err)
	}

	expectedSQL := `INSERT INTO "public"."users" ("email", "id", "name") VALUES ($1, $2, $3) ON CONFLICT ("email") DO UPDATE SET "name" = EXCLUDED."name" RETURNING *`
	if sqlStr != expectedSQL {
		t.Errorf("unexpected SQL output:\ngot:  %s\nwant: %s", sqlStr, expectedSQL)
	}
}

func TestBuildUpsertWithWhere(t *testing.T) {
	q := &query.Upsert{
		Into:       "users",
		ConflictOn: []string{"email"},
		Data: map[string]any{
			"id":    "123",
			"email": "test@example.com",
			"name":  "Alice",
		},
		Where: &query.Filter{
			Neq: map[string]any{"status": "banned"},
		},
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "", false)
	if err != nil {
		t.Fatalf("failed to build UPSERT: %v", err)
	}

	expectedSQL := `INSERT INTO "public"."users" ("email", "id", "name") VALUES ($1, $2, $3) ON CONFLICT ("email") DO UPDATE SET "name" = EXCLUDED."name" WHERE "users"."status" != $4 RETURNING *`
	if sqlStr != expectedSQL {
		t.Errorf("unexpected SQL output:\ngot:  %s\nwant: %s", sqlStr, expectedSQL)
	}
}

func TestSqlutilInterpolate(t *testing.T) {
	queryStr := "SELECT * FROM users WHERE email = $1 AND age = $2"
	params := []any{"bob@example.com", 25}
	interpolated := sqlutil.Interpolate(queryStr, params)

	expected := "SELECT * FROM users WHERE email = 'bob@example.com' AND age = 25"
	if interpolated != expected {
		t.Errorf("unexpected interpolation result: got %q, want %q", interpolated, expected)
	}
}

func TestBuildDeleteHardNoSoftFilter(t *testing.T) {
	q := &query.Delete{
		From: "users",
		Where: &query.Filter{
			Eq: map[string]any{"id": "123"},
		},
		Soft: false,
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "deleted_at", false)
	if err != nil {
		t.Fatalf("failed to build delete: %v", err)
	}

	expected := `DELETE FROM "public"."users" WHERE "users"."id" = $1 RETURNING *`
	if sqlStr != expected {
		t.Errorf("unexpected SQL output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildUpdateEmptyDataError(t *testing.T) {
	q := &query.Update{
		Table: "users",
		Data:  nil,
	}

	pt := &ParamTracker{}
	_, err := Build(q, pt, "public", "", false)
	if err == nil {
		t.Error("expected error for update with empty data map, got nil")
	}
}

func TestBuildGetEmptyJoinOnConditionError(t *testing.T) {
	q := &query.Get{
		From: "orders",
		Include: []query.Join{
			{
				From: "users",
				On:   nil,
			},
		},
	}

	pt := &ParamTracker{}
	_, err := Build(q, pt, "public", "", false)
	if err == nil {
		t.Error("expected error for join with empty On map, got nil")
	}
}

func TestBuildAggregate(t *testing.T) {
	q := &query.Aggregate{
		From:    "orders",
		Fields:  []string{"category"},
		GroupBy: []string{"category"},
		Sum:     []string{"total"},
		Avg:     []string{"total"},
		Include: []query.Join{
			{
				From:      "users",
				Alias:     "customer",
				Selection: []string{"name"},
				On:        map[string]string{"customer_id": "id"},
			},
		},
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "deleted_at", false)
	if err != nil {
		t.Fatalf("failed to build aggregate query: %v", err)
	}

	expected := `SELECT "orders"."category", COALESCE(AVG("orders"."total"), 0)::float AS "total__avg", COALESCE(SUM("orders"."total"), 0)::float AS "total__sum", "customer"."name" AS "customer__name" FROM "public"."orders" LEFT JOIN "public"."users" AS "customer" ON "orders"."customer_id" = "customer"."id" WHERE "public"."orders"."deleted_at" IS NULL GROUP BY "orders"."category"`
	if sqlStr != expected {
		t.Errorf("unexpected SQL output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildExpr(t *testing.T) {
	uq := &query.Update{
		Table: "users",
		Data: map[string]any{
			"attempts": query.Expr("attempts + 1"),
			"name":     "Bob",
		},
		Where: &query.Filter{Eq: map[string]any{"id": "123"}},
	}
	pt := &ParamTracker{}
	sqlStr, err := Build(uq, pt, "public", "", false)
	if err != nil {
		t.Fatalf("failed to build update: %v", err)
	}
	expectedUpdate := `UPDATE "public"."users" SET "attempts" = attempts + 1, "name" = $1 WHERE "users"."id" = $2 RETURNING *`
	if sqlStr != expectedUpdate {
		t.Errorf("unexpected SQL for update: got %q, want %q", sqlStr, expectedUpdate)
	}
	if len(pt.Params) != 2 {
		t.Errorf("unexpected parameters count: got %d, want 2", len(pt.Params))
	}

	iq := &query.Insert{
		Into: "users",
		Data: map[string]any{
			"created_at": query.Expr("NOW()"),
			"name":       "Alice",
		},
	}
	pt = &ParamTracker{}
	sqlStr, err = Build(iq, pt, "public", "", false)
	if err != nil {
		t.Fatalf("failed to build insert: %v", err)
	}
	expectedInsert := `INSERT INTO "public"."users" ("created_at", "name") VALUES (NOW(), $1) RETURNING *`
	if sqlStr != expectedInsert {
		t.Errorf("unexpected SQL for insert: got %q, want %q", sqlStr, expectedInsert)
	}

	usq := &query.Upsert{
		Into:       "users",
		ConflictOn: []string{"email"},
		Data: map[string]any{
			"email":    "test@example.com",
			"attempts": query.Expr("attempts + 1"),
		},
	}
	pt = &ParamTracker{}
	sqlStr, err = Build(usq, pt, "public", "", false)
	if err != nil {
		t.Fatalf("failed to build upsert: %v", err)
	}
	expectedUpsert := `INSERT INTO "public"."users" ("attempts", "email") VALUES (attempts + 1, $1) ON CONFLICT ("email") DO UPDATE SET "attempts" = EXCLUDED."attempts" RETURNING *`
	if sqlStr != expectedUpsert {
		t.Errorf("unexpected SQL for upsert: got %q, want %q", sqlStr, expectedUpsert)
	}
}
