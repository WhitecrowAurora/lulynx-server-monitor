package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WhitecrowAurora/lulynx-server-monitor/internal/agent"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "probe.json", "Path to probe config JSON")
	flag.Parse()

	cfg, err := agent.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if cfg.IngestToken == "" && cfg.EnrollToken != "" {
		enrollCtx, cancelEnroll := context.WithTimeout(ctx, 15*time.Second)
		cfg2, err := agent.EnrollAndPersist(enrollCtx, configPath, cfg)
		cancelEnroll()
		if err != nil {
			fmt.Fprintln(os.Stderr, "enroll:", err)
			os.Exit(1)
		}
		cfg = cfg2
	}

	if err := agent.Run(ctx, cfg); err != nil && err != context.Canceled {
		fmt.Fprintln(os.Stderr, "probe:", err)
		os.Exit(1)
	}
}
