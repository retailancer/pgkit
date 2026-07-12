package sqlutil

import (
	"fmt"
	"sort"
	"strings"
)

// QuoteIdent double-quotes a database table or column identifier.
// If the identifier contains a dot (e.g., "table.column"), it quotes each part separately.
func QuoteIdent(parts ...string) string {
	var quoted []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		subParts := strings.Split(part, ".")
		for _, sp := range subParts {
			sp = strings.Trim(sp, `"'`)
			if sp == "*" {
				quoted = append(quoted, "*")
			} else {
				quoted = append(quoted, `"`+sp+`"`)
			}
		}
	}
	return strings.Join(quoted, ".")
}

// QuoteColumn quotes a column identifier, keeping table prefixes intact if present.
func QuoteColumn(table, col string) string {
	col = strings.TrimSpace(col)
	if col == "" {
		return ""
	}
	if strings.Contains(col, ".") {
		return QuoteIdent(col)
	}
	if table != "" {
		return QuoteIdent(table, col)
	}
	return QuoteIdent(col)
}

// SortedKeys returns the keys of a map sorted alphabetically to ensure deterministic query generation.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Interpolate replaces placeholders ($1, $2, etc.) in a SQL query with string representations of parameters for logging.
func Interpolate(query string, params []any) string {
	for i, param := range params {
		placeholder := fmt.Sprintf("$%d", i+1)
		var paramStr string
		switch v := param.(type) {
		case string:
			escaped := strings.ReplaceAll(v, "'", "''")
			paramStr = fmt.Sprintf("'%s'", escaped)
		case nil:
			paramStr = "NULL"
		default:
			paramStr = fmt.Sprintf("%v", v)
		}
		query = strings.Replace(query, placeholder, paramStr, 1)
	}
	return query
}
