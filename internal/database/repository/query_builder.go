package repository

import (
	"fmt"
	"strings"
)

// QueryBuilder helps construct SQL queries safely
type QueryBuilder struct {
	table     string
	columns   []string
	where     []string
	args      []interface{}
	orderBy   []string
	limit     *int
	offset    *int
	joins     []string
	groupBy   []string
	having    []string
	returning []string
}

// NewQueryBuilder creates a new query builder for a table
func NewQueryBuilder(table string) *QueryBuilder {
	return &QueryBuilder{
		table:   table,
		columns: []string{"*"},
	}
}

// Select sets the columns to select
func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	if len(columns) > 0 {
		qb.columns = columns
	}
	return qb
}

// Where adds a WHERE condition
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	qb.where = append(qb.where, condition)
	qb.args = append(qb.args, args...)
	return qb
}

// OrderBy adds ORDER BY clause
func (qb *QueryBuilder) OrderBy(column string, desc bool) *QueryBuilder {
	if desc {
		qb.orderBy = append(qb.orderBy, column+" DESC")
	} else {
		qb.orderBy = append(qb.orderBy, column+" ASC")
	}
	return qb
}

// Limit sets the LIMIT clause
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = &limit
	return qb
}

// Offset sets the OFFSET clause
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.offset = &offset
	return qb
}

// Join adds a JOIN clause
func (qb *QueryBuilder) Join(join string) *QueryBuilder {
	qb.joins = append(qb.joins, join)
	return qb
}

// GroupBy adds GROUP BY clause
func (qb *QueryBuilder) GroupBy(columns ...string) *QueryBuilder {
	qb.groupBy = append(qb.groupBy, columns...)
	return qb
}

// Having adds HAVING clause
func (qb *QueryBuilder) Having(condition string, args ...interface{}) *QueryBuilder {
	qb.having = append(qb.having, condition)
	qb.args = append(qb.args, args...)
	return qb
}

// Returning adds RETURNING clause for INSERT/UPDATE/DELETE
func (qb *QueryBuilder) Returning(columns ...string) *QueryBuilder {
	qb.returning = columns
	return qb
}

// BuildSelect builds a SELECT query
func (qb *QueryBuilder) BuildSelect() (string, []interface{}) {
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(qb.columns, ", "), qb.table)

	if len(qb.joins) > 0 {
		query += " " + strings.Join(qb.joins, " ")
	}

	if len(qb.where) > 0 {
		query += " WHERE " + strings.Join(qb.where, " AND ")
	}

	if len(qb.groupBy) > 0 {
		query += " GROUP BY " + strings.Join(qb.groupBy, ", ")
	}

	if len(qb.having) > 0 {
		query += " HAVING " + strings.Join(qb.having, " AND ")
	}

	if len(qb.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(qb.orderBy, ", ")
	}

	if qb.limit != nil {
		query += fmt.Sprintf(" LIMIT %d", *qb.limit)
	}

	if qb.offset != nil {
		query += fmt.Sprintf(" OFFSET %d", *qb.offset)
	}

	return query, qb.args
}

// BuildInsert builds an INSERT query
func (qb *QueryBuilder) BuildInsert(columns []string, values []interface{}) (string, []interface{}) {
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		qb.table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	if len(qb.returning) > 0 {
		query += " RETURNING " + strings.Join(qb.returning, ", ")
	}

	return query, values
}

// BuildUpdate builds an UPDATE query
func (qb *QueryBuilder) BuildUpdate(columns []string, values []interface{}) (string, []interface{}) {
	sets := make([]string, len(columns))
	for i, col := range columns {
		sets[i] = fmt.Sprintf("%s = $%d", col, i+1)
	}

	query := fmt.Sprintf("UPDATE %s SET %s", qb.table, strings.Join(sets, ", "))

	args := values
	if len(qb.where) > 0 {
		query += " WHERE " + strings.Join(qb.where, " AND ")
		args = append(values, qb.args...)
	}

	if len(qb.returning) > 0 {
		query += " RETURNING " + strings.Join(qb.returning, ", ")
	}

	return query, args
}

// BuildDelete builds a DELETE query
func (qb *QueryBuilder) BuildDelete() (string, []interface{}) {
	query := fmt.Sprintf("DELETE FROM %s", qb.table)

	if len(qb.where) > 0 {
		query += " WHERE " + strings.Join(qb.where, " AND ")
	}

	if len(qb.returning) > 0 {
		query += " RETURNING " + strings.Join(qb.returning, ", ")
	}

	return query, qb.args
}
