// Package fto ((f)ive(t)hree(o)ne) contains the domain types
// for doing exercise stuff.
package fto

import (
	"errors"
	"strconv"
	"time"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserID uint64

func (uID UserID) String() string {
	return strconv.FormatUint(uint64(uID), 10)
}

type User struct {
	ID   UserID
	Name string
}

func MainExercises() []Exercise {
	return []Exercise{
		OverheadPress,
		Squat,
		BenchPress,
		Deadlift,
	}
}

type Exercise string

const (
	OverheadPress = Exercise("OVERHEAD_PRESS")
	Squat         = Exercise("SQUAT")
	BenchPress    = Exercise("BENCH_PRESS")
	Deadlift      = Exercise("DEADLIFT")
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

type TrainingMax struct {
	Max      Weight
	Exercise Exercise
}

type Routine struct {
	Name  string
	Weeks []*WorkoutWeek
}

type WorkoutWeek struct {
	WeekName string
	Days     []*WorkoutDay
}

type WorkoutDay struct {
	DayName   string
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
	// ToFailure indicates if this set should go until no more reps can be done.
	// If true, usually indicated with a "+" in the UI, like "5+"
	ToFailure bool
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

	// Day - 0, 1, 2, ... in a given week
	// Week - 0, 1, 2, ... in a given iteration
	// Iteration - 0, 1, 2, ... basically how many times you've gone through the
	// routine
	DayNumber       int
	WeekNumber      int
	IterationNumber int
}
