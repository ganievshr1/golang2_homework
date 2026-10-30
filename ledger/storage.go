package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

// Connect вызывается один раз при старте,
// Close — после завершения запросов.
var db *sql.DB
var cache *redis.Client

const operationTimeout = 5 * time.Second

// Максимальная сумма в копейках для NUMERIC(14,2).
const maxMoney int64 = 99_999_999_999_999

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func databaseDSN() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}

	u := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			envOr("DB_USER", "postgres"),
			envOr("DB_PASS", "postgres"),
		),
		Host: net.JoinHostPort(
			envOr("DB_HOST", "localhost"),
			envOr("DB_PORT", "5432"),
		),
		Path: "/" + envOr("DB_NAME", "cashapp"),
	}

	query := u.Query()
	query.Set("sslmode", "disable")
	u.RawQuery = query.Encode()

	return u.String()
}

func Connect(ctx context.Context) error {
	if db != nil || cache != nil {
		return errors.New("Ledger уже подключён")
	}

	redisDB, err := strconv.Atoi(envOr("REDIS_DB", "0"))
	if err != nil || redisDB < 0 {
		return errors.New(
			"REDIS_DB должен быть целым неотрицательным числом",
		)
	}

	conn, err := sql.Open("pgx", databaseDSN())
	if err != nil {
		return fmt.Errorf(
			"PostgreSQL: открыть подключение: %w",
			err,
		)
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()

		return fmt.Errorf(
			"PostgreSQL недоступен; проверьте DATABASE_URL и запуск БД: %w",
			err,
		)
	}

	client := redis.NewClient(&redis.Options{
		Addr:                  envOr("REDIS_ADDR", "localhost:6379"),
		DB:                    redisDB,
		Password:              os.Getenv("REDIS_PASSWORD"),
		DialTimeout:           2 * time.Second,
		ReadTimeout:           time.Second,
		WriteTimeout:          time.Second,
		MaxRetries:            -1,
		ContextTimeoutEnabled: true,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		conn.Close()

		return fmt.Errorf(
			"Redis недоступен; проверьте REDIS_* и запуск Redis: %w",
			err,
		)
	}

	db, cache = conn, client

	log.Print("Ledger: подключение к PostgreSQL установлено")
	log.Print("Ledger: подключение к Redis установлено")

	return nil
}

func Close() error {
	var result error

	if cache != nil {
		result = errors.Join(result, cache.Close())
		cache = nil
	}

	if db != nil {
		result = errors.Join(result, db.Close())
		db = nil
	}

	return result
}

func database() (*sql.DB, error) {
	if db == nil {
		return nil, errors.New(
			"Ledger: сначала вызовите Connect",
		)
	}

	return db, nil
}

// AddTransaction и SetBudget используют один ключ категории.
// Блокировка действует до commit/rollback SQL-транзакции.
func lockCategory(
	ctx context.Context,
	tx *sql.Tx,
	category string,
) error {
	_, err := tx.ExecContext(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1, 0))",
		category,
	)

	return err
}
