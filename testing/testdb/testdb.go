// Package testdb is an in-memory implementation of the server.DB interface.
package testdb

import "github.com/lexacali/fivethreeone/fto"

func New() *DB {
	return &DB{}
}

type DB struct {
	lifts []fto.Lift
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
