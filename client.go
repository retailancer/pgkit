package pgkit

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/retailancer/pgkit/query"
)

type Client struct {
	id       string
	db       *DB
	activeTx *Tx
}

func (c *Client) Close() error {
	if c.activeTx != nil {
		_ = c.RollbackTx(context.Background())
	}
	c.db.mu.Lock()
	delete(c.db.clients, c.id)
	c.db.mu.Unlock()
	return nil
}

func (c *Client) CloseSilently() {
	_ = c.Close()
}

func (c *Client) StartTx(ctx context.Context) error {
	if c.activeTx != nil {
		return ErrTxAlreadyStarted
	}

	pgxTx, err := c.db.pool.Begin(ctx)
	if err != nil {
		return MapError(err)
	}

	c.activeTx = &Tx{
		pgxTx: pgxTx,
		db:    c.db,
		depth: 0,
	}
	return nil
}

func (c *Client) CommitTx(ctx context.Context) error {
	if c.activeTx == nil {
		return nil
	}
	err := c.activeTx.Commit(ctx)
	if err != nil {
		_ = c.activeTx.Rollback(ctx)
	}
	c.activeTx = nil
	return err
}

func (c *Client) RollbackTx(ctx context.Context) error {
	if c.activeTx == nil {
		return nil
	}
	err := c.activeTx.Rollback(ctx)
	c.activeTx = nil
	return err
}

func (c *Client) WithTx(ctx context.Context, fn func(tx *Tx) error) error {
	if err := c.StartTx(ctx); err != nil {
		return err
	}

	defer func() {
		if c.activeTx != nil {
			_ = c.RollbackTx(ctx)
		}
	}()

	if err := fn(c.activeTx); err != nil {
		return err
	}

	return c.CommitTx(ctx)
}

func (c *Client) One(ctx context.Context, q *query.Get, dest any) error {
	if c.activeTx != nil {
		return c.activeTx.One(ctx, q, dest)
	}
	res, err := c.db.execGet(ctx, c.db.pool, q, false)
	if err != nil {
		return err
	}
	return res.Scan(dest)
}

func (c *Client) Many(ctx context.Context, q *query.Get, dest any) (int64, error) {
	if c.activeTx != nil {
		return c.activeTx.Many(ctx, q, dest)
	}
	res, err := c.db.execGet(ctx, c.db.pool, q, true)
	if err != nil {
		return 0, err
	}
	if err := res.Scan(dest); err != nil {
		return 0, err
	}
	return res.Total, nil
}

func (c *Client) Count(ctx context.Context, q *query.Get) (int64, error) {
	if c.activeTx != nil {
		return c.activeTx.Count(ctx, q)
	}
	return c.db.count(ctx, c.db.pool, q)
}

func (c *Client) Insert(ctx context.Context, q *query.Insert) (string, error) {
	if c.activeTx != nil {
		return c.activeTx.Insert(ctx, q)
	}
	res, err := c.db.execWrite(ctx, c.db.pool, q)
	if err != nil {
		return "", err
	}
	return res.LastInsertID(), nil
}

func (c *Client) InsertMany(ctx context.Context, q *query.InsertMany) ([]string, error) {
	if c.activeTx != nil {
		return c.activeTx.InsertMany(ctx, q)
	}
	res, err := c.db.execWrite(ctx, c.db.pool, q)
	if err != nil {
		return nil, err
	}
	return res.LastInsertIDs(), nil
}

func (c *Client) Upsert(ctx context.Context, q *query.Upsert) (string, error) {
	if c.activeTx != nil {
		return c.activeTx.Upsert(ctx, q)
	}
	res, err := c.db.execWrite(ctx, c.db.pool, q)
	if err != nil {
		return "", err
	}
	return res.LastInsertID(), nil
}

func (c *Client) Update(ctx context.Context, q *query.Update) error {
	if c.activeTx != nil {
		return c.activeTx.Update(ctx, q)
	}
	_, err := c.db.execWrite(ctx, c.db.pool, q)
	return err
}

func (c *Client) Delete(ctx context.Context, q *query.Delete) error {
	if c.activeTx != nil {
		return c.activeTx.Delete(ctx, q)
	}
	_, err := c.db.execWrite(ctx, c.db.pool, q)
	return err
}

func (c *Client) Exec(ctx context.Context, q query.Query) (*query.Result, error) {
	if c.activeTx != nil {
		return c.activeTx.Exec(ctx, q)
	}
	return c.db.exec(ctx, c.db.pool, q)
}

func (c *Client) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if c.activeTx != nil {
		return c.activeTx.pgxTx.Query(ctx, sql, args...)
	}
	return c.db.pool.Query(ctx, sql, args...)
}
