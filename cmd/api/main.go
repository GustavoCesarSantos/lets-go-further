package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"greenlight.gustavosantos.net/internal/data"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
}

type application struct {
	config config
	logger *slog.Logger
    models data.Models
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	loadEnvErr := godotenv.Load(".env")
	if loadEnvErr != nil {
		logger.Error(loadEnvErr.Error())
		os.Exit(1)
	}
	var cfg config
	flag.IntVar(
		&cfg.port, 
		"port", 
		4000, 
		"API server port",
	)
	flag.StringVar(
		&cfg.env, 
		"env", 
		"development", 
		"Environment (development|staging|production)",
	)
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")
	maxOpenConns, maxOpenConnsErr := strconv.Atoi(os.Getenv("GREENLIGHT_DB_MAX_OPEN_CONNS"))
	if maxOpenConnsErr != nil {
		logger.Error(maxOpenConnsErr.Error())
		os.Exit(1)
	}
	flag.IntVar(
		&cfg.db.maxOpenConns,
		"db-max-open-conns",
		maxOpenConns,
		"PostgreSQL max open connections",
	)
	maxIdleConns, maxIdleConnsErr := strconv.Atoi(os.Getenv("GREENLIGHT_DB_MAX_IDLE_CONNS"))
	if maxIdleConnsErr != nil {
		logger.Error(maxIdleConnsErr.Error())
		os.Exit(1)
	}
	flag.IntVar(
		&cfg.db.maxIdleConns, 
		"db-max-idle-conns", 
		maxIdleConns, 
		"PostgreSQL max idle connections",
	)
	maxIdleTime, maxIdleTimeErr := time.ParseDuration(os.Getenv("GREENLIGHT_DB_MAX_IDLE_TIME"))
	if maxIdleTimeErr != nil {
		logger.Error(maxIdleTimeErr.Error())
		os.Exit(1)
	}
	flag.DurationVar(
		&cfg.db.maxIdleTime, 
		"db-max-idle-time", 
		maxIdleTime, 
		"PostgreSQL max connection idle time",
	)
	flag.Parse()
	db, openErr := openDB(cfg)
	if openErr != nil {
		logger.Error(openErr.Error())
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("Database connection pool established")
	app := &application{
		config: cfg,
		logger: logger,
        models: data.NewModels(db),
	}
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	logger.Info("starting server", "addr", srv.Addr, "env", cfg.env)
	err := srv.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pingErr := db.PingContext(ctx)
	if pingErr != nil {
		return nil, pingErr
	}
	return db, nil
}
