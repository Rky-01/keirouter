// Command keirouter is the KeiRouter server entrypoint. It loads configuration,
// builds the application, and serves until interrupted.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mydisha/keirouter/backend/internal/app"
	"github.com/mydisha/keirouter/backend/internal/config"
)

// Version is set at build time via -ldflags "-X main.Version=...".
// Defaults to "dev" for local builds.
var Version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "keirouter:", err)
		os.Exit(1)
	}
}

func run() error {
	showVersion := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "", "path to a YAML config file (optional)")
	bootstrap := flag.Bool("bootstrap", false, "create an initial API key and exit")
	healthcheck := flag.Bool("healthcheck", false, "check the local HTTP health endpoint and exit")
	keyName := flag.String("key-name", "default", "name for the bootstrapped API key")
	flag.Parse()

	if *showVersion {
		fmt.Println("keirouter", Version)
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := newLogger(cfg.Log)
	slog.SetDefault(log)

	if *healthcheck {
		return runHealthcheck(cfg)
	}

	// Cancel on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *bootstrap {
		plaintext, berr := app.Bootstrap(ctx, cfg, *keyName)
		if berr != nil {
			return berr
		}
		fmt.Println("Created API key (copy it now, it will not be shown again):")
		fmt.Println(plaintext)
		return nil
	}

	application, err := app.Build(ctx, cfg, log, Version)
	if err != nil {
		return fmt.Errorf("build app: %w", err)
	}

	return application.Run(ctx)
}

func runHealthcheck(cfg config.Config) error {
	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s:%d/healthz", host, cfg.Server.Port)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("healthcheck %s: %s", url, resp.Status)
	}
	return nil
}

// newLogger builds a structured logger per config.
func newLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
