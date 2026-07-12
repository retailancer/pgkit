package pgkit

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/retailancer/pgkit/query"
)

var (
	// ErrNotFound is returned when no records match the query.
	ErrNotFound = errors.New("pgkit: record not found")

	// ErrMultipleRecords is returned when a single record was expected but multiple were returned.
	ErrMultipleRecords = errors.New("pgkit: multiple records returned")

	// ErrUniqueViolation is returned when a unique constraint is violated.
	ErrUniqueViolation = errors.New("pgkit: unique constraint violation")

	// ErrForeignKeyViolation is returned when a foreign key constraint is violated.
	ErrForeignKeyViolation = errors.New("pgkit: foreign key constraint violation")

	// ErrCheckViolation is returned when a check constraint is violated.
	ErrCheckViolation = errors.New("pgkit: check constraint violation")

	// ErrNullViolation is returned when a NOT NULL constraint is violated.
	ErrNullViolation = errors.New("pgkit: not-null constraint violation")

	// ErrConnectionFailed is returned when connection to database cannot be established.
	ErrConnectionFailed = errors.New("pgkit: connection failed")

	// ErrTxClosed is returned when an action is performed on an already committed/rolled back transaction.
	ErrTxClosed = errors.New("pgkit: transaction already closed")

	// ErrUnknownQuery is returned when an unsupported query type is executed.
	ErrUnknownQuery = errors.New("pgkit: unknown query type")

	// ErrTxAlreadyStarted is returned when starting an already active transaction on a client.
	ErrTxAlreadyStarted = errors.New("pgkit: transaction already started")

	// ErrInvalidPointer is returned when a non-pointer type is passed to Scan methods.
	ErrInvalidPointer = query.ErrInvalidPointer

	// ErrDataCannotBeScanned is returned when query result shape doesn't match scan destination structure.
	ErrDataCannotBeScanned = query.ErrDataCannotBeScanned
)

// IsPgErrorCode checks if err or any error in its chain is a PostgreSQL error with the specified code.
func IsPgErrorCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}

// MapError translates standard pgconn.PgError errors into friendly sentinel errors.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %w", ErrUniqueViolation, pgErr)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: %w", ErrForeignKeyViolation, pgErr)
		case "23514": // check_violation
			return fmt.Errorf("%w: %w", ErrCheckViolation, pgErr)
		case "23502": // not_null_violation
			return fmt.Errorf("%w: %w", ErrNullViolation, pgErr)
		}
	}
	return err
}
