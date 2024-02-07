// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import (
	"fmt"
	"sort"

	"github.com/bcspragu/stronk"
)

func New() *DB {
	return &DB{}
}

type DB struct {
	lifts          []*stronk.Lift
	trainingMaxes  []*stronk.TrainingMax
	smallestDenoms []stronk.Weight
	skippedWeeks   []stronk.SkippedWeek
}

func (db *DB) Lift(id stronk.LiftID) (*stronk.Lift, error) {
	for _, l := range db.lifts {
		if l.ID == id {
			return l, nil
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
	copy(lifts, db.lifts)

	sort.Slice(lifts, func(i, j int) bool {
		if lifts[i].IterationNumber != lifts[j].IterationNumber {
			return lifts[i].IterationNumber > lifts[j].IterationNumber
		}

		if lifts[i].WeekNumber != lifts[j].WeekNumber {
			return lifts[i].WeekNumber > lifts[j].WeekNumber
		}

		if lifts[i].DayNumber != lifts[j].DayNumber {
			return lifts[i].DayNumber > lifts[j].DayNumber
		}
		return lifts[i].ID > lifts[j].ID
	})

	return lifts, nil
}

func (db *DB) RecordLift(ex stronk.Exercise, st stronk.SetType, weight stronk.Weight, set int, reps int, note string, day, week, iter int, toFailure bool) (stronk.LiftID, error) {
	id := stronk.LiftID(len(db.lifts) + 1)
	db.lifts = append(db.lifts, &stronk.Lift{
		ID:              id,
		Exercise:        ex,
		SetType:         st,
		Weight:          weight,
		SetNumber:       set,
		Reps:            reps,
		DayNumber:       day,
		WeekNumber:      week,
		IterationNumber: iter,
		Note:            note,
		ToFailure:       toFailure,
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
