// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds environment-driven database configuration.
type Config struct {
	// Either full DSN or individual fields below
	DSN string

	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	LogLevel logger.LogLevel
}

// FromEnv reads configuration from environment variables.
// Supported variables:
//   - DB_DSN (preferred)
//   - DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
//   - DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS
//   - DB_CONN_MAX_LIFETIME, DB_CONN_MAX_IDLE_TIME (duration, e.g. 30m, 1h)
//   - DB_LOG_LEVEL (silent|error|warn|info)
func FromEnv() Config {
	cfg := Config{
		DSN:      os.Getenv("DB_DSN"),
		Host:     getenvDefault("DB_HOST", "localhost"),
		Port:     atoiDefault(os.Getenv("DB_PORT"), 5432),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  getenvDefault("DB_SSLMODE", "disable"),

		MaxOpenConns:    atoiDefault(os.Getenv("DB_MAX_OPEN_CONNS"), 10),
		MaxIdleConns:    atoiDefault(os.Getenv("DB_MAX_IDLE_CONNS"), 5),
		ConnMaxLifetime: durationDefault(os.Getenv("DB_CONN_MAX_LIFETIME"), 30*time.Minute),
		ConnMaxIdleTime: durationDefault(os.Getenv("DB_CONN_MAX_IDLE_TIME"), 10*time.Minute),
		LogLevel:        parseLogLevel(os.Getenv("DB_LOG_LEVEL")),
	}
	return cfg
}

// FromConfigAndEnv merges a file-based Database config with environment variables (env wins).
func FromConfigAndEnv(c *struct {
	DSN             string
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        string
}) Config {
	if c == nil {
		env := FromEnv()
		return env
	}
	// start with file values
	cfg := Config{
		DSN:             c.DSN,
		Host:            c.Host,
		Port:            c.Port,
		User:            c.User,
		Password:        c.Password,
		DBName:          c.Name,
		SSLMode:         c.SSLMode,
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		ConnMaxIdleTime: c.ConnMaxIdleTime,
		LogLevel:        parseLogLevel(c.LogLevel),
	}
	// overlay env
	env := FromEnv()
	if env.DSN != "" {
		cfg.DSN = env.DSN
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Host = env.Host
	}
	if os.Getenv("DB_PORT") != "" {
		cfg.Port = env.Port
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.User = env.User
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Password = env.Password
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = env.DBName
	}
	if v := os.Getenv("DB_SSLMODE"); v != "" {
		cfg.SSLMode = env.SSLMode
	}
	if os.Getenv("DB_MAX_OPEN_CONNS") != "" {
		cfg.MaxOpenConns = env.MaxOpenConns
	}
	if os.Getenv("DB_MAX_IDLE_CONNS") != "" {
		cfg.MaxIdleConns = env.MaxIdleConns
	}
	if os.Getenv("DB_CONN_MAX_LIFETIME") != "" {
		cfg.ConnMaxLifetime = env.ConnMaxLifetime
	}
	if os.Getenv("DB_CONN_MAX_IDLE_TIME") != "" {
		cfg.ConnMaxIdleTime = env.ConnMaxIdleTime
	}
	if v := os.Getenv("DB_LOG_LEVEL"); v != "" {
		cfg.LogLevel = env.LogLevel
	}
	return cfg
}

// Open creates a *gorm.DB using Postgres driver and applies pool settings.
func Open(cfg Config) (*gorm.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		// Build DSN from parts
		// Example: host=localhost port=5432 user=postgres password=secret dbname=chorus sslmode=disable TimeZone=UTC
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
		)
	}

	gcfg := &gorm.Config{Logger: logger.Default.LogMode(cfg.LogLevel)}
	gdb, err := gorm.Open(postgres.Open(dsn), gcfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	return gdb, nil
}

// Ping checks the database connectivity with a timeout context.
func Ping(ctx context.Context, gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	type pinger interface{ PingContext(context.Context) error }
	if db, ok := any(sqlDB).(pinger); ok {
		return db.PingContext(ctx)
	}
	return sqlDB.Ping()
}

// Close closes the underlying sql.DB.
func Close(gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func durationDefault(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func parseLogLevel(s string) logger.LogLevel {
	switch s {
	case "silent", "SILENT":
		return logger.Silent
	case "error", "ERROR":
		return logger.Error
	case "warn", "WARN", "warning", "WARNING":
		return logger.Warn
	case "info", "INFO":
		return logger.Info
	default:
		return logger.Error
	}
}
