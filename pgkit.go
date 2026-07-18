package pgkit

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nrednav/cuid2"
	"github.com/retailancer/pgkit/internal/builder"
	"github.com/retailancer/pgkit/internal/identifier"
	"github.com/retailancer/pgkit/internal/sqlutil"
	"github.com/retailancer/pgkit/query"
	"github.com/retailancer/pgkit/rows"
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type defaultLogger struct {
	l *slog.Logger
}

func (d *defaultLogger) Info(msg string, args ...any) {
	d.l.Info(msg, args...)
}

func (d *defaultLogger) Error(msg string, args ...any) {
	d.l.Error(msg, args...)
}

type Options struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	Schema            string
	SoftDeleteColumn  string
	IDGenerator       identifier.Generator
	AutoUpdatedAt     bool
	Logger            Logger
}

type DB struct {
	pool    *pgxpool.Pool
	opts    Options
	mu      sync.RWMutex
	clients map[string]*Client
}

type dbRunner interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func Bool(v bool) *bool {
	return &v
}

func New(ctx context.Context, dsn string, opts Options) (*DB, error) {
	if opts.Schema == "" {
		opts.Schema = "public"
	}
	if opts.IDGenerator == nil {
		opts.IDGenerator = identifier.NewIgnoreGenerator()
	}
	if opts.Logger == nil {
		opts.Logger = &defaultLogger{l: slog.Default()}
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgkit: failed to parse DSN: %w", err)
	}

	if opts.MaxConns > 0 {
		config.MaxConns = opts.MaxConns
	}
	if opts.MinConns > 0 {
		config.MinConns = opts.MinConns
	}
	if opts.MaxConnLifetime > 0 {
		config.MaxConnLifetime = opts.MaxConnLifetime
	} else {
		config.MaxConnLifetime = 5 * time.Minute
	}
	if opts.MaxConnIdleTime > 0 {
		config.MaxConnIdleTime = opts.MaxConnIdleTime
	}
	if opts.HealthCheckPeriod > 0 {
		config.HealthCheckPeriod = opts.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, MapError(err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgkit: connection test failed: %w", err)
	}

	return &DB{
		pool:    pool,
		opts:    opts,
		clients: make(map[string]*Client),
	}, nil
}

// Close closes the DB client and releases all resources.
func (db *DB) Close() error {
	db.mu.Lock()
	clientsToClose := make([]*Client, 0, len(db.clients))
	for _, cl := range db.clients {
		clientsToClose = append(clientsToClose, cl)
	}
	db.mu.Unlock()

	for _, cl := range clientsToClose {
		_ = cl.Close()
	}

	db.pool.Close()
	return nil
}

// Client creates a Client instance to interface with the database.
func (db *DB) Client() *Client {
	cl := &Client{
		id: cuid2.Generate(),
		db: db,
	}
	db.mu.Lock()
	db.clients[cl.id] = cl
	db.mu.Unlock()
	return cl
}

// Listen creates a Listener subscribing to a PostgreSQL NOTIFY channel.
func (db *DB) Listen(ctx context.Context, channel string) (*Listener, error) {
	return NewListener(ctx, db.pool.Config().ConnConfig, channel)
}

// Exec executes raw SQL directly against the connection pool.
func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := db.pool.Exec(ctx, sql, args...)
	return tag, MapError(err)
}

// WithTx runs the function fn inside a transaction.
func (db *DB) WithTx(ctx context.Context, fn func(tx *Tx) error) error {
	client := db.Client()
	defer client.Close()

	if err := client.StartTx(ctx); err != nil {
		return err
	}

	defer func() {
		if client.activeTx != nil {
			_ = client.RollbackTx(ctx)
		}
	}()

	if err := fn(client.activeTx); err != nil {
		return err
	}

	return client.CommitTx(ctx)
}

func (db *DB) execGet(ctx context.Context, runner dbRunner, q *query.Get, many bool) (*query.Result, error) {
	pt := &builder.ParamTracker{}
	sqlStr, err := builder.Build(q, pt, db.opts.Schema, db.opts.SoftDeleteColumn, db.opts.AutoUpdatedAt)
	if err != nil {
		return nil, err
	}

	if q.Log && db.opts.Logger != nil {
		db.opts.Logger.Info("pgkit: executing query", "sql", sqlutil.Interpolate(sqlStr, pt.Params))
	}

	pgRows, err := runner.Query(ctx, sqlStr, pt.Params...)
	if err != nil {
		return nil, MapError(err)
	}
	defer pgRows.Close()

	data, err := rows.ScanRows(pgRows)
	if err != nil {
		return nil, MapError(err)
	}

	data = rows.NestRelations(data)

	hasManyInclude := false
	for _, inc := range q.Include {
		if inc.Many {
			hasManyInclude = true
			break
		}
	}

	if many || hasManyInclude {
		var manyAliases []string
		for _, inc := range q.Include {
			if inc.Many {
				alias := inc.Alias
				if alias == "" {
					alias = inc.From
				}
				manyAliases = append(manyAliases, alias)
			}
		}
		data = rows.AggregateRows(data, manyAliases)
	}

	result := &query.Result{}
	if many {
		result.Data = data

		if !q.SkipCount {
			total, err := db.count(ctx, runner, q)
			if err != nil {
				return nil, err
			}
			result.Total = total
		}
	} else {
		if len(data) == 0 {
			return nil, ErrNotFound
		}
		if len(data) > 1 {
			return nil, ErrMultipleRecords
		}
		result.Data = data[0]
	}

	return result, nil
}

func (db *DB) autoGenerateID(data map[string]any) map[string]any {
	if _, exists := data["id"]; exists {
		return data
	}
	if id := db.opts.IDGenerator.Generate(); id != "" {
		if data == nil {
			data = make(map[string]any)
		}
		data["id"] = id
	}
	return data
}

func (db *DB) execWrite(ctx context.Context, runner dbRunner, q query.Query) (*query.Result, error) {
	var manyIDs []string
	switch queryVal := q.(type) {
	case *query.Insert:
		queryVal.Data = db.autoGenerateID(queryVal.Data)
	case *query.Upsert:
		queryVal.Data = db.autoGenerateID(queryVal.Data)
	case *query.InsertMany:
		if len(queryVal.Values) > 0 {
			firstID := db.opts.IDGenerator.Generate()
			if firstID != "" {
				hasID := false
				idIdx := -1
				for idx, colName := range queryVal.Fields {
					if colName == "id" {
						hasID = true
						idIdx = idx
						break
					}
				}
				if !hasID {
					queryVal.Fields = append(queryVal.Fields, "id")
				}

				manyIDs = make([]string, len(queryVal.Values))
				for i, row := range queryVal.Values {
					var id string
					if i == 0 {
						id = firstID
					} else {
						id = db.opts.IDGenerator.Generate()
					}
					manyIDs[i] = id
					if !hasID {
						queryVal.Values[i] = append(row, id)
					} else {
						if idIdx < len(row) {
							if row[idIdx] != nil {
								manyIDs[i] = fmt.Sprintf("%v", row[idIdx])
							} else {
								queryVal.Values[i][idIdx] = id
							}
						}
					}
				}
			}
		}
	}

	pt := &builder.ParamTracker{}
	sqlStr, err := builder.Build(q, pt, db.opts.Schema, db.opts.SoftDeleteColumn, db.opts.AutoUpdatedAt)
	if err != nil {
		return nil, err
	}

	rowsList, err := runner.Query(ctx, sqlStr, pt.Params...)
	if err != nil {
		return nil, MapError(err)
	}
	defer rowsList.Close()

	data, err := rows.ScanRows(rowsList)
	if err != nil {
		return nil, MapError(err)
	}

	result := &query.Result{}
	switch qType := q.(type) {
	case *query.Insert:
		if len(data) > 0 {
			result.Data = data[0]
		} else {
			result.Data = map[string]any{"id": qType.Data["id"]}
		}
	case *query.InsertMany:
		if db.opts.IDGenerator.Generate() == "" {
			manyIDs = make([]string, len(data))
			for i, rowMap := range data {
				if idVal, exists := rowMap["id"]; exists && idVal != nil {
					manyIDs[i] = fmt.Sprintf("%v", idVal)
				}
			}
		}
		result.Data = map[string]any{"ids": manyIDs}
	default:
		if len(data) > 0 {
			result.Data = data[0]
		}
	}

	return result, nil
}

func (db *DB) exec(ctx context.Context, runner dbRunner, q query.Query) (*query.Result, error) {
	switch queryVal := q.(type) {
	case *query.Get:
		return db.execGet(ctx, runner, queryVal, false)
	case *query.Aggregate:
		pt := &builder.ParamTracker{}
		sqlStr, err := builder.Build(q, pt, db.opts.Schema, db.opts.SoftDeleteColumn, db.opts.AutoUpdatedAt)
		if err != nil {
			return nil, err
		}
		rowsList, err := runner.Query(ctx, sqlStr, pt.Params...)
		if err != nil {
			return nil, MapError(err)
		}
		defer rowsList.Close()

		data, err := rows.ScanRows(rowsList)
		if err != nil {
			return nil, MapError(err)
		}
		data = rows.NestRelations(data)
		return &query.Result{Data: data}, nil
	default:
		return db.execWrite(ctx, runner, q)
	}
}

func copyGetQuery(q *query.Get) *query.Get {
	copyQ := *q
	if q.Selection != nil {
		copyQ.Selection = make([]string, len(q.Selection))
		copy(copyQ.Selection, q.Selection)
	}
	if q.DistinctOn != nil {
		copyQ.DistinctOn = make([]string, len(q.DistinctOn))
		copy(copyQ.DistinctOn, q.DistinctOn)
	}
	if q.GroupBy != nil {
		copyQ.GroupBy = make([]string, len(q.GroupBy))
		copy(copyQ.GroupBy, q.GroupBy)
	}
	if q.Include != nil {
		copyQ.Include = make([]query.Join, len(q.Include))
		copy(copyQ.Include, q.Include)
	}
	if q.Types != nil {
		copyQ.Types = make(map[string]string)
		maps.Copy(copyQ.Types, q.Types)
	}
	if q.Order != nil {
		copyQ.Order = make(map[string]string)
		maps.Copy(copyQ.Order, q.Order)
	}
	return &copyQ
}

func (db *DB) count(ctx context.Context, runner dbRunner, q *query.Get) (int64, error) {
	countQ := copyGetQuery(q)
	countQ.Limit = 0
	countQ.Offset = 0
	countQ.ForUpdate = false
	ptCount := &builder.ParamTracker{}
	countSQL, err := builder.Build(countQ, ptCount, db.opts.Schema, db.opts.SoftDeleteColumn, db.opts.AutoUpdatedAt)
	if err != nil {
		return 0, err
	}
	wrappedCountSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS _pgkit_count", countSQL)
	var total int64
	err = runner.QueryRow(ctx, wrappedCountSQL, ptCount.Params...).Scan(&total)
	return total, MapError(err)
}
