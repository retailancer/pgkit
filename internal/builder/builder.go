package builder

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/retailancer/pgkit/internal/sqlutil"
	"github.com/retailancer/pgkit/query"
)

// ParamTracker tracks the query parameters to bind placeholders ($1, $2, etc.) deterministically.
type ParamTracker struct {
	Params []any
}

func (pt *ParamTracker) Next(v any) string {
	pt.Params = append(pt.Params, v)
	return fmt.Sprintf("$%d", len(pt.Params))
}

func Build(q query.Query, pt *ParamTracker, searchPath string, softDeleteCol string, autoUpdatedAt bool) (string, error) {
	switch queryObj := q.(type) {
	case *query.Get:
		return buildGet(queryObj, pt, searchPath, softDeleteCol)
	case *query.Insert:
		return buildInsert(queryObj, pt, searchPath, autoUpdatedAt)
	case *query.InsertMany:
		return buildInsertMany(queryObj, pt, searchPath, autoUpdatedAt)
	case *query.Upsert:
		return buildUpsert(queryObj, pt, searchPath, autoUpdatedAt)
	case *query.Update:
		return buildUpdate(queryObj, pt, searchPath, autoUpdatedAt)
	case *query.Delete:
		return buildDelete(queryObj, pt, searchPath, softDeleteCol)
	case *query.Aggregate:
		return buildAggregate(queryObj, pt, searchPath, softDeleteCol)
	default:
		return "", fmt.Errorf("unsupported query type: %T", q)
	}
}

func shouldFilterSoftDelete(includeDeleted *bool, softDeleteCol string) bool {
	if softDeleteCol == "" {
		return false
	}
	if includeDeleted != nil {
		return !*includeDeleted
	}
	return true
}

func shouldSetUpdatedAt(setUpdatedAt *bool, autoUpdatedAt bool) bool {
	if setUpdatedAt != nil {
		return *setUpdatedAt
	}
	return autoUpdatedAt
}

func buildGet(q *query.Get, pt *ParamTracker, schema string, softDeleteCol string) (string, error) {
	var queryStr strings.Builder

	queryStr.WriteString("SELECT ")

	if len(q.DistinctOn) > 0 {
		queryStr.WriteString("DISTINCT ON (")
		var parts []string
		for _, v := range q.DistinctOn {
			if strings.Contains(v, ".") {
				parts = append(parts, sqlutil.QuoteIdent(strings.Split(v, ".")[0], strings.Split(v, ".")[1]))
			} else {
				parts = append(parts, sqlutil.QuoteIdent(q.From, v))
			}
		}
		queryStr.WriteString(strings.Join(parts, ", "))
		queryStr.WriteString(") ")
	}

	var selectionParts []string
	for _, v := range q.Selection {
		selectionParts = append(selectionParts, sqlutil.QuoteIdent(q.From, v))
	}

	if len(selectionParts) == 0 {
		selectionParts = append(selectionParts, sqlutil.QuoteIdent(q.From)+".*")
	}

	for _, v := range q.Include {
		alias := v.Alias
		if alias == "" {
			alias = v.From
		}
		if len(v.Selection) == 0 {
			selectionParts = append(selectionParts, fmt.Sprintf("%s AS %s", sqlutil.QuoteIdent(alias, "id"), sqlutil.QuoteIdent(alias+"__id")))
		} else {
			for _, s := range v.Selection {
				selectionParts = append(selectionParts, fmt.Sprintf("%s AS %s", sqlutil.QuoteIdent(alias, s), sqlutil.QuoteIdent(alias+"__"+s)))
			}
		}
	}

	queryStr.WriteString(strings.Join(selectionParts, ", "))
	queryStr.WriteString(" FROM ")
	queryStr.WriteString(sqlutil.QuoteIdent(schema, q.From))

	for _, v := range q.Include {
		alias := v.Alias
		if alias == "" {
			alias = v.From
		}
		if len(v.On) == 0 {
			return "", fmt.Errorf("pgkit: join on table %q requires at least one On condition", v.From)
		}
		joinType := v.Type
		if joinType == "" {
			joinType = query.LeftJoin
		}
		fmt.Fprintf(&queryStr, " %s JOIN %s AS %s ON ", joinType, sqlutil.QuoteIdent(schema, v.From), sqlutil.QuoteIdent(alias))

		var condParts []string
		condKeys := sqlutil.SortedKeys(v.On)
		for _, t := range condKeys {
			c := v.On[t]
			fromTable := q.From
			colName := t
			if strings.Contains(t, ".") {
				fromTable = strings.Split(t, ".")[0]
				colName = strings.Split(t, ".")[1]
			}
			condParts = append(condParts, fmt.Sprintf("%s = %s", sqlutil.QuoteIdent(fromTable, colName), sqlutil.QuoteIdent(alias, c)))
		}
		queryStr.WriteString(strings.Join(condParts, " AND "))
	}

	var whereParts []string

	if shouldFilterSoftDelete(q.IncludeDeleted, softDeleteCol) {
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", sqlutil.QuoteIdent(schema, q.From, softDeleteCol)))
	}

	if q.Where != nil {
		cStr := BuildFilter(q.Where, q.From, pt, q.Types)
		if cStr != "" {
			whereParts = append(whereParts, "("+cStr+")")
		}
	}

	for _, v := range q.Include {
		if v.Where != nil {
			alias := v.Alias
			if alias == "" {
				alias = v.From
			}
			cStr := BuildFilter(v.Where, alias, pt, v.Types)
			if cStr != "" {
				whereParts = append(whereParts, "("+cStr+")")
			}
		}
	}

	if len(whereParts) > 0 {
		queryStr.WriteString(" WHERE ")
		queryStr.WriteString(strings.Join(whereParts, " AND "))
	}

	var groupByParts []string
	if len(q.GroupBy) > 0 {
		for _, v := range q.GroupBy {
			groupByParts = append(groupByParts, sqlutil.QuoteIdent(q.From, v))
		}
	}
	for _, v := range q.Include {
		if len(v.GroupBy) > 0 {
			alias := v.Alias
			if alias == "" {
				alias = v.From
			}
			for _, vv := range v.GroupBy {
				groupByParts = append(groupByParts, sqlutil.QuoteIdent(alias, vv))
			}
		}
	}
	if len(groupByParts) > 0 {
		queryStr.WriteString(" GROUP BY ")
		queryStr.WriteString(strings.Join(groupByParts, ", "))
	}

	if q.ShuffleOn != "" {
		seed := time.Now().Format("2006-01-02-15")
		queryStr.WriteString(fmt.Sprintf(` ORDER BY md5(%s || '%s')`, sqlutil.QuoteIdent(q.From, q.ShuffleOn), seed))
	} else {
		var orderByParts []string
		if len(q.Order) > 0 {
			orderKeys := sqlutil.SortedKeys(q.Order)
			for _, k := range orderKeys {
				v := q.Order[k]
				orderByParts = append(orderByParts, fmt.Sprintf("%s %s", sqlutil.QuoteIdent(q.From, k), v))
			}
		}
		for _, v := range q.Include {
			if len(v.Order) > 0 {
				alias := v.Alias
				if alias == "" {
					alias = v.From
				}
				orderKeys := sqlutil.SortedKeys(v.Order)
				for _, k := range orderKeys {
					vv := v.Order[k]
					orderByParts = append(orderByParts, fmt.Sprintf("%s %s", sqlutil.QuoteIdent(alias, k), vv))
				}
			}
		}
		if len(orderByParts) > 0 {
			queryStr.WriteString(" ORDER BY ")
			queryStr.WriteString(strings.Join(orderByParts, ", "))
		}
	}

	if q.Limit > 0 {
		queryStr.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))
	}
	if q.Offset > 0 {
		queryStr.WriteString(fmt.Sprintf(" OFFSET %d", q.Offset))
	}

	if q.ForUpdate {
		queryStr.WriteString(" FOR UPDATE")
	}

	return queryStr.String(), nil
}

func buildInsert(q *query.Insert, pt *ParamTracker, schema string, autoUpdatedAt bool) (string, error) {
	var queryStr strings.Builder
	queryStr.WriteString("INSERT INTO ")
	queryStr.WriteString(sqlutil.QuoteIdent(schema, q.Into))
	queryStr.WriteString(" (")

	keys := sqlutil.SortedKeys(q.Data)
	var colParts []string
	var valParts []string

	for _, k := range keys {
		v := q.Data[k]
		colParts = append(colParts, sqlutil.QuoteIdent(k))

		placeholder := pt.Next(v)
		if t, ok := q.Types[k]; ok {
			placeholder += "::" + t
		}
		valParts = append(valParts, placeholder)
	}

	if shouldSetUpdatedAt(q.SetUpdatedAt, autoUpdatedAt) {
		colParts = append(colParts, sqlutil.QuoteIdent("updated_at"))
		placeholder := pt.Next(time.Now())
		if t, ok := q.Types["updated_at"]; ok {
			placeholder += "::" + t
		} else {
			placeholder += "::timestamptz"
		}
		valParts = append(valParts, placeholder)
	}

	queryStr.WriteString(strings.Join(colParts, ", "))
	queryStr.WriteString(") VALUES (")
	queryStr.WriteString(strings.Join(valParts, ", "))
	queryStr.WriteString(") RETURNING *")

	return queryStr.String(), nil
}

func buildInsertMany(q *query.InsertMany, pt *ParamTracker, schema string, autoUpdatedAt bool) (string, error) {
	if len(q.Values) == 0 {
		return "", fmt.Errorf("no values provided for insert many")
	}

	var queryStr strings.Builder
	queryStr.WriteString("INSERT INTO ")
	queryStr.WriteString(sqlutil.QuoteIdent(schema, q.Into))
	queryStr.WriteString(" (")

	var colParts []string
	for _, col := range q.Fields {
		colParts = append(colParts, sqlutil.QuoteIdent(col))
	}

	hasUpdatedAt := slices.Contains(q.Fields, "updated_at")

	setUpdate := shouldSetUpdatedAt(q.SetUpdatedAt, autoUpdatedAt)
	if setUpdate && !hasUpdatedAt {
		colParts = append(colParts, sqlutil.QuoteIdent("updated_at"))
	}

	queryStr.WriteString(strings.Join(colParts, ", "))
	queryStr.WriteString(") VALUES ")

	var rowParts []string
	for idx, row := range q.Values {
		if len(row) < len(q.Fields) {
			return "", fmt.Errorf("row %d has fewer values (%d) than fields (%d)", idx, len(row), len(q.Fields))
		}
		var valParts []string
		for i, col := range q.Fields {
			val := row[i]
			placeholder := pt.Next(val)
			if t, ok := q.Types[col]; ok {
				placeholder += "::" + t
			}
			valParts = append(valParts, placeholder)
		}

		if setUpdate && !hasUpdatedAt {
			placeholder := pt.Next(time.Now())
			if t, ok := q.Types["updated_at"]; ok {
				placeholder += "::" + t
			} else {
				placeholder += "::timestamptz"
			}
			valParts = append(valParts, placeholder)
		}

		rowParts = append(rowParts, "("+strings.Join(valParts, ", ")+")")
	}

	queryStr.WriteString(strings.Join(rowParts, ", "))
	queryStr.WriteString(" RETURNING *")

	return queryStr.String(), nil
}

func buildUpsert(q *query.Upsert, pt *ParamTracker, schema string, autoUpdatedAt bool) (string, error) {
	if len(q.ConflictOn) == 0 {
		return "", fmt.Errorf("upsert requires at least one field in ConflictOn")
	}

	var queryStr strings.Builder
	queryStr.WriteString("INSERT INTO ")
	queryStr.WriteString(sqlutil.QuoteIdent(schema, q.Into))
	queryStr.WriteString(" (")

	keys := sqlutil.SortedKeys(q.Data)
	var colParts []string
	var valParts []string

	for _, k := range keys {
		v := q.Data[k]
		colParts = append(colParts, sqlutil.QuoteIdent(k))

		placeholder := pt.Next(v)
		if t, ok := q.Types[k]; ok {
			placeholder += "::" + t
		}
		valParts = append(valParts, placeholder)
	}

	if shouldSetUpdatedAt(q.SetUpdatedAt, autoUpdatedAt) {
		colParts = append(colParts, sqlutil.QuoteIdent("updated_at"))
		placeholder := pt.Next(time.Now())
		if t, ok := q.Types["updated_at"]; ok {
			placeholder += "::" + t
		} else {
			placeholder += "::timestamptz"
		}
		valParts = append(valParts, placeholder)
	}

	queryStr.WriteString(strings.Join(colParts, ", "))
	queryStr.WriteString(") VALUES (")
	queryStr.WriteString(strings.Join(valParts, ", "))
	queryStr.WriteString(") ON CONFLICT (")

	var conflictParts []string
	for _, col := range q.ConflictOn {
		conflictParts = append(conflictParts, sqlutil.QuoteIdent(col))
	}
	queryStr.WriteString(strings.Join(conflictParts, ", "))
	queryStr.WriteString(") DO UPDATE SET ")

	conflictMap := make(map[string]bool)
	for _, col := range q.ConflictOn {
		conflictMap[col] = true
	}

	var updateParts []string
	for _, k := range keys {
		if k == "id" || conflictMap[k] {
			continue
		}
		updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", sqlutil.QuoteIdent(k), sqlutil.QuoteIdent(k)))
	}

	if shouldSetUpdatedAt(q.SetUpdatedAt, autoUpdatedAt) {
		updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", sqlutil.QuoteIdent("updated_at"), sqlutil.QuoteIdent("updated_at")))
	}

	if len(updateParts) == 0 {
		queryStr.WriteString("NOTHING")
	} else {
		queryStr.WriteString(strings.Join(updateParts, ", "))
	}

	queryStr.WriteString(" RETURNING *")

	return queryStr.String(), nil
}

func buildUpdate(q *query.Update, pt *ParamTracker, schema string, autoUpdatedAt bool) (string, error) {
	if len(q.Data) == 0 {
		return "", fmt.Errorf("pgkit: update requires at least one field in Data")
	}
	var queryStr strings.Builder
	queryStr.WriteString("UPDATE ")
	queryStr.WriteString(sqlutil.QuoteIdent(schema, q.Table))
	queryStr.WriteString(" SET ")

	keys := sqlutil.SortedKeys(q.Data)
	var setParts []string

	for _, k := range keys {
		v := q.Data[k]
		placeholder := pt.Next(v)
		if t, ok := q.Types[k]; ok {
			placeholder += "::" + t
		}
		setParts = append(setParts, fmt.Sprintf("%s = %s", sqlutil.QuoteIdent(k), placeholder))
	}

	if shouldSetUpdatedAt(q.SetUpdatedAt, autoUpdatedAt) {
		placeholder := pt.Next(time.Now())
		if t, ok := q.Types["updated_at"]; ok {
			placeholder += "::" + t
		} else {
			placeholder += "::timestamptz"
		}
		setParts = append(setParts, fmt.Sprintf("%s = %s", sqlutil.QuoteIdent("updated_at"), placeholder))
	}

	queryStr.WriteString(strings.Join(setParts, ", "))

	if q.Where != nil {
		cStr := BuildFilter(q.Where, q.Table, pt, q.Types)
		if cStr != "" {
			queryStr.WriteString(" WHERE ")
			queryStr.WriteString(cStr)
		}
	}

	queryStr.WriteString(" RETURNING *")

	return queryStr.String(), nil
}

func buildDelete(q *query.Delete, pt *ParamTracker, schema string, softDeleteCol string) (string, error) {
	var queryStr strings.Builder
	if q.Soft {
		if softDeleteCol == "" {
			return "", fmt.Errorf("cannot perform soft delete: no soft delete column configured in Options")
		}
		queryStr.WriteString("UPDATE ")
		queryStr.WriteString(sqlutil.QuoteIdent(schema, q.From))
		queryStr.WriteString(" SET ")
		queryStr.WriteString(sqlutil.QuoteIdent(softDeleteCol) + " = " + pt.Next(time.Now()))
	} else {
		queryStr.WriteString("DELETE FROM ")
		queryStr.WriteString(sqlutil.QuoteIdent(schema, q.From))
	}

	var whereParts []string
	if q.Soft && shouldFilterSoftDelete(q.IncludeDeleted, softDeleteCol) {
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", sqlutil.QuoteIdent(schema, q.From, softDeleteCol)))
	}

	if q.Where != nil {
		cStr := BuildFilter(q.Where, q.From, pt, nil)
		if cStr != "" {
			whereParts = append(whereParts, cStr)
		}
	}

	if len(whereParts) > 0 {
		queryStr.WriteString(" WHERE ")
		queryStr.WriteString(strings.Join(whereParts, " AND "))
	}

	queryStr.WriteString(" RETURNING *")

	return queryStr.String(), nil
}

func buildAggregate(q *query.Aggregate, pt *ParamTracker, schema string, softDeleteCol string) (string, error) {
	var queryStr strings.Builder
	queryStr.WriteString("SELECT ")

	var selectParts []string
	for _, v := range q.Fields {
		selectParts = append(selectParts, sqlutil.QuoteIdent(q.From, v))
	}

	for _, v := range q.Avg {
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(AVG(%s), 0)::float AS %s", sqlutil.QuoteIdent(q.From, v), sqlutil.QuoteIdent(v+"__avg")))
	}

	for _, v := range q.Count {
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(COUNT(%s), 0)::float AS %s", sqlutil.QuoteIdent(q.From, v), sqlutil.QuoteIdent(v+"__count")))
	}

	for _, v := range q.Max {
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(MAX(%s), 0)::float AS %s", sqlutil.QuoteIdent(q.From, v), sqlutil.QuoteIdent(v+"__max")))
	}

	for _, v := range q.Min {
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(MIN(%s), 0)::float AS %s", sqlutil.QuoteIdent(q.From, v), sqlutil.QuoteIdent(v+"__min")))
	}

	for _, v := range q.Sum {
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(SUM(%s), 0)::float AS %s", sqlutil.QuoteIdent(q.From, v), sqlutil.QuoteIdent(v+"__sum")))
	}

	for _, v := range q.Include {
		alias := v.Alias
		if alias == "" {
			alias = v.From
		}
		if len(v.Selection) == 0 {
			selectParts = append(selectParts, fmt.Sprintf("COUNT(%s)::float AS %s", sqlutil.QuoteIdent(alias, "id"), sqlutil.QuoteIdent(alias+"__count")))
		} else {
			for _, s := range v.Selection {
				selectParts = append(selectParts, fmt.Sprintf("%s AS %s", sqlutil.QuoteIdent(alias, s), sqlutil.QuoteIdent(alias+"__"+s)))
			}
		}
	}

	queryStr.WriteString(strings.Join(selectParts, ", "))
	queryStr.WriteString(" FROM ")
	queryStr.WriteString(sqlutil.QuoteIdent(schema, q.From))

	for _, v := range q.Include {
		alias := v.Alias
		if alias == "" {
			alias = v.From
		}
		if len(v.On) == 0 {
			return "", fmt.Errorf("pgkit: join on table %q requires at least one On condition", v.From)
		}
		joinType := "LEFT"
		if v.Type != "" {
			joinType = strings.ToUpper(string(v.Type))
		}
		fmt.Fprintf(&queryStr, " %s JOIN %s AS %s ON ", joinType, sqlutil.QuoteIdent(schema, v.From), sqlutil.QuoteIdent(alias))

		var condParts []string
		condKeys := sqlutil.SortedKeys(v.On)
		for _, t := range condKeys {
			c := v.On[t]
			fromTable := q.From
			colName := t
			if strings.Contains(t, ".") {
				fromTable = strings.Split(t, ".")[0]
				colName = strings.Split(t, ".")[1]
			}
			condParts = append(condParts, fmt.Sprintf("%s = %s", sqlutil.QuoteIdent(fromTable, colName), sqlutil.QuoteIdent(alias, c)))
		}
		queryStr.WriteString(strings.Join(condParts, " AND "))
	}

	var whereParts []string
	if shouldFilterSoftDelete(q.IncludeDeleted, softDeleteCol) {
		whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", sqlutil.QuoteIdent(schema, q.From, softDeleteCol)))
	}

	if q.Where != nil {
		cStr := BuildFilter(q.Where, q.From, pt, nil)
		if cStr != "" {
			whereParts = append(whereParts, "("+cStr+")")
		}
	}

	for _, v := range q.Include {
		if v.Where != nil {
			alias := v.Alias
			if alias == "" {
				alias = v.From
			}
			cStr := BuildFilter(v.Where, alias, pt, nil)
			if cStr != "" {
				whereParts = append(whereParts, "("+cStr+")")
			}
		}
	}

	if len(whereParts) > 0 {
		queryStr.WriteString(" WHERE ")
		queryStr.WriteString(strings.Join(whereParts, " AND "))
	}

	var groupByParts []string
	if len(q.GroupBy) > 0 {
		for _, v := range q.GroupBy {
			groupByParts = append(groupByParts, sqlutil.QuoteIdent(q.From, v))
		}
	}
	for _, v := range q.Include {
		if len(v.GroupBy) > 0 {
			alias := v.Alias
			if alias == "" {
				alias = v.From
			}
			for _, vv := range v.GroupBy {
				groupByParts = append(groupByParts, sqlutil.QuoteIdent(alias, vv))
			}
		}
	}
	if len(groupByParts) > 0 {
		queryStr.WriteString(" GROUP BY ")
		queryStr.WriteString(strings.Join(groupByParts, ", "))
	}

	if len(q.Order) > 0 {
		queryStr.WriteString(" ORDER BY ")
		var orderParts []string
		orderKeys := sqlutil.SortedKeys(q.Order)
		for _, k := range orderKeys {
			v := q.Order[k]
			orderParts = append(orderParts, fmt.Sprintf("%s %s", sqlutil.QuoteIdent(k), v))
		}
		queryStr.WriteString(strings.Join(orderParts, ", "))
	}

	if q.Limit > 0 {
		queryStr.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))
	}
	if q.Offset > 0 {
		queryStr.WriteString(fmt.Sprintf(" OFFSET %d", q.Offset))
	}

	return queryStr.String(), nil
}
