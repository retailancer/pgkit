package pgkit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/retailancer/pgkit/query"
)

type Tx struct {
	pgxTx  pgx.Tx
	db     *DB
	parent *Tx
	depth  int
	closed bool
}

func (tx *Tx) Exec(ctx context.Context, q query.Query) (*query.Result, error) {
	if tx.closed {
		return nil, ErrTxClosed
	}
	return tx.db.exec(ctx, tx.pgxTx, q)
}

func (tx *Tx) One(ctx context.Context, q *query.Get, dest any) error {
	if tx.closed {
		return ErrTxClosed
	}
	res, err := tx.db.execGet(ctx, tx.pgxTx, q, false)
	if err != nil {
		return err
	}
	return res.Scan(dest)
}

func (tx *Tx) Many(ctx context.Context, q *query.Get, dest any) (int64, error) {
	if tx.closed {
		return 0, ErrTxClosed
	}
	res, err := tx.db.execGet(ctx, tx.pgxTx, q, true)
	if err != nil {
		return 0, err
	}
	if err := res.Scan(dest); err != nil {
		return 0, err
	}
	return res.Total, nil
}

func (tx *Tx) Count(ctx context.Context, q *query.Get) (int64, error) {
	if tx.closed {
		return 0, ErrTxClosed
	}
	return tx.db.count(ctx, tx.pgxTx, q)
}

func (tx *Tx) Insert(ctx context.Context, q *query.Insert) (string, error) {
	if tx.closed {
		return "", ErrTxClosed
	}
	res, err := tx.db.execWrite(ctx, tx.pgxTx, q)
	if err != nil {
		return "", err
	}
	return res.LastInsertID(), nil
}

func (tx *Tx) InsertMany(ctx context.Context, q *query.InsertMany) ([]string, error) {
	if tx.closed {
		return nil, ErrTxClosed
	}
	res, err := tx.db.execWrite(ctx, tx.pgxTx, q)
	if err != nil {
		return nil, err
	}
	return res.LastInsertIDs(), nil
}

func (tx *Tx) Upsert(ctx context.Context, q *query.Upsert) (string, error) {
	if tx.closed {
		return "", ErrTxClosed
	}
	res, err := tx.db.execWrite(ctx, tx.pgxTx, q)
	if err != nil {
		return "", err
	}
	return res.LastInsertID(), nil
}

func (tx *Tx) Update(ctx context.Context, q *query.Update) error {
	if tx.closed {
		return ErrTxClosed
	}
	_, err := tx.db.execWrite(ctx, tx.pgxTx, q)
	return err
}

func (tx *Tx) Delete(ctx context.Context, q *query.Delete) error {
	if tx.closed {
		return ErrTxClosed
	}
	_, err := tx.db.execWrite(ctx, tx.pgxTx, q)
	return err
}

// Begin starts a nested transaction using a SAVEPOINT.
func (tx *Tx) Begin(ctx context.Context) (*Tx, error) {
	if tx.closed {
		return nil, ErrTxClosed
	}

	childDepth := tx.depth + 1
	spName := fmt.Sprintf("pgkit_sp_%d", childDepth)

	_, err := tx.pgxTx.Exec(ctx, "SAVEPOINT "+spName)
	if err != nil {
		return nil, MapError(err)
	}

	return &Tx{
		pgxTx:  tx.pgxTx,
		db:     tx.db,
		parent: tx,
		depth:  childDepth,
	}, nil
}

// Commit commits the transaction. If it is a nested transaction, it releases the SAVEPOINT.
func (tx *Tx) Commit(ctx context.Context) error {
	if tx.closed {
		return ErrTxClosed
	}
	tx.closed = true

	if tx.depth > 0 {
		spName := fmt.Sprintf("pgkit_sp_%d", tx.depth)
		_, err := tx.pgxTx.Exec(ctx, "RELEASE SAVEPOINT "+spName)
		return MapError(err)
	}

	err := tx.pgxTx.Commit(ctx)
	return MapError(err)
}

// Rollback rolls back the transaction. If it is a nested transaction, it rolls back to the SAVEPOINT.
func (tx *Tx) Rollback(ctx context.Context) error {
	if tx.closed {
		return ErrTxClosed
	}
	tx.closed = true

	if tx.depth > 0 {
		spName := fmt.Sprintf("pgkit_sp_%d", tx.depth)
		_, err := tx.pgxTx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
		return MapError(err)
	}

	err := tx.pgxTx.Rollback(ctx)
	return MapError(err)
}

// WithTx runs the function fn inside a nested transaction.
func (tx *Tx) WithTx(ctx context.Context, fn func(nestedTx *Tx) error) error {
	child, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if !child.closed {
			_ = child.Rollback(ctx)
		}
	}()
	if err := fn(child); err != nil {
		return err
	}
	return child.Commit(ctx)
}
