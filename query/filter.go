package query

// LogicOp controls how conditions within a single Filter are joined.
type LogicOp string

const (
	// And joins all conditions with AND (the default behavior).
	And LogicOp = "AND"
	// Or joins all conditions with OR.
	Or LogicOp = "OR"
)

// FilterGroup represents a named nested filter group within a Filter.
// Groups are wrapped in parentheses and appended to the parent filter's conditions
// with the parent's Op. Supports recursive nesting to any depth.
type FilterGroup struct {
	// Name is an optional label for the group (used for logging/debugging, not in SQL).
	Name string

	// Filter is the nested filter condition. Required.
	Filter *Filter
}

// Filter builds the WHERE clause for Get, Aggregate, Update, and Delete queries.
// All map keys are sorted alphabetically at build time for deterministic SQL generation
// and prepared-statement reuse.
//
// Within a single Filter, all conditions are joined with AND by default.
// Use Op to switch to OR. For mixed AND/OR logic, use Groups.
type Filter struct {
	// Eq generates "col = $1" for each entry. If the value is nil, generates "col IS NULL".
	Eq map[string]any

	// Neq generates "col != $1" for each entry. If the value is nil, generates "col IS NOT NULL".
	Neq map[string]any

	// Gt generates "col > $1" for each entry.
	Gt map[string]any

	// Gte generates "col >= $1" for each entry.
	Gte map[string]any

	// Lt generates "col < $1" for each entry.
	Lt map[string]any

	// Lte generates "col <= $1" for each entry.
	Lte map[string]any

	// In generates "col IN ($1, $2, ...)" for each entry.
	// If the slice is empty, generates FALSE (safe no-op preventing invalid SQL).
	In map[string][]any

	// NotIn generates "col NOT IN ($1, $2, ...)" for each entry.
	// If the slice is empty, generates TRUE.
	NotIn map[string][]any

	// Like generates "col LIKE $1" (case-sensitive pattern matching) for each entry.
	Like map[string]string

	// ILike generates "col ILIKE $1" (case-insensitive pattern matching) for each entry.
	ILike map[string]string

	// Regexp generates "col ~* $1" (PostgreSQL case-insensitive regex) for each entry.
	Regexp map[string]string

	// IsNotNull generates "col IS NOT NULL" for each column name in the slice.
	IsNotNull []string

	// IsNull generates "col IS NULL" for each column name in the slice.
	IsNull []string

	// Op controls how conditions within this Filter are joined.
	// And (default): all conditions are joined with AND.
	// Or: all conditions are joined with OR.
	Op LogicOp

	// Groups is a list of nested filter groups. Each group is wrapped in parentheses
	// and appended to the parent filter's conditions using the parent's Op.
	// Supports recursive nesting to any depth.
	// Example: WHERE org_id = $1 AND (name ILIKE $2 OR email ILIKE $3)
	Groups []FilterGroup
}
