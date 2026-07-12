package rows

import (
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ScanRows converts a pgx.Rows result set into a slice of maps, automatically parsing
// common database types like JSON/JSONB and arrays.
func ScanRows(rows pgx.Rows) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = f.Name
	}

	var results []map[string]any

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		rowMap := make(map[string]any)
		for i, col := range columns {
			if i >= len(values) {
				rowMap[col] = nil
				continue
			}

			val := values[i]
			if val == nil {
				rowMap[col] = nil
				continue
			}

			// handle JSON/JSONB fields that are scanned as raw []byte
			if b, ok := val.([]byte); ok {
				strVal := string(b)
				if (strings.HasPrefix(strVal, "{") && strings.HasSuffix(strVal, "}")) ||
					(strings.HasPrefix(strVal, "[") && strings.HasSuffix(strVal, "]")) {
					var jsonParsed any
					if err := json.Unmarshal(b, &jsonParsed); err == nil {
						rowMap[col] = jsonParsed
						continue
					}
				}
				rowMap[col] = strVal
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	return results, rows.Err()
}
