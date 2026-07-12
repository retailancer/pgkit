package rows

import (
	"fmt"
	"strings"
)

// NestRelations processes flat map rows containing double-underscore keys (e.g. alias__col)
// and moves them into nested maps under the alias key.
func NestRelations(rows []map[string]any) []map[string]any {
	for _, r := range rows {
		var toDelete []string
		for k, v := range r {
			if strings.Contains(k, "__") {
				parts := strings.SplitN(k, "__", 2)
				alias := parts[0]
				field := parts[1]

				if _, ok := r[alias]; !ok {
					r[alias] = map[string]any{}
				}

				if nestedMap, ok := r[alias].(map[string]any); ok {
					nestedMap[field] = v
				}
				toDelete = append(toDelete, k)
			}
		}
		for _, k := range toDelete {
			delete(r, k)
		}
	}
	return rows
}

// AggregateRows merges multiple duplicate parent rows resulting from one-to-many joins
// and groups child records into slices under the joined table's alias.
func AggregateRows(rows []map[string]any, manyAliases []string) []map[string]any {
	if len(rows) == 0 {
		return rows
	}

	manyFields := map[string]bool{}
	for _, alias := range manyAliases {
		manyFields[alias] = true
	}

	groupedMap := map[string]map[string]any{}
	seenMap := make(map[string]map[string]map[any]bool)
	var order []string

	for _, row := range rows {
		idVal, ok := row["id"]
		if !ok {
			continue
		}
		id := fmt.Sprintf("%v", idVal)

		mainEntry, exists := groupedMap[id]
		if !exists {
			mainEntry = make(map[string]any)
			seenMap[id] = make(map[string]map[any]bool)
			for k, v := range row {
				if isMany := manyFields[k]; isMany {
					mainEntry[k] = []any{}
					seenMap[id][k] = make(map[any]bool)
					if vMap, ok := v.(map[string]any); ok && len(vMap) > 0 {
						if childIDVal, ok := vMap["id"]; ok && childIDVal != nil {
							mainEntry[k] = append(mainEntry[k].([]any), vMap)
							seenMap[id][k][childIDVal] = true
						}
					}
				} else {
					mainEntry[k] = v
				}
			}
			groupedMap[id] = mainEntry
			order = append(order, id)
		} else {
			for field := range manyFields {
				val := row[field]
				if vMap, ok := val.(map[string]any); ok && len(vMap) > 0 {
					childIDVal, ok := vMap["id"]
					if !ok || childIDVal == nil {
						continue
					}

					if seenMap[id][field] == nil {
						seenMap[id][field] = make(map[any]bool)
					}

					if !seenMap[id][field][childIDVal] {
						list, ok := mainEntry[field].([]any)
						if !ok {
							list = []any{}
						}
						mainEntry[field] = append(list, vMap)
						seenMap[id][field][childIDVal] = true
					}
				}
			}
		}
	}

	result := make([]map[string]any, 0, len(order))
	for _, id := range order {
		result = append(result, groupedMap[id])
	}

	return result
}
