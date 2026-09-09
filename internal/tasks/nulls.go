package tasks

import (
	"database/sql"
	"time"
)

func nullInt64(v int64) sql.NullInt64    { return sql.NullInt64{Int64: v, Valid: true} }
func nullString(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
func nullBool(v bool) sql.NullBool       { return sql.NullBool{Bool: v, Valid: true} }

func timePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func int64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
