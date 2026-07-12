package pgkit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

type Notification struct {
	Channel string
	Payload string
}

type Listener struct {
	connConfig *pgx.ConnConfig
	channel    string
	ch         chan Notification
	done       chan struct{}
	closeOnce  sync.Once
	conn       *pgx.Conn
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewListener(ctx context.Context, connConfig *pgx.ConnConfig, channel string) (*Listener, error) {
	cfg := connConfig.Copy()

	listenCtx, cancel := context.WithCancel(context.Background())

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		cancel()
		return nil, MapError(err)
	}

	_, err = conn.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize())
	if err != nil {
		_ = conn.Close(ctx)
		cancel()
		return nil, MapError(err)
	}

	l := &Listener{
		connConfig: cfg,
		channel:    channel,
		ch:         make(chan Notification, 100),
		done:       make(chan struct{}),
		conn:       conn,
		ctx:        listenCtx,
		cancel:     cancel,
	}

	go l.listenLoop()

	return l, nil
}

func (l *Listener) C() <-chan Notification {
	return l.ch
}

func (l *Listener) Close() error {
	var err error
	l.closeOnce.Do(func() {
		l.cancel()
		close(l.done)
		l.mu.Lock()
		if l.conn != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = l.conn.Exec(ctx, "UNLISTEN "+pgx.Identifier{l.channel}.Sanitize())
			err = l.conn.Close(ctx)
		}
		l.mu.Unlock()
	})
	return err
}

func (l *Listener) listenLoop() {
	defer close(l.ch)
	for {
		select {
		case <-l.done:
			return
		default:
			l.mu.Lock()
			conn := l.conn
			l.mu.Unlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				if err := l.reconnect(); err != nil {
					continue
				}
				continue
			}

			notification, err := conn.WaitForNotification(l.ctx)
			if err != nil {
				select {
				case <-l.done:
					return
				default:
				}
				// connection lost, close and trigger reconnect
				l.mu.Lock()
				if l.conn != nil {
					_ = l.conn.Close(context.Background())
					l.conn = nil
				}
				l.mu.Unlock()
				continue
			}

			if notification != nil {
				select {
				case l.ch <- Notification{Channel: notification.Channel, Payload: notification.Payload}:
				case <-l.done:
					return
				}
			}
		}
	}
}

func (l *Listener) reconnect() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	select {
	case <-l.done:
		return errors.New("listener closed")
	default:
	}

	conn, err := pgx.ConnectConfig(l.ctx, l.connConfig)
	if err != nil {
		return err
	}

	_, err = conn.Exec(l.ctx, "LISTEN "+pgx.Identifier{l.channel}.Sanitize())
	if err != nil {
		_ = conn.Close(l.ctx)
		return err
	}

	l.conn = conn
	return nil
}
