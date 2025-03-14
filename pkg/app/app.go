package app

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// AppInfo is application context value.
type AppInfo interface {
	ID() string
	Name() string
	Version() string
}

// App is an application components lifecycle manager.
type App struct {
	opts   options
	ctx    context.Context
	cancel context.CancelFunc
}

// New create an application lifecycle manager.
func New(opts ...Option) *App {
	o := options{
		ctx:         context.Background(),
		sigs:        []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
		stopTimeout: 10 * time.Second,
	}
	if id, err := uuid.NewUUID(); err == nil {
		o.id = id.String()
	}
	for _, opt := range opts {
		opt(&o)
	}
	ctx, cancel := context.WithCancel(o.ctx)
	return &App{
		ctx:    ctx,
		cancel: cancel,
		opts:   o,
	}
}

// ID returns app instance id.
func (a *App) ID() string { return a.opts.id }

// Name returns service name.
func (a *App) Name() string { return a.opts.name }

// Version returns app version.
func (a *App) Version() string { return a.opts.version }

// Run executes all OnStart hooks registered with the application's Lifecycle.
func (a *App) Run() error {
	var err error
	sctx := NewContext(a.ctx, a)
	eg, ctx := errgroup.WithContext(sctx)
	wg := sync.WaitGroup{}

	for _, fn := range a.opts.beforeStart {
		if err = fn(sctx); err != nil {
			return err
		}
	}
	for _, srv := range a.opts.servers {
		srv := srv
		eg.Go(func() error {
			<-ctx.Done() // wait for stop signal
			stopCtx, cancel := context.WithTimeout(NewContext(a.opts.ctx, a), a.opts.stopTimeout)
			defer cancel()
			return srv.Stop(stopCtx)
		})
		wg.Add(1)
		eg.Go(func() error {
			wg.Done() // here is to ensure server start has begun running before register, so defer is not needed
			return srv.Start(NewContext(a.opts.ctx, a))
		})
	}
	wg.Wait()
	for _, fn := range a.opts.afterStart {
		if err = fn(sctx); err != nil {
			return err
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, a.opts.sigs...)
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return nil
		case <-c:
			return a.Stop()
		}
	})
	if err = eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	err = nil
	for _, fn := range a.opts.afterStop {
		err = fn(sctx)
	}
	return err
}

// Stop gracefully stops the application.
func (a *App) Stop() (err error) {
	sctx := NewContext(a.ctx, a)
	for _, fn := range a.opts.beforeStop {
		err = fn(sctx)
	}
	if a.cancel != nil {
		a.cancel()
	}
	return err
}

type appKey struct{}

// NewContext returns a new Context that carries value.
func NewContext(ctx context.Context, s AppInfo) context.Context {
	return context.WithValue(ctx, appKey{}, s)
}

// FromContext returns the Transport value stored in ctx, if any.
func FromContext(ctx context.Context) (s AppInfo, ok bool) {
	s, ok = ctx.Value(appKey{}).(AppInfo)
	return
}
