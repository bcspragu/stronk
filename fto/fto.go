// Package fto ((f)ive(t)hree(o)ne) contains the domain types
// for doing exercise stuff.
package fto

import "time"

type UserID int64

type Exercise string

const (
	OverheadPress = Exercise("OVERHEAD_PRESS")
	Squat         = Exercise("SQUAT")
	BenchPress    = Exercise("BENCH_PRESS")
	Deadlife      = Exercise("DEADLIFT")
)

type SetType string

const (
	Warmup     = SetType("WARMUP")
	Main       = SetType("MAIN")
	Assistance = SetType("ASSISTANCE")
)

type WeightUnit string

const (
	// E.g. 1775 decipounds == 177.5 lbs
	DeciPounds = WeightUnit("DECI_POUNDS")
)

type Weight struct {
	Unit  WeightUnit
	Value int
}

type Routine []*Workout

type Workout struct {
	DayOfWeek time.Weekday
	Movements []*Movement
}

type Movement struct {
	Exercise Exercise
	SetType  SetType
	Sets     []*Set
}

type Set struct {
	RepTarget int
	// TrainingMaxPercentage is a number between 0 and 100 indicating what
	// portion of your training max this lift is going for.
	TrainingMaxPercentage int
}

type Lift struct {
	Exercise  Exercise
	SetType   SetType
	Weight    Weight
	SetNumber int
	Reps      int
	Note      string
	UserID    UserID
}
