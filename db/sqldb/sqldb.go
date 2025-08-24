// Package sqldb implements the server.DB interface, backed by a sqlite database.
package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/bcspragu/stronk"
	"github.com/golang-migrate/migrate/v4"
	"github.com/mattn/go-sqlite3"

	migratesqlite3 "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const baseLiftQuery = `
SELECT
	lifts.id,
	routine_sets.id,
	exercises.name,
	routine_movements.set_type,
	lifts.weight,
	routine_sets.set_order,
	lifts.reps,
	lifts.lift_note,
	routine_days.day_order,
	routine_weeks.week_order,
	(SELECT COUNT(routine_set_id)-1 FROM lifts lifts2 WHERE lifts2.created_at <= lifts.created_at GROUP BY routine_set_id),
	routine_sets.to_failure
FROM lifts
JOIN routine_sets
	ON lifts.routine_set_id = routine_sets.id
JOIN routine_movements
	ON routine_sets.routine_movement_id = routine_movements.id
JOIN routine_days
	ON routine_movements.routine_day_id = routine_days.id
JOIN routine_weeks
	ON routine_days.routine_week_id = routine_weeks.id
JOIN exercises
	ON routine_movements.exercise_id = exercises.id
`

type DB struct {
	mu          sync.Mutex
	sql         *sql.DB
	mainLiftIDs map[stronk.Exercise]int
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Lift(id stronk.LiftID) (*stronk.Lift, error) {
	var lift *stronk.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := baseLiftQuery + `WHERE lifts.id = ?`

		rows, err := tx.Query(q, id)
		if err != nil {
			return fmt.Errorf("failed to query training_maxes: %w", err)
		}
		lfs, err := lifts(rows)
		if err != nil {
			return fmt.Errorf("failed to scan training_maxes: %w", err)
		}
		if n := len(lfs); n != 1 {
			return fmt.Errorf("unexpected number of lifts %d", n)
		}
		lift = lfs[0]
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set lifts: %w", err)
	}
	return lift, nil
}

func (db *DB) SkippedWeeks() ([]stronk.SkippedWeek, error) {
	var weeks []stronk.SkippedWeek
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

func (db *DB) ComparableLifts(ex stronk.Exercise, weight stronk.Weight) (*stronk.ComparableLifts, error) {
	// We want to find two comparable lifts:
	//  1. The closest in weight, breaking ties by highest ORM equivalent ("Most Similar")
	//  2. The highest ORM equivalent reps, period. ("PR")
	var lfs []*stronk.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := baseLiftQuery + `
WHERE exercises.name = ?
	AND routine_sets.to_failure = TRUE
ORDER BY lifts.created_at DESC
LIMIT 250`

		rows, err := tx.Query(q, ex)
		if err != nil {
			return fmt.Errorf("failed to query comparable lifts: %w", err)
		}
		if lfs, err = lifts(rows); err != nil {
			return fmt.Errorf("failed to scan comparable lifts: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load comparables: %w", err)
	}

	return stronk.CalcComparables(lfs, weight), nil
}

func (db *DB) RecentFailureSets() ([]*stronk.Lift, error) {
	var lfs []*stronk.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := baseLiftQuery + `
WHERE routine_movements.set_type = 'MAIN'
	AND routine_sets.to_failure = TRUE
ORDER BY created_at DESC
LIMIT 250`

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

func (db *DB) LatestLift() (*stronk.Lift, error) {
	var lf *stronk.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := baseLiftQuery + `
ORDER BY lifts.created_at DESC
LIMIT 1
`

		theLift, err := lift(tx.QueryRow(q))
		if err != nil {
			return fmt.Errorf("failed to load latest lift: %w", err)
		}
		lf = theLift
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to set lifts: %w", err)
	}
	return lf, nil
}

func (db *DB) RecentLifts() ([]*stronk.Lift, error) {
	var lfs []*stronk.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := baseLiftQuery + `LIMIT 100`

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

func (db *DB) LatestLiftPerSetID(setIDs []stronk.RoutineSetID) (map[stronk.RoutineSetID]*stronk.Lift, error) {
	var lfs []*stronk.Lift
	err := db.transact(func(tx *sql.Tx) error {
		q := baseLiftQuery + fmt.Sprintf(`
WHERE routine_sets.id IN %s
LIMIT 100`, repeatedArgs(len(setIDs)))

		args := make([]any, 0, len(setIDs))
		for _, id := range setIDs {
			args = append(args, id)
		}

		rows, err := tx.Query(q, args...)
		if err != nil {
			return fmt.Errorf("failed to query lifts: %w", err)
		}
		if lfs, err = lifts(rows); err != nil {
			return fmt.Errorf("failed to scan lifts: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get lifts: %w", err)
	}
	m := make(map[stronk.RoutineSetID][]*stronk.Lift)
	for _, lf := range lfs {
		m[lf.RoutineSetID] = append(m[lf.RoutineSetID], lf)
	}
	for _, lfs := range m {
		slices.SortFunc(lfs, func(a, b *stronk.Lift) int {
			return b.IterationNumber - a.IterationNumber
		})
	}
	out := make(map[stronk.RoutineSetID]*stronk.Lift)
	for id, lfs := range m {
		out[id] = lfs[0]
	}
	return out, nil
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

func (db *DB) SetTrainingMaxes(press, squat, bench, deadlift stronk.Weight) error {
	err := db.transact(func(tx *sql.Tx) error {
		q := `INSERT INTO training_maxes
(exercise_id, training_max_weight) VALUES
(?, ?), (?, ?), (?, ?), (?, ?)`
		args := []any{
			db.mainLiftIDs[stronk.OverheadPress], &sqlWeight{&press},
			db.mainLiftIDs[stronk.Squat], &sqlWeight{&squat},
			db.mainLiftIDs[stronk.BenchPress], &sqlWeight{&bench},
			db.mainLiftIDs[stronk.Deadlift], &sqlWeight{&deadlift},
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

func (db *DB) TrainingMaxes() ([]*stronk.TrainingMax, error) {
	var tms []*stronk.TrainingMax
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

func trainingMaxes(rows *sql.Rows) ([]*stronk.TrainingMax, error) {
	defer rows.Close()

	var tms []*stronk.TrainingMax
	for rows.Next() {
		var tm stronk.TrainingMax
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

func (db *DB) SetSmallestDenom(small stronk.Weight) error {
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

func (db *DB) SmallestDenom() (stronk.Weight, error) {
	var small stronk.Weight
	err := db.transact(func(tx *sql.Tx) error {
		q := `
SELECT a.smallest_denom
FROM smallest_denom a
ORDER BY a.created_at DESC
LIMIT 1`
		err := tx.QueryRow(q).Scan(&sqlWeight{&small})
		if errors.Is(err, sql.ErrNoRows) {
			return stronk.ErrNoSmallestDenom
		}
		if err != nil {
			return fmt.Errorf("failed to scan smallest denominator: %w", err)
		}
		return nil
	})
	if err != nil {
		return stronk.Weight{}, err
	}
	return small, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func lift(sc scanner) (*stronk.Lift, error) {
	var (
		lf   stronk.Lift
		note sql.NullString
	)
	if err := sc.Scan(
		&lf.ID, &lf.RoutineSetID,
		&lf.Exercise, &lf.SetType, &sqlWeight{&lf.Weight},
		&lf.SetNumber, &lf.Reps, &note,
		&lf.DayNumber, &lf.WeekNumber, &lf.IterationNumber,
		&lf.ToFailure); err != nil {
		return nil, fmt.Errorf("failed to scan lift: %w", err)
	}
	if note.Valid {
		lf.Note = note.String
	}
	return &lf, nil
}

func lifts(rows *sql.Rows) ([]*stronk.Lift, error) {
	defer rows.Close()

	var lfs []*stronk.Lift
	for rows.Next() {
		lf, err := lift(rows)
		if err != nil {
			return nil, err
		}
		lfs = append(lfs, lf)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan lifts: %w", err)
	}
	return lfs, nil
}

func skippedWeeks(rows *sql.Rows) ([]stronk.SkippedWeek, error) {
	defer rows.Close()

	var wks []stronk.SkippedWeek
	for rows.Next() {
		var wk stronk.SkippedWeek
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

func New(dbPath, migrationsPath string, routine *stronk.Routine) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_loc=UTC")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite DB: %w", err)
	}
	cleanupOnError := func(origErr error) error {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("error closing DB (%v) while handling original error: %w", closeErr, origErr)
		}
		return origErr
	}

	// Set optimized SQLite defaults for single-user fitness app
	// See https://briandouglas.ie/sqlite-defaults/
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		// Reduced from article, 1s is fine for a single-user app.
		"PRAGMA busy_timeout = 1000",
		// Reduced from article, our entire DB will never be more than a few megs tops
		"PRAGMA cache_size = -5000",
		"PRAGMA foreign_keys = ON",
		// We don't really delete anything
		"PRAGMA auto_vacuum = FULL",
		"PRAGMA temp_store = MEMORY",
		// Reduced from article, 256MB memory-mapped I/O is fine for us
		"PRAGMA mmap_size = 268435456",
		// Reduced from article, our rows are small
		"PRAGMA page_size = 4096",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, cleanupOnError(fmt.Errorf("failed to set pragma %q: %w", pragma, err))
		}
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

	sdb := &DB{sql: db}

	// 20250820053857 is millisecond_precision_on_lifts, the last migration before
	// our manual backfill with the new routine system.
	if prevV <= 20250820053857 {
		// Upgrade to where we introduce the new table in 20250824124147_routine_system
		log.Println("Migrating DB to 20250824124147 to prepare for routine/lift migration")
		if err := m.Migrate(20250824124147); err != nil {
			return nil, cleanupOnError(fmt.Errorf("failed to migrate to new routine version: %w", err))
		}

		// Required if this is a fresh DB
		if err := sdb.initMainLifts(); err != nil {
			return nil, fmt.Errorf("failed to init main lifts: %w", err)
		}

		// Since we just did the migration, we know that we have no routines in our
		// `routines` table, so store the first one.
		storedRoutine, err := sdb.StoreRoutine(routine)
		if err != nil {
			return nil, cleanupOnError(fmt.Errorf("failed to store initial routine in DB: %w", err))
		}

		// Now do the manual migration
		log.Println("Copying lifts to the new table")
		if err := sdb.MigrateLiftsToNewFormat(storedRoutine); err != nil {
			return nil, cleanupOnError(fmt.Errorf("failed to migrate lifts to new DB table: %w", err))
		}
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

	if err := sdb.initMainLifts(); err != nil {
		return nil, fmt.Errorf("failed to init main lifts: %w", err)
	}

	return sdb, nil
}

func (db *DB) CreateExercise(ex stronk.Exercise) error {
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
	Exercise stronk.Exercise
}

func (db *DB) exercises(exs []stronk.Exercise) ([]exercise, error) {
	var out []exercise
	err := db.transact(func(tx *sql.Tx) error {
		q := fmt.Sprintf(`
SELECT id, name
FROM exercises
WHERE name IN %s`, repeatedArgs(len(exs)))

		var args []any
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
	exs := stronk.MainExercises()
	for _, ex := range exs {
		if err := db.CreateExercise(ex); err != nil {
			return fmt.Errorf("failed to create exercise %q: %w", ex, err)
		}
	}

	// Now, load all of their IDs.
	mainLiftIDs := make(map[stronk.Exercise]int)
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

// GetCurrentRoutine returns the most recently stored routine.
func (db *DB) GetCurrentRoutine() (*stronk.StoredRoutine, error) {
	var routine *stronk.StoredRoutine
	err := db.transact(func(tx *sql.Tx) error {
		// Get the most recent routine
		routineQuery := `
		SELECT id, name, created_at
		FROM routines
		ORDER BY created_at DESC
		LIMIT 1`

		var r stronk.StoredRoutine
		if err := tx.QueryRow(routineQuery).Scan(&r.ID, &r.Name, &r.CreatedAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil // No routine stored yet
			}
			return fmt.Errorf("failed to query current routine: %w", err)
		}

		// Load weeks
		weeksQuery := `
		SELECT id, week_name, optional, week_order
		FROM routine_weeks
		WHERE routine_id = ?
		ORDER BY week_order`

		weekRows, err := tx.Query(weeksQuery, r.ID)
		if err != nil {
			return fmt.Errorf("failed to query routine weeks: %w", err)
		}
		defer weekRows.Close()

		weekMap := make(map[stronk.RoutineWeekID]*stronk.StoredRoutineWeek)
		for weekRows.Next() {
			var week stronk.StoredRoutineWeek
			week.RoutineID = r.ID
			if err := weekRows.Scan(&week.ID, &week.WeekName, &week.Optional, &week.WeekOrder); err != nil {
				return fmt.Errorf("failed to scan routine week: %w", err)
			}
			weekMap[week.ID] = &week
			r.Weeks = append(r.Weeks, &week)
		}

		// Load days
		if len(r.Weeks) > 0 {
			var weekIDs []any
			for _, week := range r.Weeks {
				weekIDs = append(weekIDs, week.ID)
			}

			daysQuery := fmt.Sprintf(`
			SELECT id, routine_week_id, day_name, day_order
			FROM routine_days
			WHERE routine_week_id IN %s
			ORDER BY routine_week_id, day_order`, repeatedArgs(len(weekIDs)))

			dayRows, err := tx.Query(daysQuery, weekIDs...)
			if err != nil {
				return fmt.Errorf("failed to query routine days: %w", err)
			}
			defer dayRows.Close()

			dayMap := make(map[stronk.RoutineDayID]*stronk.StoredRoutineDay)
			for dayRows.Next() {
				var day stronk.StoredRoutineDay
				if err := dayRows.Scan(&day.ID, &day.RoutineWeekID, &day.DayName, &day.DayOrder); err != nil {
					return fmt.Errorf("failed to scan routine day: %w", err)
				}
				dayMap[day.ID] = &day
				weekMap[day.RoutineWeekID].Days = append(weekMap[day.RoutineWeekID].Days, &day)
			}

			// Load movements if we have days
			if len(dayMap) > 0 {
				var dayIDs []any
				for dayID := range dayMap {
					dayIDs = append(dayIDs, dayID)
				}

				movementsQuery := fmt.Sprintf(`
				SELECT rm.id, rm.routine_day_id, e.name, rm.set_type, rm.movement_order
				FROM routine_movements rm
				JOIN exercises e ON rm.exercise_id = e.id
				WHERE rm.routine_day_id IN %s
				ORDER BY rm.routine_day_id, rm.movement_order`, repeatedArgs(len(dayIDs)))

				movementRows, err := tx.Query(movementsQuery, dayIDs...)
				if err != nil {
					return fmt.Errorf("failed to query routine movements: %w", err)
				}
				defer movementRows.Close()

				movementMap := make(map[stronk.RoutineMovementID]*stronk.StoredRoutineMovement)
				for movementRows.Next() {
					var movement stronk.StoredRoutineMovement
					if err := movementRows.Scan(&movement.ID, &movement.RoutineDayID, &movement.Exercise, &movement.SetType, &movement.MovementOrder); err != nil {
						return fmt.Errorf("failed to scan routine movement: %w", err)
					}
					movementMap[movement.ID] = &movement
					dayMap[movement.RoutineDayID].Movements = append(dayMap[movement.RoutineDayID].Movements, &movement)
				}

				// Load sets if we have movements
				if len(movementMap) > 0 {
					var movementIDs []any
					for movementID := range movementMap {
						movementIDs = append(movementIDs, movementID)
					}

					setsQuery := fmt.Sprintf(`
					SELECT id, routine_movement_id, rep_target, to_failure, training_max_percentage, set_order
					FROM routine_sets
					WHERE routine_movement_id IN %s
					ORDER BY routine_movement_id, set_order`, repeatedArgs(len(movementIDs)))

					setRows, err := tx.Query(setsQuery, movementIDs...)
					if err != nil {
						return fmt.Errorf("failed to query routine sets: %w", err)
					}
					defer setRows.Close()

					for setRows.Next() {
						var set stronk.StoredRoutineSet
						if err := setRows.Scan(&set.ID, &set.RoutineMovementID, &set.RepTarget, &set.ToFailure, &set.TrainingMaxPercentage, &set.SetOrder); err != nil {
							return fmt.Errorf("failed to scan routine set: %w", err)
						}
						movementMap[set.RoutineMovementID].Sets = append(movementMap[set.RoutineMovementID].Sets, &set)
					}
				}
			}
		}

		routine = &r
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get current routine: %w", err)
	}
	return routine, nil
}

// StoreRoutine stores a new routine in the database
func (db *DB) StoreRoutine(routine *stronk.Routine) (*stronk.StoredRoutine, error) {
	var stored *stronk.StoredRoutine
	err := db.transact(func(tx *sql.Tx) error {
		// Insert routine
		routineQuery := `
		INSERT INTO routines (name) 
		VALUES (?) 
		RETURNING id`

		var routineID stronk.RoutineID
		if err := tx.QueryRow(routineQuery, routine.Name).Scan(&routineID); err != nil {
			return fmt.Errorf("failed to insert routine: %w", err)
		}

		stored = stronk.ConvertToStoredRoutine(routine, routineID)

		// Insert weeks
		for _, week := range stored.Weeks {
			weekQuery := `
			INSERT INTO routine_weeks (routine_id, week_name, optional, week_order)
			VALUES (?, ?, ?, ?)
			RETURNING id`

			if err := tx.QueryRow(weekQuery, routineID, week.WeekName, week.Optional, week.WeekOrder).Scan(&week.ID); err != nil {
				return fmt.Errorf("failed to insert routine week: %w", err)
			}
			week.RoutineID = routineID

			// Insert days
			for _, day := range week.Days {
				dayQuery := `
				INSERT INTO routine_days (routine_week_id, day_name, day_order)
				VALUES (?, ?, ?)
				RETURNING id`

				if err := tx.QueryRow(dayQuery, week.ID, day.DayName, day.DayOrder).Scan(&day.ID); err != nil {
					return fmt.Errorf("failed to insert routine day: %w", err)
				}
				day.RoutineWeekID = week.ID

				// Insert movements
				for _, movement := range day.Movements {
					movementQuery := `
					INSERT INTO routine_movements (routine_day_id, exercise_id, set_type, movement_order)
					VALUES (?, (SELECT id FROM exercises WHERE name = ?), ?, ?)
					RETURNING id`

					if err := tx.QueryRow(movementQuery, day.ID, movement.Exercise, movement.SetType, movement.MovementOrder).Scan(&movement.ID); err != nil {
						return fmt.Errorf("failed to insert routine movement: %w", err)
					}
					movement.RoutineDayID = day.ID

					// Insert sets
					for _, set := range movement.Sets {
						setQuery := `
						INSERT INTO routine_sets (routine_movement_id, rep_target, to_failure, training_max_percentage, set_order)
						VALUES (?, ?, ?, ?, ?)
						RETURNING id`

						if err := tx.QueryRow(setQuery, movement.ID, set.RepTarget, set.ToFailure, set.TrainingMaxPercentage, set.SetOrder).Scan(&set.ID); err != nil {
							return fmt.Errorf("failed to insert routine set: %w", err)
						}
						set.RoutineMovementID = movement.ID
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store routine: %w", err)
	}
	return stored, nil
}

// GetRoutineSet gets a specific routine set by ID
func (db *DB) GetRoutineSet(id stronk.RoutineSetID) (*stronk.StoredRoutineSet, error) {
	var set *stronk.StoredRoutineSet
	err := db.transact(func(tx *sql.Tx) error {
		query := `
		SELECT id, routine_movement_id, rep_target, to_failure, training_max_percentage, set_order
		FROM routine_sets
		WHERE id = ?`

		var s stronk.StoredRoutineSet
		if err := tx.QueryRow(query, id).Scan(&s.ID, &s.RoutineMovementID, &s.RepTarget, &s.ToFailure, &s.TrainingMaxPercentage, &s.SetOrder); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("routine set %d not found", id)
			}
			return fmt.Errorf("failed to query routine set: %w", err)
		}
		set = &s
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get routine set: %w", err)
	}
	return set, nil
}

// RecordLift records a lift using the new routine system
func (db *DB) RecordLift(routineSetID stronk.RoutineSetID, reps int, note string, weight stronk.Weight) (stronk.LiftID, error) {
	var id stronk.LiftID
	err := db.transact(func(tx *sql.Tx) error {
		query := `
		INSERT INTO lifts (routine_set_id, reps, lift_note, weight)
		VALUES (?, ?, ?, ?)
		RETURNING id`

		if err := tx.QueryRow(query, routineSetID, reps, nullString(note), &sqlWeight{&weight}).Scan(&id); err != nil {
			return fmt.Errorf("failed to insert lift: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to record lift: %w", err)
	}
	return id, nil
}

// EditLift edits a lift using the new system
func (db *DB) EditLift(id stronk.LiftID, note string, reps int) error {
	return db.transact(func(tx *sql.Tx) error {
		query := `
		UPDATE lifts
		SET reps = ?, lift_note = ?
		WHERE id = ?`
		_, err := tx.Exec(query, reps, nullString(note), id)
		return err
	})
}

// MigrateLiftsToNewFormat migrates existing lifts from the old format to the new one
// This should be called after storing the routine in the database
func (db *DB) MigrateLiftsToNewFormat(storedRoutine *stronk.StoredRoutine) error {
	return db.transact(func(tx *sql.Tx) error {
		// Get all existing lifts
		liftsQuery := `
		SELECT l.id, e.name, l.set_type, l.weight, l.set_number, l.reps, l.lift_note, l.day_number, l.week_number, l.iteration_number, l.to_failure, l.created_at
		FROM lifts l
		JOIN exercises e ON l.exercise_id = e.id
		ORDER BY l.iteration_number, l.week_number, l.day_number, l.created_at`

		rows, err := tx.Query(liftsQuery)
		if err != nil {
			return fmt.Errorf("failed to query existing lifts: %w", err)
		}
		defer rows.Close()

		var migratedCount int
		for rows.Next() {
			var (
				id              stronk.LiftID
				exercise        stronk.Exercise
				setType         stronk.SetType
				weight          stronk.Weight
				setNumber       int
				reps            int
				note            sql.NullString
				dayNumber       int
				weekNumber      int
				iterationNumber int
				toFailure       bool
				createdAt       string
			)

			if err := rows.Scan(
				&id, &exercise, &setType, &sqlWeight{&weight},
				&setNumber, &reps, &note, &dayNumber, &weekNumber,
				&iterationNumber, &toFailure, &createdAt); err != nil {
				return fmt.Errorf("failed to scan lift: %w", err)
			}

			// Find the corresponding routine set
			routineSetID, err := db.findMatchingRoutineSet(storedRoutine, exercise, setType,
				dayNumber, weekNumber, setNumber)
			if err != nil {
				// Log and skip this lift if we can't find a match
				log.Printf("Warning: Could not find matching routine set for lift %d: %v", id, err)
				continue
			}

			// Insert into new lifts table
			insertQuery := `
			INSERT INTO lifts_new (routine_set_id, weight, reps, lift_note, created_at)
			VALUES (?, ?, ?, ?, ?)`

			if _, err := tx.Exec(insertQuery, routineSetID, &sqlWeight{&weight}, reps, note, createdAt); err != nil {
				return fmt.Errorf("failed to insert migrated lift: %w", err)
			}

			migratedCount++
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating through lifts: %w", err)
		}

		log.Printf("Successfully migrated %d lifts to new format", migratedCount)
		return nil
	})
}

// findMatchingRoutineSet finds the routine set ID that matches the given lift parameters
func (db *DB) findMatchingRoutineSet(storedRoutine *stronk.StoredRoutine, exercise stronk.Exercise, setType stronk.SetType, dayNumber, weekNumber, setNumber int) (stronk.RoutineSetID, error) {

	// Navigate through the routine structure to find the matching set
	if weekNumber >= len(storedRoutine.Weeks) {
		return 0, fmt.Errorf("week %d not found in routine", weekNumber)
	}
	week := storedRoutine.Weeks[weekNumber]

	if dayNumber >= len(week.Days) {
		return 0, fmt.Errorf("day %d not found in week %d", dayNumber, weekNumber)
	}
	day := week.Days[dayNumber]

	// Find the movement that matches the exercise and set type
	var targetMovement *stronk.StoredRoutineMovement
	for _, movement := range day.Movements {
		if movement.Exercise == exercise && movement.SetType == setType {
			targetMovement = movement
			break
		}
	}

	if targetMovement == nil {
		return 0, fmt.Errorf("no movement found for exercise %s, set type %s on day %d, week %d",
			exercise, setType, dayNumber, weekNumber)
	}

	// Get the set (0-indexed)
	if setNumber >= len(targetMovement.Sets) {
		return 0, fmt.Errorf("set %d not found in movement (has %d sets)",
			setNumber, len(targetMovement.Sets))
	}

	return targetMovement.Sets[setNumber].ID, nil
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
