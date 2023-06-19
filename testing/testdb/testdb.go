// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import (
	"fmt"
	"sort"

	"github.com/lexacali/fivethreeone/fto"
)

func New() *DB {
	return &DB{}
}

type DB struct {
	lifts          []*fto.Lift
	trainingMaxes  []*fto.TrainingMax
	smallestDenoms []fto.Weight
	skippedWeeks   []fto.SkippedWeek
}

func (db *DB) Lift(id fto.LiftID) (*fto.Lift, error) {
	for _, l := range db.lifts {
		if l.ID == id {
			return l, nil
		}
	}
	return nil, fmt.Errorf("lift %d not found", id)
}

func (db *DB) EditLift(id fto.LiftID, note string, reps int) error {
	for _, l := range db.lifts {
		if l.ID == id {
			l.Note = note
			l.Reps = reps
			return nil
		}
	}
	return fmt.Errorf("lift %d not found", id)
}

func (db *DB) RecentLifts() ([]*fto.Lift, error) {
	lifts := make([]*fto.Lift, len(db.lifts))
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

func (db *DB) RecordLift(ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int, toFailure bool) (fto.LiftID, error) {
	id := fto.LiftID(len(db.lifts) + 1)
	db.lifts = append(db.lifts, &fto.Lift{
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

func (db *DB) SetTrainingMaxes(press, squat, bench, deadlift fto.Weight) error {
	db.trainingMaxes = append(db.trainingMaxes,
		&fto.TrainingMax{Exercise: fto.OverheadPress, Max: press},
		&fto.TrainingMax{Exercise: fto.Squat, Max: squat},
		&fto.TrainingMax{Exercise: fto.BenchPress, Max: bench},
		&fto.TrainingMax{Exercise: fto.Deadlift, Max: deadlift},
	)
	return nil
}

func (db *DB) TrainingMaxes() ([]*fto.TrainingMax, error) {
	var (
		out   []*fto.TrainingMax
		found = make(map[fto.Exercise]bool)
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

func (db *DB) SetSmallestDenom(small fto.Weight) error {
	db.smallestDenoms = append(db.smallestDenoms, small)
	return nil
}

func (db *DB) SmallestDenom() (fto.Weight, error) {
	denoms := db.smallestDenoms
	if len(denoms) == 0 {
		return fto.Weight{}, fto.ErrNoSmallestDenom
	}
	return denoms[len(denoms)-1], nil
}

func (db *DB) ComparableLifts(ex fto.Exercise, weight fto.Weight) (*fto.ComparableLifts, error) {
	return &fto.ComparableLifts{}, nil
}

func (db *DB) SkippedWeeks() ([]fto.SkippedWeek, error) {
	return db.skippedWeeks, nil
}

func (db *DB) SkipWeek(note string, week, iter int) error {
	db.skippedWeeks = append(db.skippedWeeks, fto.SkippedWeek{
		Week:      week,
		Iteration: iter,
		Note:      note,
	})
	return nil
}
