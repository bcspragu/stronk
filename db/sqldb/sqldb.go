// Package sqldb implements the server.DB interface, backed by a sqlite database.
package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/lexacali/fivethreeone/fto"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	sql *sql.DB
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string) error {
	return errors.New("not implemented")
}

func New(dbPath, migrationsPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite DB: %w", err)
	}
	cleanupOnError := func(origErr error) error {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("error closing DB (%v) while handling original error: %w", closeErr, origErr)
		}
		return origErr
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{
		MigrationsTable: sqlite3.DefaultMigrationsTable,
	})
	if err != nil {
		return nil, cleanupOnError(fmt.Errorf("failed to init go-migrate driver: %w", err))
	}

	rootedMigrationsPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return nil, cleanupOnError(fmt.Errorf("failed to get a rooted migrations file path: %w", err))
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+rootedMigrationsPath,
		"sqlite3", driver)
	if err != nil {
		return nil, cleanupOnError(fmt.Errorf("failed to create migrate instance: %w", err))
	}

	prevV, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return nil, cleanupOnError(fmt.Errorf("failed to load current DB version: %w", err))
	}
	if dirty {
		return nil, cleanupOnError(errors.New("database was marked dirty"))
	}

	if err := m.Up(); err != nil {
		return nil, cleanupOnError(fmt.Errorf("failed to migrate database up: %w", err))
	}

	curV, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return nil, cleanupOnError(fmt.Errorf("failed to load DB version post-migration: %w", err))
	}
	if dirty {
		return nil, cleanupOnError(errors.New("database was marked dirty after migration"))
	}

	if prevV != curV {
		log.Printf("Migrated from version %d to version %d", prevV, curV)
	}

	return &DB{
		sql: db,
	}, nil
}
