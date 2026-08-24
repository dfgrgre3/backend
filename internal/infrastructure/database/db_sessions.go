package db

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// RawWriteDB returns a GORM session that connects without the app_user role,
// bypassing Row-Level Security. It is intended for internal telemetry tables
// (e.g. http_metric_buckets) that do not require multi-tenant row isolation.
// When the app role is not in use, it falls back to WriteDB().
func RawWriteDB(ctxs ...context.Context) *gorm.DB {
	if rawWriteDB != nil {
		session := rawWriteDB.Session(&gorm.Session{})
		if len(ctxs) > 0 && ctxs[0] != nil {
			session = session.WithContext(ctxs[0])
		}
		return session
	}
	return WriteDB(ctxs...)
}

// RawReadDB returns a GORM session that connects without the app_user role,
// bypassing Row-Level Security for reads on internal telemetry tables.
// Falls back to ReadDB() when the app role is not in use.
func RawReadDB(ctxs ...context.Context) *gorm.DB {
	if rawWriteDB != nil {
		session := rawWriteDB.Session(&gorm.Session{})
		if len(ctxs) > 0 && ctxs[0] != nil {
			session = session.WithContext(ctxs[0])
		}
		return session
	}
	return ReadDB(ctxs...)
}

// ReadDB returns a GORM session explicitly routed to a read replica.
// Use this in all query (read) handlers to enforce CQRS read path.
func ReadDB(ctxs ...context.Context) *gorm.DB {
	if DB == nil {
		return nil
	}
	db := DB.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Read)
	if len(ctxs) > 0 && ctxs[0] != nil {
		db = db.WithContext(ctxs[0])
	}
	return db
}

// WriteDB returns a GORM session explicitly routed to the write source.
// Use this in all command (write) handlers to enforce CQRS write path.
func WriteDB(ctxs ...context.Context) *gorm.DB {
	if DB == nil {
		return nil
	}
	db := DB.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Write)
	if len(ctxs) > 0 && ctxs[0] != nil {
		db = db.WithContext(ctxs[0])
	}
	return db
}

// WithWriteTx executes fn within a write-routed transaction.
// This guarantees all operations in fn go to the write source.
func WithWriteTx(fn func(tx *gorm.DB) error, ctxs ...context.Context) error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	session := DB.Session(&gorm.Session{}).Clauses(dbresolver.Write)
	if len(ctxs) > 0 && ctxs[0] != nil {
		session = session.WithContext(ctxs[0])
	}
	return session.Transaction(fn)
}
