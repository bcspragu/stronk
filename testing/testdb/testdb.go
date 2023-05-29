// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import (
	"sort"

	"github.com/lexacali/fivethreeone/fto"
)

func New() *DB {
	return &DB{
		lifts:          make(map[fto.UserID][]*fto.Lift),
		trainingMaxes:  make(map[fto.UserID][]*fto.TrainingMax),
		smallestDenoms: make(map[fto.UserID][]fto.Weight),
	}
}

type DB struct {
	users          []*fto.User
	lifts          map[fto.UserID][]*fto.Lift
	trainingMaxes  map[fto.UserID][]*fto.TrainingMax
	smallestDenoms map[fto.UserID][]fto.Weight
}

func (db *DB) CreateUser(name string) (fto.UserID, error) {
	id := fto.UserID(len(db.users))
	db.users = append(db.users, &fto.User{
		ID:   id,
		Name: name,
	})
	return id, nil
}

func (db *DB) User(id fto.UserID) (*fto.User, error) {
	idx := int(id)
	if idx >= len(db.users) {
		return nil, fto.ErrUserNotFound
	}
	return db.users[idx], nil
}

func (db *DB) UserByName(name string) (*fto.User, error) {
	for _, u := range db.users {
		if u.Name == name {
			return u, nil
		}
	}
	return nil, fto.ErrUserNotFound
}

func (db *DB) RecentLifts(uID fto.UserID) ([]*fto.Lift, error) {
	lifts := db.lifts[uID]

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

func (db *DB) RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int) error {
	db.lifts[uID] = append(db.lifts[uID], &fto.Lift{
		UserID:          uID,
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

func (db *DB) SetTrainingMaxes(uID fto.UserID, press, squat, bench, deadlift fto.Weight) error {
	db.trainingMaxes[uID] = append(db.trainingMaxes[uID],
		&fto.TrainingMax{Exercise: fto.OverheadPress, Max: press},
		&fto.TrainingMax{Exercise: fto.Squat, Max: squat},
		&fto.TrainingMax{Exercise: fto.BenchPress, Max: bench},
		&fto.TrainingMax{Exercise: fto.Deadlift, Max: deadlift},
	)
	return nil
}

func (db *DB) TrainingMaxes(uID fto.UserID) ([]*fto.TrainingMax, error) {
	var (
		out   []*fto.TrainingMax
		found = make(map[fto.Exercise]bool)
	)
	tms := db.trainingMaxes[uID]
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

func (db *DB) SetSmallestDenom(uID fto.UserID, small fto.Weight) error {
	db.smallestDenoms[uID] = append(db.smallestDenoms[uID], small)
	return nil
}

func (db *DB) SmallestDenom(uID fto.UserID) (fto.Weight, error) {
	denoms := db.smallestDenoms[uID]
	if len(denoms) == 0 {
		return fto.Weight{}, fto.ErrNoSmallestDenom
	}
	return denoms[len(denoms)-1], nil
}
