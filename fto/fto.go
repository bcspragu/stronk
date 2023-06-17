// Package fto ((f)ive(t)hree(o)ne) contains the domain types
// for doing exercise stuff.
package fto

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrNoSmallestDenom = errors.New("no smallest denom")
)

type SkippedWeek struct {
	Week      int
	Iteration int
	Note      string
}

type ComparableLifts struct {
	ClosestWeight    *Lift
	PersonalRecord   *Lift
	PREquivalentReps float64
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

func (w *Weight) String() string {
	if w.Unit != DeciPounds {
		return "UNKNOWN_UNIT"
	}
	if w.Value%10 == 0 {
		return strconv.Itoa(w.Value / 10)
	}
	return fmt.Sprintf("%d.%d", w.Value/10, w.Value%10)
}

type TrainingMax struct {
	Max      Weight
	Exercise Exercise
}

type Routine struct {
	Name  string
	Weeks []*WorkoutWeek
}

func (r *Routine) Clone() *Routine {
	if r == nil {
		return nil
	}

	return &Routine{
		Name:  r.Name,
		Weeks: cloneWeeks(r.Weeks),
	}
}

func cloneWeeks(weeks []*WorkoutWeek) []*WorkoutWeek {
	var out []*WorkoutWeek
	for _, wk := range weeks {
		out = append(out, wk.Clone())
	}
	return out
}

type WorkoutWeek struct {
	WeekName string
	Optional bool
	Days     []*WorkoutDay
}

func (w *WorkoutWeek) Clone() *WorkoutWeek {
	if w == nil {
		return nil
	}

	return &WorkoutWeek{
		WeekName: w.WeekName,
		Optional: w.Optional,
		Days:     cloneDays(w.Days),
	}
}

func cloneDays(days []*WorkoutDay) []*WorkoutDay {
	var out []*WorkoutDay
	for _, d := range days {
		out = append(out, d.Clone())
	}
	return out
}

type WorkoutDay struct {
	DayName   string
	Movements []*Movement
}

func (w *WorkoutDay) Clone() *WorkoutDay {
	if w == nil {
		return nil
	}

	return &WorkoutDay{
		DayName:   w.DayName,
		Movements: cloneMovements(w.Movements),
	}
}

func cloneMovements(mvmts []*Movement) []*Movement {
	var out []*Movement
	for _, mvmt := range mvmts {
		out = append(out, mvmt.Clone())
	}
	return out
}

type Movement struct {
	Exercise Exercise
	SetType  SetType
	Sets     []*Set
}

func (m *Movement) Clone() *Movement {
	if m == nil {
		return nil
	}

	return &Movement{
		Exercise: m.Exercise,
		SetType:  m.SetType,
		Sets:     cloneSets(m.Sets),
	}
}

func cloneSets(sets []*Set) []*Set {
	var out []*Set
	for _, set := range sets {
		out = append(out, set.Clone())
	}
	return out
}

type Set struct {
	RepTarget int
	// ToFailure indicates if this set should go until no more reps can be done.
	// If true, usually indicated with a "+" in the UI, like "5+"
	ToFailure bool
	// TrainingMaxPercentage is a number between 0 and 100 indicating what
	// portion of your training max this lift is going for.
	TrainingMaxPercentage int

	// WeightTarget isn't set when users configure it, only in responses sent to
	// clients.
	WeightTarget Weight

	// Only set if the lift is to failure (i.e. ToFailure == true)
	FailureComparables *ComparableLifts
}

func (s *Set) Clone() *Set {
	if s == nil {
		return nil
	}

	return &Set{
		RepTarget:             s.RepTarget,
		ToFailure:             s.ToFailure,
		TrainingMaxPercentage: s.TrainingMaxPercentage,
		WeightTarget:          s.WeightTarget,
	}
}

type Lift struct {
	Exercise  Exercise
	SetType   SetType
	Weight    Weight
	SetNumber int
	Reps      int
	Note      string

	// Day - 0, 1, 2, ... in a given week
	// Week - 0, 1, 2, ... in a given iteration
	// Iteration - 0, 1, 2, ... basically how many times you've gone through the
	// routine
	DayNumber       int
	WeekNumber      int
	IterationNumber int
	ToFailure       bool
}

func (l *Lift) AsOneRepMax() Weight {
	return Weight{
		// ORM = Weight + (Weight * Num reps * 0.0333333)
		Value: int(float64(l.Weight.Value) + 0.033333333*float64(l.Weight.Value)*float64(l.Reps)),
		Unit:  l.Weight.Unit,
	}
}

func (l *Lift) CalcEquivalentReps(weight Weight) float64 {
	// To calculate how many reps that would be, we basically run the ORM calc in reverse:
	// ORM = Weight + (Weight * Num reps * 0.0333333)
	// (ORM - Weight) / (Weight * 0.0333333) = Num reps
	orm := l.AsOneRepMax()
	return float64((orm.Value-weight.Value)*30) / float64(weight.Value)
}
