package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"test_minio/bucket"
	"test_minio/client"
	"test_minio/config"
	"test_minio/handler"
)

func Run() error {
	ctx, cancel := initContext()
	defer cancel()

	cfg, err := config.Get()
	if err != nil {
		return err
	}

	minioClient, err := client.Create(*cfg)
	if err != nil {
		return err
	}

	err = bucket.Create(ctx, minioClient, cfg.BucketName, cfg.Location)
	if err != nil {
		return err
	}

	s := handler.NewServer(ctx, minioClient, cfg.BucketName)

	go startHTTPServer(s.HTTPServer, s.HTTPServer.Addr)

	return shutdownServer(s)

}

func initContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func startHTTPServer(server *http.Server, port string) {
	slog.Info(fmt.Sprintf("starting HTTP server on port %s", port[1:]))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("error starting HTTP server", "error", err)
	}
}

func shutdownServer(s *handler.Server) error {
	server := s.HTTPServer
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)

	select {
	case <-shutdownSignals:
		slog.Info("received shutdown signal")
	case <-s.Ctx.Done():
		slog.Info("context deadline exceeded")
	}

	if err := server.Shutdown(s.Ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)

		if err := server.Close(); err != nil {
			slog.Error("forced shutdown failed", "error", err)
			return err
		}
	}

	slog.Info("server shutdown complete")
	return nil
}
