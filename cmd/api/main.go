package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"greenlight.gustavosantos.net/internal/data"
	"greenlight.gustavosantos.net/internal/mailer"
	"greenlight.gustavosantos.net/internal/vcs"
)

var (
	version = vcs.Version()
)

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
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
	jwt struct {
		secret string
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	loadEnvErr := godotenv.Load(`/home/gustavo/code/estudo/go/lets-go-further/.env`)
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
	flag.StringVar(&cfg.smtp.host, "smtp-host", os.Getenv("SMTP_HOST"), "SMTP host")
	smtpPort, smtpPortErr := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if smtpPortErr != nil {
		logger.Error(smtpPortErr.Error())
		os.Exit(1)
	}
	flag.IntVar(
		&cfg.smtp.port,
		"smtp-port",
		smtpPort,
		"SMTP port",
	)
	flag.StringVar(&cfg.smtp.username, "smtp-username", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", os.Getenv("SMTP_SENDER"), "SMTP sender")
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})
	flag.StringVar(&cfg.jwt.secret, "jwt-secret", "", "JWT secret")
	displayVersion := flag.Bool("version", false, "Display version and exit")
	flag.Parse()
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}
	db, openErr := openDB(cfg)
	if openErr != nil {
		logger.Error(openErr.Error())
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("Database connection pool established")
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
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
		db.Close()
		return nil, pingErr
	}
	return db, nil
}
