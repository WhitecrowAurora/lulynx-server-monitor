package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/center"
)

//go:embed web/*
var webFS embed.FS

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "center.json", "Path to center config JSON")
	flag.Parse()

	cfg, err := center.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	svc, err := center.NewService(cfg, webFS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "center init:", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           svc.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		svc.Close()
	}()

	fmt.Println("center listening on", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
}
