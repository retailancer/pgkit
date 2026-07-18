package query

type Query interface {
	isQuery()
}

type Get struct {
	From           string
	Selection      []string
	Where          *Filter
	Order          map[string]string
	GroupBy        []string
	DistinctOn     []string
	Limit          int
	Offset         int
	IncludeDeleted *bool
	Include        []Join
	Types          map[string]string
	Log            bool
	ForUpdate      bool
	ShuffleOn      string
	SkipCount      bool
}

type JoinType string

const (
	LeftJoin  JoinType = "LEFT"
	InnerJoin JoinType = "INNER"
	RightJoin JoinType = "RIGHT"
	FullJoin  JoinType = "FULL"
)

type Join struct {
	Type      JoinType
	From      string
	Alias     string
	Selection []string
	Many      bool
	On        map[string]string
	Where     *Filter
	Types     map[string]string
	Order     map[string]string
	GroupBy   []string
}

type Aggregate struct {
	From           string
	Fields         []string
	Avg            []string
	Count          []string
	Max            []string
	Min            []string
	Sum            []string
	GroupBy        []string
	Where          *Filter
	Include        []Join
	Order          map[string]string
	Limit          int
	Offset         int
	Log            bool
	IncludeDeleted *bool
}

type Insert struct {
	Into         string
	Data         map[string]any
	Types        map[string]string
	SetUpdatedAt *bool
}

type InsertMany struct {
	Into         string
	Fields       []string
	Values       [][]any
	Types        map[string]string
	SetUpdatedAt *bool
}

type Upsert struct {
	Into         string
	ConflictOn   []string
	Data         map[string]any
	Where        *Filter
	Types        map[string]string
	SetUpdatedAt *bool
}

type Update struct {
	Table        string
	Data         map[string]any
	Where        *Filter
	Types        map[string]string
	SetUpdatedAt *bool
}

type Delete struct {
	From           string
	Where          *Filter
	Soft           bool
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
