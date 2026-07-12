package query

type LogicOp string

const (
	And LogicOp = "AND"
	Or  LogicOp = "OR"
)

type FilterGroup struct {
	Name   string
	Filter *Filter
}

type Filter struct {
	Eq        map[string]any
	Neq       map[string]any
	Gt        map[string]any
	Gte       map[string]any
	Lt        map[string]any
	Lte       map[string]any
	In        map[string][]any
	NotIn     map[string][]any
	Like      map[string]string
	ILike     map[string]string
	Regexp    map[string]string
	IsNotNull []string
	IsNull    []string
	Op        LogicOp
	Groups    []FilterGroup
}
