package builder

import (
	"testing"

	"github.com/retailancer/pgkit/query"
)

func TestBuildFilterDeterministic(t *testing.T) {
	c := &query.Filter{
		Eq: map[string]any{
			"status": "active",
			"role":   "admin",
		},
		In: map[string][]any{
			"tags": {"vip", "premium"},
		},
		Op: query.And,
	}

	for i := 0; i < 20; i++ {
		pt := &ParamTracker{}
		sqlStr := BuildFilter(c, "users", pt, nil)

		expected := `"users"."role" = $1 AND "users"."status" = $2 AND "users"."tags" IN ($3, $4)`
		if sqlStr != expected {
			t.Errorf("unexpected filter output: got %q, want %q", sqlStr, expected)
		}
		if len(pt.Params) != 4 {
			t.Errorf("unexpected bound params count: got %d, want 4", len(pt.Params))
		}
	}
}

func TestBuildFilterNestedGroups(t *testing.T) {
	c := &query.Filter{
		Op: query.And,
		Eq: map[string]any{
			"organization_id": "org123",
		},
		Groups: []query.FilterGroup{
			{
				Name: "search",
				Filter: &query.Filter{
					Op: query.Or,
					Eq: map[string]any{
						"email": "user@example.com",
						"name":  "User",
					},
				},
			},
		},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "users", pt, nil)

	expected := `"users"."organization_id" = $1 AND ("users"."email" = $2 OR "users"."name" = $3)`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildFilterOperators(t *testing.T) {
	c := &query.Filter{
		Op:    query.And,
		Neq:   map[string]any{"status": "deleted"},
		Gt:    map[string]any{"age": 18},
		Lte:   map[string]any{"attempts": 3},
		NotIn: map[string][]any{"country": {"US", "CA"}},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "users", pt, nil)

	expected := `"users"."status" != $1 AND "users"."age" > $2 AND "users"."attempts" <= $3 AND "users"."country" NOT IN ($4, $5)`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildFilterIsNullDeterministic(t *testing.T) {
	c := &query.Filter{
		Op:        query.And,
		IsNull:    []string{"deleted_at", "activated_at"},
		IsNotNull: []string{"email", "id"},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "users", pt, nil)

	expected := `"users"."email" IS NOT NULL AND "users"."id" IS NOT NULL AND "users"."activated_at" IS NULL AND "users"."deleted_at" IS NULL`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildFilterLikeAndILike(t *testing.T) {
	c := &query.Filter{
		Op:    query.And,
		Like:  map[string]string{"name": "%Alice%"},
		ILike: map[string]string{"email": "%alice%"},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "users", pt, nil)

	expected := `"users"."name" LIKE $1 AND "users"."email" ILIKE $2`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildFilterContains(t *testing.T) {
	c := &query.Filter{
		Contains: map[string]any{
			"metadata": map[string]any{"status": "active"},
		},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "users", pt, nil)

	expected := `"users"."metadata" @> $1::jsonb`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}

	if len(pt.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(pt.Params))
	}

	// The param should be the JSON-encoded string
	paramStr, ok := pt.Params[0].(string)
	if !ok {
		t.Fatalf("expected string param, got %T", pt.Params[0])
	}
	if paramStr != `{"status":"active"}` {
		t.Errorf("unexpected param value: got %q, want %q", paramStr, `{"status":"active"}`)
	}
}

func TestBuildFilterContainsWithArray(t *testing.T) {
	c := &query.Filter{
		Contains: map[string]any{
			"tags": []string{"go", "postgres"},
		},
	}
	types := map[string]string{"tags": "text[]"}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "posts", pt, types)

	expected := `"posts"."tags" @> $1::text[]`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}

	paramStr := pt.Params[0].(string)
	if paramStr != `["go","postgres"]` {
		t.Errorf("unexpected param value: got %q, want %q", paramStr, `["go","postgres"]`)
	}
}

func TestBuildFilterContainsArrayNoCast(t *testing.T) {
	c := &query.Filter{
		Contains: map[string]any{
			"tags": []string{"go", "postgres"},
		},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "posts", pt, nil)

	// Without Types, array values get no cast (only maps default to jsonb)
	expected := `"posts"."tags" @> $1`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}

func TestBuildFilterContainsWithOtherOps(t *testing.T) {
	c := &query.Filter{
		Op:       query.And,
		Eq:       map[string]any{"role": "admin"},
		Contains: map[string]any{"metadata": map[string]any{"verified": true}},
	}

	pt := &ParamTracker{}
	sqlStr := BuildFilter(c, "users", pt, nil)

	expected := `"users"."role" = $1 AND "users"."metadata" @> $2::jsonb`
	if sqlStr != expected {
		t.Errorf("unexpected filter output:\ngot:  %s\nwant: %s", sqlStr, expected)
	}
}
