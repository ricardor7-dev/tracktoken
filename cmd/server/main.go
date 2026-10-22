package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"snsgo/internal/handler"
)

type config struct {
	addr            string
	workers         int
	supplierTimeout time.Duration
	requestTimeout  time.Duration
}

// loadConfig reads settings with precedence: flag > environment (.env included) > default.
func loadConfig() (config, error) {
	// Variables already set in the environment win over .env; a missing .env is fine.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return config{}, fmt.Errorf("loading .env: %w", err)
	}

	addr := envString("ADDR", ":8080")
	// Default to the CPUs available to the process (respects container limits since Go 1.25).
	workers, err := envInt("WORKERS", runtime.GOMAXPROCS(0))
	if err != nil {
		return config{}, err
	}
	supplierTimeout, err := envDuration("SUPPLIER_TIMEOUT", 2*time.Second)
	if err != nil {
		return config{}, err
	}
	requestTimeout, err := envDuration("REQUEST_TIMEOUT", 3*time.Second)
	if err != nil {
		return config{}, err
	}

	var cfg config
	flag.StringVar(&cfg.addr, "addr", addr, "HTTP listen address (env ADDR)")
	flag.IntVar(&cfg.workers, "workers", workers, "max concurrent supplier calls (env WORKERS)")
	flag.DurationVar(&cfg.supplierTimeout, "supplier-timeout", supplierTimeout, "timeout per supplier call (env SUPPLIER_TIMEOUT)")
	flag.DurationVar(&cfg.requestTimeout, "request-timeout", requestTimeout, "timeout per /price request (env REQUEST_TIMEOUT)")
	flag.Parse()

	if cfg.workers < 1 {
		return config{}, fmt.Errorf("workers must be at least 1, got %d", cfg.workers)
	}
	if cfg.supplierTimeout <= 0 || cfg.requestTimeout <= 0 {
		return config{}, errors.New("timeouts must be positive")
	}
	return cfg, nil
}

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           handler.New(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (workers=%d, supplier-timeout=%s, request-timeout=%s)",
			cfg.addr, cfg.workers, cfg.supplierTimeout, cfg.requestTimeout)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
