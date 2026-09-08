package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON stores an arbitrary Go value as a single TEXT/JSON column, so structs
// like Address don't need to be flattened into one column per field — it
// works the same way on SQLite and Postgres.
type JSON[T any] struct {
	Data T
}

func NewJSON[T any](data T) JSON[T] {
	return JSON[T]{Data: data}
}

// MarshalJSON/UnmarshalJSON make JSON[T] transparent in API responses — it
// serializes as T itself instead of {"Data": ...}.
func (j JSON[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Data)
}

func (j *JSON[T]) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &j.Data)
}

func (j JSON[T]) Value() (driver.Value, error) {
	b, err := json.Marshal(j.Data)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (j *JSON[T]) Scan(src any) error {
	if src == nil {
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("models.JSON: unsupported scan type %T", src)
	}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, &j.Data)
}
