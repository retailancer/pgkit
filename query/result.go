package query

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrInvalidPointer is returned when the scan destination is not a pointer.
	ErrInvalidPointer = errors.New("pgkit: destination must be a non-nil pointer")

	// ErrDataCannotBeScanned is returned when query result shape doesn't match scan destination structure.
	ErrDataCannotBeScanned = errors.New("pgkit: data cannot be scanned")
)

// Result wraps query execution data and metadata.
type Result struct {
	Data  any
	Total int64
}

// Scan unmarshals the query result data into dest.
func (r *Result) Scan(dest any) error {
	if dest == nil || reflect.TypeOf(dest).Kind() != reflect.Ptr {
		return ErrInvalidPointer
	}
	if r.Data == nil {
		return nil
	}
	b, err := json.Marshal(r.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// ScanAt unmarshals the query result element at index into dest.
// Only valid for queries returning multiple records.
func (r *Result) ScanAt(dest any, index int) error {
	if dest == nil || reflect.TypeOf(dest).Kind() != reflect.Ptr {
		return ErrInvalidPointer
	}
	if r.Data == nil {
		return nil
	}
	sliceVal := reflect.ValueOf(r.Data)
	if sliceVal.Kind() != reflect.Slice {
		return ErrDataCannotBeScanned
	}
	if index < 0 || index >= sliceVal.Len() {
		return fmt.Errorf("index %d out of bounds (length %d)", index, sliceVal.Len())
	}
	item := sliceVal.Index(index).Interface()
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// ScanIncluded unmarshals a joined relation from a single-record query result into dest.
func (r *Result) ScanIncluded(dest any, joinAlias string) error {
	if dest == nil || reflect.TypeOf(dest).Kind() != reflect.Ptr {
		return ErrInvalidPointer
	}
	if r.Data == nil {
		return nil
	}
	m, ok := r.Data.(map[string]any)
	if !ok {
		return ErrDataCannotBeScanned
	}
	relationData, ok := m[joinAlias]
	if !ok || relationData == nil {
		return nil
	}
	b, err := json.Marshal(relationData)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// ScanIncludedAt unmarshals a joined relation from a multi-record query result element at index into dest.
func (r *Result) ScanIncludedAt(dest any, index int, joinAlias string) error {
	if dest == nil || reflect.TypeOf(dest).Kind() != reflect.Ptr {
		return ErrInvalidPointer
	}
	if r.Data == nil {
		return nil
	}
	sliceVal := reflect.ValueOf(r.Data)
	if sliceVal.Kind() != reflect.Slice {
		return ErrDataCannotBeScanned
	}
	if index < 0 || index >= sliceVal.Len() {
		return fmt.Errorf("index %d out of bounds (length %d)", index, sliceVal.Len())
	}
	row := sliceVal.Index(index).Interface()
	m, ok := row.(map[string]any)
	if !ok {
		return ErrDataCannotBeScanned
	}
	relationData, ok := m[joinAlias]
	if !ok || relationData == nil {
		return nil
	}
	b, err := json.Marshal(relationData)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

// LastInsertID returns the ID string of the newly inserted record.
func (r *Result) LastInsertID() string {
	if r.Data == nil {
		return ""
	}
	m, ok := r.Data.(map[string]any)
	if !ok {
		return ""
	}
	id, ok := m["id"]
	if !ok || id == nil {
		return ""
	}
	return fmt.Sprintf("%v", id)
}

// LastInsertIDs returns the list of ID strings for a batch insertion.
func (r *Result) LastInsertIDs() []string {
	if r.Data == nil {
		return nil
	}
	m, ok := r.Data.(map[string]any)
	if !ok {
		return nil
	}
	idsVal, ok := m["ids"]
	if !ok || idsVal == nil {
		return nil
	}
	switch v := idsVal.(type) {
	case []string:
		return v
	case []any:
		res := make([]string, len(v))
		for i, item := range v {
			res[i] = fmt.Sprintf("%v", item)
		}
		return res
	}
	return nil
}
