package query

// Query is the interface implemented by all query types (Get, Insert, InsertMany, Upsert, Update, Delete, Aggregate).
type Query interface {
	isQuery()
}

// Get fetches rows from a table with filtering, ordering, pagination, and joins.
type Get struct {
	// From is the primary table to query (e.g. "users", "orders"). Required.
	From string

	// Selection is the list of columns to select from the primary table.
	// Each column is qualified with the table name (e.g. "users"."name").
	// If empty or nil, all columns are selected ("table".*).
	// Cannot be used together with Omit (mutually exclusive).
	Selection []string

	// Omit is the list of columns to omit from the primary table's SELECT.
	// When set, all columns except these are selected (denylist mode).
	// Cannot be used together with Selection (mutually exclusive).
	Omit []string

	// Where is the filter applied as the WHERE clause. Nil means no filter (all rows).
	// Supports Eq, Neq, Gt, Gte, Lt, Lte, In, NotIn, Like, ILike, Regexp, IsNull, IsNotNull,
	// and nested Groups for arbitrary AND/OR logic.
	Where *Filter

	// Order specifies the ORDER BY clause. Keys are column names, values are "ASC" or "DESC".
	// Entries are sorted alphabetically at build time for deterministic SQL.
	// Cannot be used together with ShuffleOn (ShuffleOn takes precedence).
	Order map[string]string

	// GroupBy is the list of columns for the GROUP BY clause from the primary table.
	GroupBy []string

	// DistinctOn is the list of expressions for DISTINCT ON. Supports "table.col" dot notation
	// for explicit table qualification. The leftmost ORDER BY key should typically match
	// a DistinctOn expression per PostgreSQL rules.
	DistinctOn []string

	// Limit is the maximum number of rows to return. 0 means no limit.
	Limit int

	// Offset is the number of rows to skip before returning results. 0 means no offset.
	// Useful for pagination in combination with Limit.
	Offset int

	// IncludeDeleted overrides the global soft-delete filter for this specific query.
	//   - nil: inherit the global default (exclude soft-deleted rows when SoftDeleteColumn is set).
	//   - true: include soft-deleted rows in the result.
	//   - false: force-exclude soft-deleted rows even if the global default changes.
	IncludeDeleted *bool

	// Include is a list of JOIN definitions for related tables. Each Join specifies the
	// table, join type, conditions, and which columns to select from the joined table.
	Include []Join

	// Types is a map of column names to PostgreSQL type casts for WHERE clause parameters.
	// For example, {"status": "text"} generates WHERE "status" = $1::text.
	// This is useful for columns that need explicit type casting (e.g. jsonb, text[], inet).
	Types map[string]string

	// Log enables SQL logging for this query. When true, the interpolated SQL (with actual
	// parameter values substituted) is passed to the configured Logger's Info method.
	Log bool

	// ForUpdate appends FOR UPDATE to the query, locking selected rows for the duration
	// of the enclosing transaction. Prevents concurrent updates to the same rows.
	ForUpdate bool

	// ShuffleOn produces a daily-stable pseudo-random ORDER BY using md5(col || 'YYYY-MM-DD-HH').
	// The value is the column name to shuffle on (e.g. "id"). Useful for discovery feeds.
	// Takes precedence over Order; both cannot be used together.
	ShuffleOn string

	// SkipCount bypasses the COUNT(*) query executed by Many() for pagination.
	// Set to true when you don't need the total record count for better performance.
	SkipCount bool
}

// JoinType represents the type of SQL JOIN.
type JoinType string

const (
	LeftJoin  JoinType = "LEFT"
	InnerJoin JoinType = "INNER"
	RightJoin JoinType = "RIGHT"
	FullJoin  JoinType = "FULL"
)

// Join defines a JOIN to a related table. Used within Get.Include or Aggregate.Include.
type Join struct {
	// Type is the SQL join type. Defaults to LeftJoin if empty.
	// Use query.LeftJoin, query.InnerJoin, query.RightJoin, or query.FullJoin.
	Type JoinType

	// From is the table to join (e.g. "users", "comments"). Required.
	From string

	// Alias is the SQL alias for the joined table. Used as the key in nested result maps
	// (e.g. Alias "customer" produces {"customer": {"name": "..."}}).
	// Defaults to From if empty.
	Alias string

	// Selection is the list of columns to select from the joined table.
	// If empty or nil, all columns are auto-discovered.
	// Cannot be used together with Omit (mutually exclusive).
	Selection []string

	// Omit is the list of columns to omit from the joined table's SELECT.
	// When set, all columns except these are auto-discovered and selected (denylist mode).
	// Cannot be used together with Selection (mutually exclusive).
	Omit []string

	// Many indicates a one-to-many relationship. When true, duplicate parent rows from
	// the join are deduplicated and child rows are aggregated into a slice per parent.
	// The parent is identified by its "id" column.
	Many bool

	// On defines the join conditions. Keys are columns on the parent table (or "table.col"
	// for explicit table qualification), and values are columns on the joined table.
	// At least one condition is required; the build will error if empty.
	// Entries are sorted alphabetically for deterministic SQL generation.
	On map[string]string

	// Where is a join-level filter appended to the main WHERE clause with AND.
	// Useful for filtering joined rows without affecting the primary query.
	Where *Filter

	// Types is a map of column names to PostgreSQL type casts for join-level filter parameters.
	Types map[string]string

	// Order specifies join-level ORDER BY clauses. Keys are column names on the joined table,
	// values are "ASC" or "DESC".
	Order map[string]string

	// GroupBy is the list of columns for join-level GROUP BY from the joined table.
	GroupBy []string
}

// Aggregate computes aggregate functions across rows with optional grouping and joins.
type Aggregate struct {
	// From is the primary table to aggregate over. Required.
	From string

	// Fields is the list of plain SELECT columns, typically the GROUP BY columns.
	// These are returned as-is without aggregation wrapping.
	Fields []string

	// Avg is the list of columns to compute COALESCE(AVG(col), 0)::float on.
	// Result columns are aliased as "col__avg".
	Avg []string

	// Count is the list of columns to compute COALESCE(COUNT(col), 0)::float on.
	// Result columns are aliased as "col__count".
	Count []string

	// Max is the list of columns to compute COALESCE(MAX(col), 0)::float on.
	// Result columns are aliased as "col__max".
	Max []string

	// Min is the list of columns to compute COALESCE(MIN(col), 0)::float on.
	// Result columns are aliased as "col__min".
	Min []string

	// Sum is the list of columns to compute COALESCE(SUM(col), 0)::float on.
	// Result columns are aliased as "col__sum".
	Sum []string

	// GroupBy is the list of columns for the GROUP BY clause.
	GroupBy []string

	// Where is the filter applied as the WHERE clause. Nil means no filter.
	Where *Filter

	// Include is a list of JOIN definitions for related tables. Uses the same Join struct
	// as Get. When a join has no Selection, all columns are auto-discovered.
	Include []Join

	// Order specifies the ORDER BY clause. Keys are column names, values are "ASC" or "DESC".
	Order map[string]string

	// Limit is the maximum number of rows to return. 0 means no limit.
	Limit int

	// Offset is the number of rows to skip. 0 means no offset.
	Offset int

	// Log enables SQL logging for this query.
	Log bool

	// IncludeDeleted overrides the global soft-delete filter for this query.
	IncludeDeleted *bool
}

// Insert inserts a single row into a table.
type Insert struct {
	// Into is the target table name. Required.
	Into string

	// Data is the column-value pairs to insert. Required.
	// If "id" is not present, it is auto-generated based on the configured IDGenerator.
	// Supports query.Expr values for server-side expressions (e.g. query.Expr("NOW()")).
	Data map[string]any

	// Types is a map of column names to PostgreSQL type casts for insert parameters.
	// For example, {"metadata": "jsonb"} generates $1::jsonb.
	Types map[string]string

	// SetUpdatedAt overrides the global AutoUpdatedAt setting for this insert.
	//   - nil: use the global AutoUpdatedAt setting.
	//   - true: always set updated_at to NOW() on this insert.
	//   - false: never set updated_at on this insert.
	SetUpdatedAt *bool
}

// InsertMany inserts multiple rows in a single round trip.
type InsertMany struct {
	// Into is the target table name. Required.
	Into string

	// Fields is the ordered list of column names. Each inner slice in Values must have
	// the same length as Fields. Required.
	Fields []string

	// Values is the list of rows to insert. Each inner slice corresponds to one row,
	// with values matching the Fields order. Required.
	// If "id" is in Fields, rows with nil or missing IDs are auto-generated.
	Values [][]any

	// Types is a map of column names to PostgreSQL type casts for insert parameters.
	Types map[string]string

	// SetUpdatedAt overrides the global AutoUpdatedAt setting for this batch insert.
	SetUpdatedAt *bool
}

// Upsert atomically inserts or updates a row via INSERT ... ON CONFLICT DO UPDATE.
type Upsert struct {
	// Into is the target table name. Required.
	Into string

	// ConflictOn is the list of columns that must have a unique constraint.
	// Used as the ON CONFLICT target. At least one column required.
	// Supports composite unique constraints (e.g. []string{"org_id", "email"}).
	ConflictOn []string

	// Data is the column-value pairs to insert or update on conflict. Required.
	// The "id" field and conflict columns are never overwritten on conflict.
	// Supports query.Expr values for server-side expressions.
	Data map[string]any

	// Where is a conditional update filter. If set, the DO UPDATE SET clause includes
	// a WHERE condition, making the update conditional. For example, only update
	// if the existing row's status is not "banned".
	Where *Filter

	// Types is a map of column names to PostgreSQL type casts for parameters.
	Types map[string]string

	// SetUpdatedAt overrides the global AutoUpdatedAt setting for this upsert.
	SetUpdatedAt *bool
}

// Update modifies rows matching the filter.
type Update struct {
	// Table is the target table name. Required.
	Table string

	// Data is the column-value pairs to set. At least one field required.
	// Supports query.Expr values for server-side expressions
	// (e.g. query.Expr("attempts + 1")).
	Data map[string]any

	// Where is the filter applied as the WHERE clause. Nil updates ALL rows in the table
	// (use with extreme caution). Always include a Where filter unless you intend
	// to update every row.
	Where *Filter

	// Types is a map of column names to PostgreSQL type casts for SET parameters.
	Types map[string]string

	// SetUpdatedAt overrides the global AutoUpdatedAt setting for this update.
	SetUpdatedAt *bool
}

// Delete removes rows from a table, either via hard delete or soft delete.
type Delete struct {
	// From is the target table name. Required.
	From string

	// Where is the filter applied as the WHERE clause. Nil deletes ALL rows in the table
	// (use with extreme caution). Always include a Where filter unless you intend
	// to delete every row.
	Where *Filter

	// Soft controls the delete mode:
	//   - true: soft delete — sets the SoftDeleteColumn to the current timestamp (requires
	//     SoftDeleteColumn to be configured in Options). The row is not physically removed.
	//   - false: hard delete — permanently removes the row from the database.
	Soft bool

	// IncludeDeleted overrides the global soft-delete filter for the WHERE clause.
	// Only relevant when Soft is true and a SoftDeleteColumn is configured.
	//   - nil: inherit the global default.
	//   - true: include soft-deleted rows (allows re-deleting already deleted rows).
	//   - false: force-exclude soft-deleted rows.
	IncludeDeleted *bool
}

func (*Get) isQuery()        {}
func (*Aggregate) isQuery()  {}
func (*Insert) isQuery()     {}
func (*InsertMany) isQuery() {}
func (*Upsert) isQuery()     {}
func (*Update) isQuery()     {}
func (*Delete) isQuery()     {}

// Expr represents a raw SQL expression that is executed server-side.
// It is rendered directly into the generated SQL query without parameter binding.
//
// WARNING: To prevent SQL injection vulnerabilities, Expr must only be used with static,
// trusted string expressions (e.g. "attempts + 1", "NOW()"). Never build Expr dynamically
// using untrusted user inputs.
type Expr string
