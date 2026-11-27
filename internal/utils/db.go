package utils

import (
	"context"
	"database/sql"
	"errors"

	"github.com/uptrace/bun"
)

// IsNotFound reports whether err represents a missing database row.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// NextSortOrder returns the next sequential sort_order value (max + 1) for a model.
func NextSortOrder(ctx context.Context, db *bun.DB, model any, whereClause string, args ...any) (int, error) {
	var maxSort int
	q := db.NewSelect().Model(model).ColumnExpr("COALESCE(MAX(sort_order), 0)")
	if whereClause != "" {
		q = q.Where(whereClause, args...)
	}
	if err := q.Scan(ctx, &maxSort); err != nil {
		return 0, err
	}
	return maxSort + 1, nil
}

// HasRowsAffected reports whether a SQL result affected at least one row.
func HasRowsAffected(res sql.Result) bool {
	n, err := res.RowsAffected()
	return err == nil && n > 0
}

// DeleteByID deletes a record by its id primary key and reports whether a row was deleted.
func DeleteByID(ctx context.Context, db *bun.DB, model any, id string) (bool, error) {
	res, err := db.NewDelete().Model(model).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	return HasRowsAffected(res), nil
}
