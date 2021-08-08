// Package sqldb implements the server.DB interface, backed by a sqlite database.
package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	"github.com/lexacali/fivethreeone/fto"
	"github.com/mattn/go-sqlite3"

	migratesqlite3 "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type DB struct {
	mu          sync.Mutex
	sql         *sql.DB
	mainLiftIDs map[fto.Exercise]int
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) CreateUser(name string) (fto.UserID, error) {
	err := db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO users (name) VALUES (?)`
		if _, err := tx.Exec(q, name); err != nil {
			return fmt.Errorf("failed to insert user: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	u, err := db.UserByName(name)
	if err != nil {
		return 0, fmt.Errorf("failed to load just created user: %w", err)
	}
	return u.ID, nil
}

func (db *DB) User(id fto.UserID) (*fto.User, error) {
	var u fto.User
	err := db.transact(func(tx *sql.Tx) error {
		q := `SELECT id, name
FROM Users
WHERE id = ?`
		return db.user(&u, tx.QueryRow(q, id))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load user: %w", err)
	}
	return &u, nil
}

func (db *DB) UserByName(name string) (*fto.User, error) {
	var u fto.User
	err := db.transact(func(tx *sql.Tx) error {
		q := `SELECT id, name
FROM Users
WHERE name = ?`
		return db.user(&u, tx.QueryRow(q, name))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load user: %w", err)
	}
	return &u, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (db *DB) user(u *fto.User, sc scanner) error {
	err := sc.Scan(&u.ID, &u.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return fto.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to query user: %w", err)
	}

	return nil
}

func (db *DB) RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int) error {
	return db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO lifts
(user_id, exercise, set_type, set_number, weight, day_number, week_number, iteration_number, lift_note)
VALUES (?, ?, ?, ?, ?, ?)`
		if _, err := tx.Exec(q, uID, ex, st, &sqlWeight{&weight}, set, reps, day, week, iter, nullString(note)); err != nil {
			return fmt.Errorf("failed to insert lift: %w", err)
		}
		return nil
	})
}

func (db *DB) RecentLifts(uID fto.UserID) ([]*fto.Lift, error) {
	var lfs []*fto.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT exercises.name, lifts.set_type, lifts.weight, lifts.set_number, lifts.reps, lifts.lift_note, lifts.day_number, lifts.week_number, lifts.iteration_number
FROM lifts
JOIN exercises
	ON lifts.exercise_id = exercises.id
WHERE user_id = ?
ORDER BY iteration_number DESC, week_number DESC, day_number DESC, lifts.created_at DESC
LIMIT 100`

		rows, err := tx.Query(q, uID)
		if err != nil {
			return fmt.Errorf("failed to query training_maxes: %w", err)
		}
		if lfs, err = lifts(rows); err != nil {
			return fmt.Errorf("failed to scan training_maxes: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set lifts: %w", err)
	}
	return lfs, nil
}

func (db *DB) transact(dbFn func(tx *sql.Tx) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.sql.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	if err := dbFn(tx); err != nil {
		return fmt.Errorf("failed to perform DB action: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (db *DB) SetTrainingMaxes(uID fto.UserID, press, squat, bench, deadlift fto.Weight) error {
	err := db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO training_maxes
(user_id, exercise_id, training_max_weight) VALUES
(?, ?, ?), (?, ?, ?), (?, ?, ?), (?, ?, ?)`
		args := []interface{}{
			uID, db.mainLiftIDs[fto.OverheadPress], &sqlWeight{&press},
			uID, db.mainLiftIDs[fto.Squat], &sqlWeight{&squat},
			uID, db.mainLiftIDs[fto.BenchPress], &sqlWeight{&bench},
			uID, db.mainLiftIDs[fto.Deadlift], &sqlWeight{&deadlift},
		}
		if _, err := tx.Exec(q, args...); err != nil {
			return fmt.Errorf("failed to insert to training_maxes: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to set training maxes: %w", err)
	}
	return nil
}

func (db *DB) TrainingMaxes(uID fto.UserID) ([]*fto.TrainingMax, error) {
	var tms []*fto.TrainingMax
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT b.exname, a.training_max_weight
FROM training_maxes a
INNER JOIN
(
	SELECT exercises.id exid, exercises.name exname, MAX(created_at) latest
	FROM training_maxes
	JOIN exercises
		ON training_maxes.exercise_id = exercises.id
	WHERE user_id = ?
	GROUP BY exercises.id
) b
ON a.exercise_id = b.exid
	AND a.created_at = b.latest`

		rows, err := tx.Query(q, uID)
		if err != nil {
			return fmt.Errorf("failed to query training_maxes: %w", err)
		}
		if tms, err = trainingMaxes(rows); err != nil {
			return fmt.Errorf("failed to scan training_maxes: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set training maxes: %w", err)
	}
	return tms, nil
}

func trainingMaxes(rows *sql.Rows) ([]*fto.TrainingMax, error) {
	defer rows.Close()

	var tms []*fto.TrainingMax
	for rows.Next() {
		var tm fto.TrainingMax
		if err := rows.Scan(&tm.Exercise, &sqlWeight{&tm.Max}); err != nil {
			return nil, fmt.Errorf("failed to scan training max: %w", err)
		}
		tms = append(tms, &tm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan training maxes: %w", err)
	}
	return tms, nil
}

func lifts(rows *sql.Rows) ([]*fto.Lift, error) {
	defer rows.Close()

	var lfs []*fto.Lift
	for rows.Next() {
		var (
			lf   fto.Lift
			note sql.NullString
		)
		if err := rows.Scan(
			&lf.Exercise, &lf.SetType, &sqlWeight{&lf.Weight},
			&lf.SetNumber, &lf.Reps, &note,
			&lf.DayNumber, &lf.WeekNumber, &lf.IterationNumber); err != nil {
			return nil, fmt.Errorf("failed to scan lift: %w", err)
		}
		if note.Valid {
			lf.Note = note.String
		}
		lfs = append(lfs, &lf)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan lifts: %w", err)
	}
	return lfs, nil
}

func New(dbPath, migrationsPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_loc=UTC")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite DB: %w", err)
	}
	cleanupOnError := func(origErr error) error {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("error closing DB (%v) while handling original error: %w", closeErr, origErr)
		}
		return origErr
	}

	driver, err := migratesqlite3.WithInstance(db, &migratesqlite3.Config{
		MigrationsTable: migratesqlite3.DefaultMigrationsTable,
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

	switch err := m.Up(); {
	case err == nil:
		// Fine, good.
	case errors.Is(err, migrate.ErrNoChange):
		log.Print("No new migrations to apply")
	default:
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

	sdb := &DB{sql: db}

	if err := sdb.initMainLifts(); err != nil {
		return nil, fmt.Errorf("failed to init main lifts: %w", err)
	}

	return sdb, nil
}

func (db *DB) CreateExercise(ex fto.Exercise) error {
	return db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO exercises (name) VALUES (?)`
		_, err := tx.Exec(q, ex)
		sqlErr := sqlite3.Error{}
		if errors.As(err, &sqlErr) && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			// An expected error if we've already inserted this, we don't need to let
			// callers know about this.
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to insert exercise: %w", err)
		}
		return nil
	})
}

type exercise struct {
	ID       int
	Exercise fto.Exercise
}

func (db *DB) exercises(exs []fto.Exercise) ([]exercise, error) {
	var out []exercise
	err := db.transact(func(tx *sql.Tx) error {
		q := fmt.Sprintf(`
SELECT id, name
FROM exercises
WHERE name IN %s`, repeatedArgs(len(exs)))

		var args []interface{}
		for _, ex := range exs {
			args = append(args, ex)
		}

		rows, err := tx.Query(q, args...)
		if err != nil {
			return fmt.Errorf("failed to query training_maxes: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var e exercise
			if err := rows.Scan(&e.ID, &e.Exercise); err != nil {
				return fmt.Errorf("failed to scan exercise: %w", err)
			}
			out = append(out, e)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to scan exercises: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load exercises: %w", err)
	}
	return out, nil
}

func (db *DB) initMainLifts() error {
	// First, create all the main lifts.
	exs := fto.MainExercises()
	for _, ex := range exs {
		if err := db.CreateExercise(ex); err != nil {
			return fmt.Errorf("failed to create exercise %q: %w", ex, err)
		}
	}

	// Now, load all of their IDs.
	mainLiftIDs := make(map[fto.Exercise]int)
	exsWithIDs, err := db.exercises(exs)
	if err != nil {
		return err
	}
	for _, ex := range exsWithIDs {
		mainLiftIDs[ex.Exercise] = ex.ID
	}

	db.mainLiftIDs = mainLiftIDs

	return nil
}

func repeatedArgs(n int) string {
	if n < 1 {
		// Normally, you wouldn't want to panic in a production application, but
		// this is clearly a programmer error and it's a personal project so imma
		// just try to not make this particular error :shrug:.
		panic(fmt.Sprintf("repeatedArgs called with value less than one, %d", n))
	}

	return "(" + strings.Repeat("?,", n-1) + "?)"
}
