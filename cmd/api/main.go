package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
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
	limiter struct {
		rps     float64
		burst   int
		enabled bool
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
	rps, rpsErr := strconv.ParseFloat(os.Getenv("RATE_LIMIT_RPS"), 64)
	if rpsErr != nil {
		logger.Error(rpsErr.Error())
		os.Exit(1)
	}
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", rps, "Rate limiter maximum requests per second")
	burst, burstErr := strconv.Atoi(os.Getenv("RATE_LIMIT_BURST"))
	if burstErr != nil {
		logger.Error(burstErr.Error())
		os.Exit(1)
	}
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", burst, "Rate limiter maximum burst")
	enabled, enabledErr := strconv.ParseBool(os.Getenv("RATE_LIMIT_ENABLED"))
	if enabledErr != nil {
		logger.Error(enabledErr.Error())
		os.Exit(1)
	}
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", enabled, "Enabled rate limiter")
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
	err := app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
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
