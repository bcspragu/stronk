// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import "github.com/lexacali/fivethreeone/fto"

func New() *DB {
	return &DB{
		trainingMaxes: make(map[fto.UserID][]*fto.TrainingMax),
	}
}

type DB struct {
	users         []*fto.User
	lifts         []fto.Lift
	trainingMaxes map[fto.UserID][]*fto.TrainingMax
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

func (db *DB) RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string) error {
	db.lifts = append(db.lifts, fto.Lift{
		UserID:    uID,
		Exercise:  ex,
		SetType:   st,
		Weight:    weight,
		SetNumber: set,
		Reps:      reps,
		Note:      note,
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
