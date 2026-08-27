package sqlutil

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetTableColumns queries information_schema to retrieve all column names for a given table,
// sorted alphabetically for deterministic query generation.
func GetTableColumns(ctx context.Context, pool *pgxpool.Pool, schema, table string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`
	rows, err := pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("pgkit: failed to query information_schema for table %q: %w", table, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, fmt.Errorf("pgkit: failed to scan column name: %w", err)
		}
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgkit: error iterating columns for table %q: %w", table, err)
	}

	sort.Strings(columns)
	return columns, nil
}
