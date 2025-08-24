// Package stronk contains the domain types for doing exercise stuff.
package stronk

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
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

	// Only set if we found a match, won't always be the case.
	AssociatedLiftID LiftID
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

type LiftID int
type RoutineID int
type RoutineWeekID int
type RoutineDayID int
type RoutineMovementID int
type RoutineSetID int

// Database routine types - these represent the stored routine structure
type StoredRoutine struct {
	ID        RoutineID
	Name      string
	CreatedAt time.Time
	Weeks     []*StoredRoutineWeek
}

func (s *StoredRoutine) LastSetID() (RoutineSetID, bool) {
	lastWeekIdx := len(s.Weeks) - 1
	if lastWeekIdx == -1 {
		return 0, false
	}
	lastWeek := s.Weeks[lastWeekIdx]
	lastDayIdx := len(lastWeek.Days) - 1
	if lastDayIdx == -1 {
		return 0, false
	}
	lastDay := lastWeek.Days[lastDayIdx]
	lastMvmtIdx := len(lastDay.Movements) - 1
	if lastMvmtIdx == -1 {
		return 0, false
	}
	lastMvmt := lastDay.Movements[lastMvmtIdx]
	lastSetIdx := len(lastMvmt.Sets) - 1
	if lastSetIdx == -1 {
		return 0, false
	}
	return lastMvmt.Sets[lastSetIdx].ID, true
}

func (s *StoredRoutine) ToRoutine() *Routine {
	var weeks []*WorkoutWeek
	for _, week := range s.Weeks {
		var days []*WorkoutDay
		for _, day := range week.Days {
			var mvmts []*Movement
			for _, mvmt := range day.Movements {
				var sets []*Set
				for _, set := range mvmt.Sets {
					sets = append(sets, &Set{
						RepTarget:             set.RepTarget,
						ToFailure:             set.ToFailure,
						TrainingMaxPercentage: set.TrainingMaxPercentage,
					})
				}
				mvmts = append(mvmts, &Movement{
					Exercise: mvmt.Exercise,
					SetType:  mvmt.SetType,
					Sets:     sets,
				})
			}
			days = append(days, &WorkoutDay{
				DayName:   day.DayName,
				Movements: mvmts,
			})
		}
		weeks = append(weeks, &WorkoutWeek{
			WeekName: week.WeekName,
			Optional: week.Optional,
			Days:     days,
		})
	}

	return &Routine{
		Name:  s.Name,
		Weeks: weeks,
	}
}

type StoredRoutineWeek struct {
	ID        RoutineWeekID
	RoutineID RoutineID
	WeekName  string
	Optional  bool
	WeekOrder int
	Days      []*StoredRoutineDay
}

type StoredRoutineDay struct {
	ID            RoutineDayID
	RoutineWeekID RoutineWeekID
	DayName       string
	DayOrder      int
	Movements     []*StoredRoutineMovement
}

type StoredRoutineMovement struct {
	ID            RoutineMovementID
	RoutineDayID  RoutineDayID
	Exercise      Exercise
	SetType       SetType
	MovementOrder int
	Sets          []*StoredRoutineSet
}

type StoredRoutineSet struct {
	ID                    RoutineSetID
	RoutineMovementID     RoutineMovementID
	RepTarget             int
	ToFailure             bool
	TrainingMaxPercentage int
	SetOrder              int
}

type Lift struct {
	ID           LiftID
	RoutineSetID RoutineSetID
	Weight       Weight
	Reps         int // NULL if same as routine or for non-failure sets
	Note         string

	// Day - 0, 1, 2, ... in a given week
	// Week - 0, 1, 2, ... in a given iteration
	// Iteration - 0, 1, 2, ... basically how many times you've gone through the
	// routine
	DayNumber       int
	WeekNumber      int
	IterationNumber int

	// Derived fields (filled from routine_sets via join)
	Exercise  Exercise
	SetType   SetType
	SetNumber int // For compatibility with existing code
	ToFailure bool
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

func FindPR(lifts []*Lift) *Lift {
	if len(lifts) == 0 {
		return nil
	}

	var max, maxIndex int
	for i, l := range lifts {
		orm := l.AsOneRepMax()
		if orm.Value > max {
			max = orm.Value
			maxIndex = i
		}
	}

	return lifts[maxIndex]
}

func CalcComparables(lifts []*Lift, weight Weight) *ComparableLifts {
	pr := FindPR(lifts)
	var equivReps float64
	if pr != nil {
		equivReps = pr.CalcEquivalentReps(weight)
	}
	return &ComparableLifts{
		ClosestWeight:    FindClosest(lifts, weight),
		PersonalRecord:   pr,
		PREquivalentReps: equivReps,
	}
}

func FindClosest(lifts []*Lift, weight Weight) *Lift {
	if len(lifts) == 0 {
		return nil
	}

	var (
		closest  = abs(lifts[0].Weight.Value - weight.Value)
		max, idx int
	)
	for i, l := range lifts {
		dist := abs(l.Weight.Value - weight.Value)
		orm := l.AsOneRepMax()
		if dist < closest || dist == closest && orm.Value > max {
			closest = dist
			max = orm.Value
			idx = i
		}
	}
	return lifts[idx]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// RoutineHash generates a SHA256 hash of the routine structure to detect changes
func (r *Routine) Hash() (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return fmt.Sprintf("%x", hash), nil
}

// ConvertToStoredRoutine converts a JSON routine to the stored database format
func ConvertToStoredRoutine(routine *Routine, routineID RoutineID) *StoredRoutine {
	stored := &StoredRoutine{
		ID:   routineID,
		Name: routine.Name,
	}

	for weekOrder, week := range routine.Weeks {
		storedWeek := &StoredRoutineWeek{
			WeekName:  week.WeekName,
			Optional:  week.Optional,
			WeekOrder: weekOrder,
		}

		for dayOrder, day := range week.Days {
			storedDay := &StoredRoutineDay{
				DayName:  day.DayName,
				DayOrder: dayOrder,
			}

			for movementOrder, movement := range day.Movements {
				storedMovement := &StoredRoutineMovement{
					Exercise:      movement.Exercise,
					SetType:       movement.SetType,
					MovementOrder: movementOrder,
				}

				for setOrder, set := range movement.Sets {
					storedSet := &StoredRoutineSet{
						RepTarget:             set.RepTarget,
						ToFailure:             set.ToFailure,
						TrainingMaxPercentage: set.TrainingMaxPercentage,
						SetOrder:              setOrder,
					}
					storedMovement.Sets = append(storedMovement.Sets, storedSet)
				}

				storedDay.Movements = append(storedDay.Movements, storedMovement)
			}

			storedWeek.Days = append(storedWeek.Days, storedDay)
		}

		stored.Weeks = append(stored.Weeks, storedWeek)
	}

	return stored
}

// FindRoutineSet locates a specific routine set by coordinates (week, day, movement, set indices)
func (sr *StoredRoutine) FindRoutineSet(weekIdx, dayIdx, movementIdx, setIdx int) *StoredRoutineSet {
	if weekIdx >= len(sr.Weeks) {
		return nil
	}
	week := sr.Weeks[weekIdx]

	if dayIdx >= len(week.Days) {
		return nil
	}
	day := week.Days[dayIdx]

	if movementIdx >= len(day.Movements) {
		return nil
	}
	movement := day.Movements[movementIdx]

	if setIdx >= len(movement.Sets) {
		return nil
	}
	return movement.Sets[setIdx]
}
