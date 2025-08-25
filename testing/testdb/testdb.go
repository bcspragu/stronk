// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import (
	"errors"
	"fmt"

	"github.com/bcspragu/stronk"
)

func New(routine *stronk.Routine) *DB {
	db := &DB{}
	if _, err := db.StoreRoutine(routine); err != nil {
		panic(fmt.Sprintf("failed to store routine: %v", err))
	}
	return db
}

type lift struct {
	ID           stronk.LiftID
	RoutineSetID stronk.RoutineSetID
	Reps         int
	Note         string
	Weight       stronk.Weight
}

type DB struct {
	lifts          []*lift
	trainingMaxes  []*stronk.TrainingMax
	smallestDenoms []stronk.Weight
	skippedWeeks   []stronk.SkippedWeek
	routines       []*stronk.StoredRoutine
}

func (db *DB) GetCurrentRoutine() (*stronk.StoredRoutine, error) {
	if len(db.routines) == 0 {
		return nil, nil
	}
	return db.routines[len(db.routines)-1], nil
}
func (db *DB) StoreRoutine(routine *stronk.Routine) (*stronk.StoredRoutine, error) {
	id := stronk.RoutineID(len(db.routines) + 1)
	storedRoutine := stronk.ConvertToStoredRoutine(routine, id)
	var (
		weekID = stronk.RoutineWeekID(1)
		dayID  = stronk.RoutineDayID(1)
		mvmtID = stronk.RoutineMovementID(1)
		setID  = stronk.RoutineSetID(1)
	)
	for _, week := range storedRoutine.Weeks {
		week.ID = weekID
		weekID++
		for _, day := range week.Days {
			day.ID = dayID
			dayID++
			for _, mvmt := range day.Movements {
				mvmt.ID = mvmtID
				mvmtID++
				for _, set := range mvmt.Sets {
					set.ID = setID
					setID++
				}
			}
		}
	}
	db.routines = append(db.routines, storedRoutine)
	return storedRoutine, nil
}
func (db *DB) GetRoutineSet(id stronk.RoutineSetID) (*stronk.StoredRoutineSet, error) {
	for _, r := range db.routines {
		for _, w := range r.Weeks {
			for _, d := range w.Days {
				for _, m := range d.Movements {
					for _, s := range m.Sets {
						if s.ID == id {
							return s, nil
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("routine set %d not found", id)
}
func (db *DB) MigrateLiftsToNewFormat(storedRoutine *stronk.StoredRoutine) error {
	return errors.New("not relevant to in-memory test DB")
}

func (db *DB) LatestLift() (*stronk.Lift, error) {
	if len(db.lifts) == 0 {
		return nil, nil
	}
	return db.toLift(db.lifts[len(db.lifts)-1]), nil
}
func (db *DB) LatestLiftPerSetID(setIDs []stronk.RoutineSetID) (map[stronk.RoutineSetID]*stronk.Lift, error) {
	wantedSetIDs := make(map[stronk.RoutineSetID]bool)
	for _, id := range setIDs {
		wantedSetIDs[id] = true
	}

	out := make(map[stronk.RoutineSetID]*stronk.Lift)
	for i := len(db.lifts) - 1; i >= 0; i-- {
		ll := db.lifts[i]
		if !wantedSetIDs[ll.RoutineSetID] {
			continue
		}
		if _, ok := out[ll.RoutineSetID]; ok {
			// Already have one for this routine set ID
			continue
		}
		out[ll.RoutineSetID] = db.toLift(ll)
		if len(out) == len(setIDs) {
			break
		}
	}
	return out, nil
}

func (db *DB) toLift(l *lift) *stronk.Lift {
	var (
		set  *stronk.StoredRoutineSet
		mvmt *stronk.StoredRoutineMovement
		day  *stronk.StoredRoutineDay
		week *stronk.StoredRoutineWeek
	)
outer:
	for _, r := range db.routines {
		for _, w := range r.Weeks {
			for _, d := range w.Days {
				for _, m := range d.Movements {
					for _, s := range m.Sets {
						if s.ID == l.RoutineSetID {
							set = s
							mvmt = m
							day = d
							week = w
							break outer
						}
					}
				}
			}
		}
	}
	if set == nil {
		panic(fmt.Sprintf("no set found for id %d", l.RoutineSetID))
	}
	return &stronk.Lift{
		ID:           l.ID,
		RoutineSetID: l.RoutineSetID,
		Weight:       l.Weight,
		Reps:         l.Reps,
		Note:         l.Note,

		DayNumber:       day.DayOrder,
		WeekNumber:      week.WeekOrder,
		IterationNumber: 0, // We just make this up for now, I think it's fine?
		Exercise:        mvmt.Exercise,
		SetType:         mvmt.SetType,
		SetNumber:       set.SetOrder,
		ToFailure:       set.ToFailure,
	}
}

func (db *DB) Lift(id stronk.LiftID) (*stronk.Lift, error) {
	for _, l := range db.lifts {
		if l.ID == id {
			return db.toLift(l), nil
		}
	}
	return nil, fmt.Errorf("lift %d not found", id)
}

func (db *DB) EditLift(id stronk.LiftID, note string, reps int) error {
	for _, l := range db.lifts {
		if l.ID == id {
			l.Note = note
			l.Reps = reps
			return nil
		}
	}
	return fmt.Errorf("lift %d not found", id)
}

func (db *DB) RecentLifts() ([]*stronk.Lift, error) {
	lifts := make([]*stronk.Lift, len(db.lifts))

	for i := len(db.lifts) - 1; i >= 0; i-- {
		ll := db.lifts[i]
		lifts = append(lifts, db.toLift(ll))
	}

	return lifts, nil
}

func (db *DB) RecordLift(routineSetID stronk.RoutineSetID, reps int, note string, weight stronk.Weight) (stronk.LiftID, error) {
	id := stronk.LiftID(len(db.lifts) + 1)
	db.lifts = append(db.lifts, &lift{
		ID:           id,
		RoutineSetID: routineSetID,
		Weight:       weight,
		Reps:         reps,
		Note:         note,
	})
	return id, nil
}

func (db *DB) SetTrainingMaxes(press, squat, bench, deadlift stronk.Weight) error {
	db.trainingMaxes = append(db.trainingMaxes,
		&stronk.TrainingMax{Exercise: stronk.OverheadPress, Max: press},
		&stronk.TrainingMax{Exercise: stronk.Squat, Max: squat},
		&stronk.TrainingMax{Exercise: stronk.BenchPress, Max: bench},
		&stronk.TrainingMax{Exercise: stronk.Deadlift, Max: deadlift},
	)
	return nil
}

func (db *DB) TrainingMaxes() ([]*stronk.TrainingMax, error) {
	var (
		out   []*stronk.TrainingMax
		found = make(map[stronk.Exercise]bool)
	)
	tms := db.trainingMaxes
	for i := len(tms) - 1; i >= 0; i-- {
		tm := tms[i]
		if found[tm.Exercise] {
			continue
		}
		out = append(out, tm)
		found[tm.Exercise] = true
	}

	return out, nil
}

func (db *DB) SetSmallestDenom(small stronk.Weight) error {
	db.smallestDenoms = append(db.smallestDenoms, small)
	return nil
}

func (db *DB) SmallestDenom() (stronk.Weight, error) {
	denoms := db.smallestDenoms
	if len(denoms) == 0 {
		return stronk.Weight{}, stronk.ErrNoSmallestDenom
	}
	return denoms[len(denoms)-1], nil
}

func (db *DB) ComparableLifts(ex stronk.Exercise, weight stronk.Weight) (*stronk.ComparableLifts, error) {
	return &stronk.ComparableLifts{}, nil
}

func (db *DB) RecentFailureSets() ([]*stronk.Lift, error) {
	return []*stronk.Lift{}, nil
}

func (db *DB) SkippedWeeks() ([]stronk.SkippedWeek, error) {
	return db.skippedWeeks, nil
}

func (db *DB) SkipWeek(note string, week, iter int) error {
	db.skippedWeeks = append(db.skippedWeeks, stronk.SkippedWeek{
		Week:      week,
		Iteration: iter,
		Note:      note,
	})
	return nil
}
