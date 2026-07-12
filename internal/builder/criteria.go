package builder

import (
	"fmt"
	"sort"
	"strings"

	"github.com/retailancer/pgkit/internal/sqlutil"
	"github.com/retailancer/pgkit/query"
)

// BuildFilter builds the WHERE clause substring for a given query.Filter recursively.
// It returns a deterministic SQL string and registers bound parameters to the ParamTracker.
func BuildFilter(c *query.Filter, table string, pt *ParamTracker, types map[string]string) string {
	if c == nil {
		return ""
	}

	op := c.Op
	if op == "" {
		op = query.And
	}
	opStr := " " + strings.TrimSpace(string(op)) + " "

	var parts []string

	if c.Eq != nil {
		for _, k := range sqlutil.SortedKeys(c.Eq) {
			v := c.Eq[k]
			if v == nil {
				parts = append(parts, fmt.Sprintf("%s IS NULL", sqlutil.QuoteColumn(table, k)))
			} else {
				castStr := ""
				if t, ok := types[k]; ok {
					castStr = "::" + t
				}
				parts = append(parts, fmt.Sprintf("%s = %s%s", sqlutil.QuoteColumn(table, k), pt.Next(v), castStr))
			}
		}
	}

	if c.Neq != nil {
		for _, k := range sqlutil.SortedKeys(c.Neq) {
			v := c.Neq[k]
			if v == nil {
				parts = append(parts, fmt.Sprintf("%s IS NOT NULL", sqlutil.QuoteColumn(table, k)))
			} else {
				castStr := ""
				if t, ok := types[k]; ok {
					castStr = "::" + t
				}
				parts = append(parts, fmt.Sprintf("%s != %s%s", sqlutil.QuoteColumn(table, k), pt.Next(v), castStr))
			}
		}
	}

	if c.Gt != nil {
		for _, k := range sqlutil.SortedKeys(c.Gt) {
			v := c.Gt[k]
			castStr := ""
			if t, ok := types[k]; ok {
				castStr = "::" + t
			}
			parts = append(parts, fmt.Sprintf("%s > %s%s", sqlutil.QuoteColumn(table, k), pt.Next(v), castStr))
		}
	}

	if c.Gte != nil {
		for _, k := range sqlutil.SortedKeys(c.Gte) {
			v := c.Gte[k]
			castStr := ""
			if t, ok := types[k]; ok {
				castStr = "::" + t
			}
			parts = append(parts, fmt.Sprintf("%s >= %s%s", sqlutil.QuoteColumn(table, k), pt.Next(v), castStr))
		}
	}

	if c.Lt != nil {
		for _, k := range sqlutil.SortedKeys(c.Lt) {
			v := c.Lt[k]
			castStr := ""
			if t, ok := types[k]; ok {
				castStr = "::" + t
			}
			parts = append(parts, fmt.Sprintf("%s < %s%s", sqlutil.QuoteColumn(table, k), pt.Next(v), castStr))
		}
	}

	if c.Lte != nil {
		for _, k := range sqlutil.SortedKeys(c.Lte) {
			v := c.Lte[k]
			castStr := ""
			if t, ok := types[k]; ok {
				castStr = "::" + t
			}
			parts = append(parts, fmt.Sprintf("%s <= %s%s", sqlutil.QuoteColumn(table, k), pt.Next(v), castStr))
		}
	}

	if c.In != nil {
		for _, k := range sqlutil.SortedKeys(c.In) {
			v := c.In[k]
			if len(v) == 0 {
				// postgreSQL doesn't allow empty IN (), so we force a false condition
				parts = append(parts, "FALSE")
				continue
			}
			var placeholders []string
			castStr := ""
			if t, ok := types[k]; ok {
				castStr = "::" + t
			}
			for _, vv := range v {
				placeholders = append(placeholders, pt.Next(vv)+castStr)
			}
			parts = append(parts, fmt.Sprintf("%s IN (%s)", sqlutil.QuoteColumn(table, k), strings.Join(placeholders, ", ")))
		}
	}

	if c.NotIn != nil {
		for _, k := range sqlutil.SortedKeys(c.NotIn) {
			v := c.NotIn[k]
			if len(v) == 0 {
				parts = append(parts, "TRUE")
				continue
			}
			var placeholders []string
			castStr := ""
			if t, ok := types[k]; ok {
				castStr = "::" + t
			}
			for _, vv := range v {
				placeholders = append(placeholders, pt.Next(vv)+castStr)
			}
			parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", sqlutil.QuoteColumn(table, k), strings.Join(placeholders, ", ")))
		}
	}

	if c.Like != nil {
		for _, k := range sqlutil.SortedKeys(c.Like) {
			v := c.Like[k]
			parts = append(parts, fmt.Sprintf("%s LIKE %s", sqlutil.QuoteColumn(table, k), pt.Next(v)))
		}
	}

	if c.ILike != nil {
		for _, k := range sqlutil.SortedKeys(c.ILike) {
			v := c.ILike[k]
			parts = append(parts, fmt.Sprintf("%s ILIKE %s", sqlutil.QuoteColumn(table, k), pt.Next(v)))
		}
	}

	if c.Regexp != nil {
		for _, k := range sqlutil.SortedKeys(c.Regexp) {
			v := c.Regexp[k]
			parts = append(parts, fmt.Sprintf("%s ~* %s", sqlutil.QuoteColumn(table, k), pt.Next(v)))
		}
	}

	if len(c.IsNotNull) > 0 {
		// sort fields to ensure deterministic generation
		sortedFields := make([]string, len(c.IsNotNull))
		copy(sortedFields, c.IsNotNull)
		sort.Strings(sortedFields)
		for _, k := range sortedFields {
			parts = append(parts, fmt.Sprintf("%s IS NOT NULL", sqlutil.QuoteColumn(table, k)))
		}
	}

	if len(c.IsNull) > 0 {
		sortedFields := make([]string, len(c.IsNull))
		copy(sortedFields, c.IsNull)
		sort.Strings(sortedFields)
		for _, k := range sortedFields {
			parts = append(parts, fmt.Sprintf("%s IS NULL", sqlutil.QuoteColumn(table, k)))
		}
	}

	if len(c.Groups) > 0 {
		for _, group := range c.Groups {
			subQuery := BuildFilter(group.Filter, table, pt, types)
			if subQuery != "" {
				parts = append(parts, "("+subQuery+")")
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, opStr)
}
