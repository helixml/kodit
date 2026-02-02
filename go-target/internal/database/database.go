// Package database provides database connection and session management using GORM.
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ErrUnsupportedDriver indicates the database URL uses an unsupported driver.
var ErrUnsupportedDriver = errors.New("unsupported database driver")

// Database wraps a GORM connection with lifecycle management.
type Database struct {
	db *gorm.DB
}

// NewDatabase creates a new Database from a connection URL.
// Supported URL formats:
// - sqlite:///path/to/file.db
// - postgresql://user:pass@host:port/dbname
// - postgres://user:pass@host:port/dbname
func NewDatabase(ctx context.Context, url string) (Database, error) {
	dialector, err := parseDialector(url)
	if err != nil {
		return Database{}, fmt.Errorf("parse database url: %w", err)
	}

	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(dialector, config)
	if err != nil {
		return Database{}, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return Database{}, fmt.Errorf("get underlying db: %w", err)
	}

	// Verify connection
	if err := sqlDB.PingContext(ctx); err != nil {
		return Database{}, fmt.Errorf("ping database: %w", err)
	}

	return Database{db: db}, nil
}

// NewDatabaseWithConfig creates a Database with custom GORM configuration.
func NewDatabaseWithConfig(ctx context.Context, url string, config *gorm.Config) (Database, error) {
	dialector, err := parseDialector(url)
	if err != nil {
		return Database{}, fmt.Errorf("parse database url: %w", err)
	}

	db, err := gorm.Open(dialector, config)
	if err != nil {
		return Database{}, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return Database{}, fmt.Errorf("get underlying db: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return Database{}, fmt.Errorf("ping database: %w", err)
	}

	return Database{db: db}, nil
}

// Session returns a GORM session with the given context.
func (d Database) Session(ctx context.Context) *gorm.DB {
	return d.db.WithContext(ctx)
}

// Close closes the database connection.
func (d Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}
	return sqlDB.Close()
}

// ConfigurePool sets connection pool parameters.
func (d Database) ConfigurePool(maxOpen, maxIdle int, maxLifetime time.Duration) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(maxLifetime)
	return nil
}

// IsPostgres returns true if the underlying database is PostgreSQL.
func (d Database) IsPostgres() bool {
	return d.db.Name() == "postgres"
}

// IsSQLite returns true if the underlying database is SQLite.
func (d Database) IsSQLite() bool {
	return d.db.Name() == "sqlite"
}

func parseDialector(url string) (gorm.Dialector, error) {
	switch {
	case strings.HasPrefix(url, "sqlite:///"):
		path := strings.TrimPrefix(url, "sqlite:///")
		return sqlite.Open(path), nil
	case strings.HasPrefix(url, "postgresql://"), strings.HasPrefix(url, "postgres://"):
		return postgres.Open(url), nil
	default:
		return nil, ErrUnsupportedDriver
	}
}
