package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/basemachina/bridge"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	serviceName string
	version     string
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	container, cleanup, err := BridgeContainerProvider()
	if err != nil {
		return fmt.Errorf("failed to injects some containers: %w", err)
	}
	defer cleanup()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	l := container.Logger

	fw := container.FetchWorker
	fw.StartWorker()

	l.Info("worker is started and waiting for ready...")
	if err := fw.WaitForReady(ctx); err != nil {
		return err
	}
	l.Info("worker is ready")

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		l.Info("http server is booting...")
		defer l.Info("finished running http server")
		err := container.HTTPServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	<-ctx.Done()
	cleanup()

	return eg.Wait()
}

func NewLogger(env *bridge.Env) (logr.Logger, func(), error) {
	logging, err := NewStackdriver(env.LogLevel)
	if err != nil {
		return logr.Logger{}, nil, err
	}
	cleanup := func() {
		logging.Sync()
	}
	return zapr.NewLogger(
		logging.
			Named(serviceName).
			With(zap.String("version", version)),
	), cleanup, nil
}
