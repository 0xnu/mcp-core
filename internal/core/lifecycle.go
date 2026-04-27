package core

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"
)

type Lifecycle struct {
	server   *Server
	router   *Router
	registry interface {
		StartAll(context.Context) error
		StopAll() error
		HealthCheckAndRestart()
	}
}

func NewLifecycle(server *Server, router *Router, reg interface {
	StartAll(context.Context) error
	StopAll() error
	HealthCheckAndRestart()
},
) *Lifecycle {
	return &Lifecycle{
		server:   server,
		router:   router,
		registry: reg,
	}
}

func (l *Lifecycle) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := l.registry.StartAll(ctx); err != nil {
		return err
	}

	l.router.RebuildToolIndex()

	go l.periodicHealthCheck(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, shutdownSignals()...)

	go func() {
		<-sigCh
		log.Printf("signal received, initiating graceful shutdown...")
		cancel()
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := l.server.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck // ctx already Done, must use Background
			log.Printf("server shutdown error: %v", err)
		}

		if err := l.registry.StopAll(); err != nil {
			log.Printf("registry shutdown error: %v", err)
		}
	}()

	return l.server.Start()
}

func (l *Lifecycle) periodicHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.registry.HealthCheckAndRestart()
		}
	}
}
