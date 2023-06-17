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

type scanner interface {
	Scan(dest ...interface{}) error
}

func (db *DB) RecordLift(ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int, toFailure bool) error {
	return db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO lifts
(exercise_id, set_type, set_number, reps, weight, day_number, week_number, iteration_number, lift_note, to_failure)
VALUES ((SELECT id FROM exercises WHERE name = ?), ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.Exec(q, ex, st, set, reps, &sqlWeight{&weight}, day, week, iter, nullString(note), toFailure); err != nil {
			return fmt.Errorf("failed to insert lift: %w", err)
		}
		return nil
	})
}

func (db *DB) SkippedWeeks() ([]fto.SkippedWeek, error) {
	var weeks []fto.SkippedWeek
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT week_number, iteration_number, note
FROM skipped_weeks
ORDER BY iteration_number DESC, week_number DESC
LIMIT 100`

		rows, err := tx.Query(q)
		if err != nil {
			return fmt.Errorf("failed to query skipped weeks: %w", err)
		}
		if weeks, err = skippedWeeks(rows); err != nil {
			return fmt.Errorf("failed to scan skipped weeks: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load skipped weeks: %w", err)
	}
	return weeks, nil
}

func (db *DB) SkipWeek(note string, week, iter int) error {
	return db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO skipped_weeks
(week_number, iteration_number, note)
VALUES (?, ?, ?)`
		if _, err := tx.Exec(q, week, iter, note); err != nil {
			return fmt.Errorf("failed to insert skipped week: %w", err)
		}
		return nil
	})
}

func (db *DB) ComparableLifts(ex fto.Exercise, weight fto.Weight) (*fto.ComparableLifts, error) {
	// We want to find two comparable lifts:
	//  1. The closest in weight, breaking ties by highest ORM equivalent ("Most Similar")
	//  2. The highest ORM equivalent reps, period. ("PR")
	var lfs []*fto.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT exercises.name, lifts.set_type, lifts.weight, lifts.set_number, lifts.reps, lifts.lift_note, lifts.day_number, lifts.week_number, lifts.iteration_number, lifts.to_failure
FROM lifts
JOIN exercises
	ON lifts.exercise_id = exercises.id
WHERE exercises.name = ?
	AND to_failure = TRUE
ORDER BY iteration_number DESC, week_number DESC, day_number DESC, lifts.created_at DESC
LIMIT 250`

		rows, err := tx.Query(q, ex)
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

	pr := findPR(lfs)
	var equivReps float64
	if pr != nil {
		equivReps = pr.CalcEquivalentReps(weight)
	}
	return &fto.ComparableLifts{
		ClosestWeight:    findClosest(lfs, weight),
		PersonalRecord:   pr,
		PREquivalentReps: equivReps,
	}, nil
}

func findClosest(lifts []*fto.Lift, weight fto.Weight) *fto.Lift {
	if len(lifts) == 0 {
		return nil
	}

	var (
		closest  = abs(lifts[0].Weight.Value - weight.Value)
		max, idx int
	)
	for i, l := range lifts {
		dist := abs(l.Weight.Value - weight.Value)
		orm := l.AsOneRepMax()
		if dist < closest || dist == closest && orm.Value > max {
			closest = dist
			max = orm.Value
			idx = i
		}
	}
	return lifts[idx]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func findPR(lifts []*fto.Lift) *fto.Lift {
	if len(lifts) == 0 {
		return nil
	}

	var max, maxIndex int
	for i, l := range lifts {
		orm := l.AsOneRepMax()
		if orm.Value > max {
			max = orm.Value
			maxIndex = i
		}
	}

	return lifts[maxIndex]
}

func (db *DB) RecentLifts() ([]*fto.Lift, error) {
	var lfs []*fto.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT exercises.name, lifts.set_type, lifts.weight, lifts.set_number, lifts.reps, lifts.lift_note, lifts.day_number, lifts.week_number, lifts.iteration_number, lifts.to_failure
FROM lifts
JOIN exercises
	ON lifts.exercise_id = exercises.id
ORDER BY iteration_number DESC, week_number DESC, day_number DESC, lifts.created_at DESC
LIMIT 100`

		rows, err := tx.Query(q)
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

func (db *DB) SetTrainingMaxes(press, squat, bench, deadlift fto.Weight) error {
	err := db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO training_maxes
(exercise_id, training_max_weight) VALUES
(?, ?), (?, ?), (?, ?), (?, ?)`
		args := []interface{}{
			db.mainLiftIDs[fto.OverheadPress], &sqlWeight{&press},
			db.mainLiftIDs[fto.Squat], &sqlWeight{&squat},
			db.mainLiftIDs[fto.BenchPress], &sqlWeight{&bench},
			db.mainLiftIDs[fto.Deadlift], &sqlWeight{&deadlift},
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

func (db *DB) TrainingMaxes() ([]*fto.TrainingMax, error) {
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
	GROUP BY exercises.id
) b
ON a.exercise_id = b.exid
	AND a.created_at = b.latest`

		rows, err := tx.Query(q)
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

func (db *DB) SetSmallestDenom(small fto.Weight) error {
	err := db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO smallest_denom (smallest_denom) VALUES (?)`
		if _, err := tx.Exec(q, &sqlWeight{&small}); err != nil {
			return fmt.Errorf("failed to insert to smallest_denom: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to set smallest denominator: %w", err)
	}
	return nil
}

func (db *DB) SmallestDenom() (fto.Weight, error) {
	var small fto.Weight
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT a.smallest_denom
FROM smallest_denom a
ORDER BY a.created_at DESC
LIMIT 1`
		err := tx.QueryRow(q).Scan(&sqlWeight{&small})
		if errors.Is(err, sql.ErrNoRows) {
			return fto.ErrNoSmallestDenom
		}
		if err != nil {
			return fmt.Errorf("failed to scan smallest denominator: %w", err)
		}
		return nil
	})
	if err != nil {
		return fto.Weight{}, err
	}
	return small, nil
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
			&lf.DayNumber, &lf.WeekNumber, &lf.IterationNumber,
			&lf.ToFailure); err != nil {
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

func skippedWeeks(rows *sql.Rows) ([]fto.SkippedWeek, error) {
	defer rows.Close()

	var wks []fto.SkippedWeek
	for rows.Next() {
		var wk fto.SkippedWeek
		if err := rows.Scan(&wk.Week, &wk.Iteration, &wk.Note); err != nil {
			return nil, fmt.Errorf("failed to scan skipped week: %w", err)
		}
		wks = append(wks, wk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan lifts: %w", err)
	}
	return wks, nil
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
			return fmt.Errorf("failed to query exercises: %w", err)
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
