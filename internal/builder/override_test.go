package builder

import (
	"strings"
	"testing"

	"github.com/retailancer/pgkit/query"
)

func TestShouldFilterSoftDelete(t *testing.T) {
	tests := []struct {
		includeDeleted *bool
		softDeleteCol  string
		expected       bool
	}{
		{nil, "", false},
		{nil, "deleted_at", true},
		{boolPtr(true), "deleted_at", false},
		{boolPtr(false), "deleted_at", true},
		{boolPtr(true), "", false},
	}

	for _, tt := range tests {
		got := shouldFilterSoftDelete(tt.includeDeleted, tt.softDeleteCol)
		if got != tt.expected {
			t.Errorf("shouldFilterSoftDelete(%v, %q) = %v; want %v", tt.includeDeleted, tt.softDeleteCol, got, tt.expected)
		}
	}
}

func TestShouldSetUpdatedAt(t *testing.T) {
	tests := []struct {
		setUpdatedAt  *bool
		autoUpdatedAt bool
		expected      bool
	}{
		{nil, false, false},
		{nil, true, true},
		{boolPtr(true), false, true},
		{boolPtr(false), true, false},
	}

	for _, tt := range tests {
		got := shouldSetUpdatedAt(tt.setUpdatedAt, tt.autoUpdatedAt)
		if got != tt.expected {
			t.Errorf("shouldSetUpdatedAt(%v, %v) = %v; want %v", tt.setUpdatedAt, tt.autoUpdatedAt, got, tt.expected)
		}
	}
}

func TestBuildInsertManyBoundsCheck(t *testing.T) {
	q := &query.InsertMany{
		Into:   "users",
		Fields: []string{"name", "email"},
		Values: [][]any{
			{"Alice", "alice@example.com"},
			{"Bob"},
		},
	}

	pt := &ParamTracker{}
	_, err := Build(q, pt, "public", "", false)
	if err == nil {
		t.Fatal("expected error from Build due to mismatched columns and values, got nil")
	}
	if !strings.Contains(err.Error(), "has fewer values") {
		t.Errorf("expected bounds check error message, got: %v", err)
	}
}

func TestBuildUpsertCompositeConflict(t *testing.T) {
	q := &query.Upsert{
		Into:       "users",
		ConflictOn: []string{"org_id", "email"},
		Data: map[string]any{
			"org_id": "org_1",
			"email":  "test@example.com",
			"role":   "member",
		},
	}

	pt := &ParamTracker{}
	sqlStr, err := Build(q, pt, "public", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPattern := `ON CONFLICT ("org_id", "email") DO UPDATE SET "role" = EXCLUDED."role"`
	if !strings.Contains(sqlStr, expectedPattern) {
		t.Errorf("expected SQL to contain composite conflict target: got %q, want it to contain %q", sqlStr, expectedPattern)
	}
}

func boolPtr(b bool) *bool {
	return &b
}
