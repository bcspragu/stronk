// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import (
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
}

func (db *DB) RecentLifts() ([]*fto.Lift, error) {
	lifts := make([]*fto.Lift, len(db.lifts))
	copy(lifts, db.lifts)

	sort.Slice(lifts, func(i, j int) bool {
		if lifts[i].IterationNumber != lifts[j].IterationNumber {
			return lifts[i].IterationNumber < lifts[j].IterationNumber
		}

		if lifts[i].WeekNumber != lifts[j].WeekNumber {
			return lifts[i].WeekNumber < lifts[j].WeekNumber
		}

		return lifts[i].DayNumber < lifts[j].DayNumber
	})

	return lifts, nil
}

func (db *DB) RecordLift(ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int) error {
	db.lifts = append(db.lifts, &fto.Lift{
		Exercise:        ex,
		SetType:         st,
		Weight:          weight,
		SetNumber:       set,
		Reps:            reps,
		DayNumber:       day,
		WeekNumber:      week,
		IterationNumber: iter,
		Note:            note,
	})
	return nil
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
