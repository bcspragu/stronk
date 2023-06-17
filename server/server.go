package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lexacali/fivethreeone/fto"
)

// SecureCookie represents anything that knows how to encode and decode cookies
// intended to be stored in a user's browser.
type SecureCookie interface {
	Encode(name string, value interface{}) (string, error)
	Decode(name, value string, dst interface{}) error
	UseSecure() bool
}

type DB interface {
	SkippedWeeks() ([]fto.SkippedWeek, error)
	SkipWeek(note string, week, iter int) error

	SetTrainingMaxes(press, squat, bench, deadlift fto.Weight) error
	TrainingMaxes() ([]*fto.TrainingMax, error)

	SetSmallestDenom(small fto.Weight) error
	SmallestDenom() (fto.Weight, error)

	RecordLift(ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int, toFailure bool) error
	RecentLifts() ([]*fto.Lift, error)
	ComparableLifts(ex fto.Exercise, weight fto.Weight) (*fto.ComparableLifts, error)
}

type Server struct {
	mux *http.ServeMux

	routine *fto.Routine
	cookies SecureCookie
	db      DB
}

func New(routine *fto.Routine, db DB) *Server {
	s := &Server{
		routine: routine,
		db:      db,
	}
	s.initMux()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) initMux() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/trainingMaxes", s.serveTrainingMaxes)
	mux.HandleFunc("/api/setTrainingMaxes", s.serveSetTrainingMaxes)
	mux.HandleFunc("/api/nextLift", s.serveNextLift)
	mux.HandleFunc("/api/recordLift", s.serveRecordLift)
	mux.HandleFunc("/api/skipOptionalWeek", s.skipOptionalWeek)

	s.mux = mux
}

func (s *Server) serveTrainingMaxes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	tms, err := s.db.TrainingMaxes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// For JSON serialization
	if tms == nil {
		tms = []*fto.TrainingMax{}
	}

	var sd *fto.Weight
	tmpSD, err := s.db.SmallestDenom()
	if err == nil {
		sd = &tmpSD
	} else if errors.Is(err, fto.ErrNoSmallestDenom) {
		// This is fine, just means we don't have one yet.
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, struct {
		TrainingMaxes []*fto.TrainingMax
		SmallestDenom *fto.Weight
	}{tms, sd})
}

func jsonResp(w http.ResponseWriter, resp interface{}) {
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) serveSetTrainingMaxes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	type tmReq struct {
		PressTM       string `json:"OverheadPress"`
		SquatTM       string `json:"Squat"`
		BenchTM       string `json:"BenchPress"`
		DeadliftTM    string `json:"Deadlift"`
		SmallestDenom string `json:"SmallestDenom"`
	}

	// We assume the client returns the units in pounds.
	var req tmReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var err error
	parseLocalTM := func(in string) fto.Weight {
		if err != nil {
			return fto.Weight{}
		}

		var w fto.Weight
		if w, err = parsePounds(in); err != nil {
			return fto.Weight{}
		}

		return w
	}

	press := parseLocalTM(req.PressTM)
	squat := parseLocalTM(req.SquatTM)
	bench := parseLocalTM(req.BenchTM)
	deadlift := parseLocalTM(req.DeadliftTM)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse weights: %v", err), http.StatusBadRequest)
		return
	}

	var smallestDenom fto.Weight
	switch req.SmallestDenom {
	case "1.25":
		smallestDenom = fto.Weight{Value: 25, Unit: fto.DeciPounds}
	case "2.5":
		smallestDenom = fto.Weight{Value: 50, Unit: fto.DeciPounds}
	case "5":
		smallestDenom = fto.Weight{Value: 100, Unit: fto.DeciPounds}
	default:
		// Unexpected, and bad.
		http.Error(w, fmt.Sprintf("invalid smallest_denom %q", req.SmallestDenom), http.StatusBadRequest)
		return
	}

	if err := s.db.SetTrainingMaxes(press, squat, bench, deadlift); err != nil {
		http.Error(w, fmt.Sprintf("failed to set training maxes: %v", err), http.StatusInternalServerError)
		return
	}

	if err := s.db.SetSmallestDenom(smallestDenom); err != nil {
		http.Error(w, fmt.Sprintf("failed to set smallest denom: %v", err), http.StatusInternalServerError)
		return
	}
}

// parsePounds takes in a string, like 177.5, and converts it to a deci-pound
// weight, like fto.Weight{Unit: fto.DeciPounds, Value: 1775}
func parsePounds(in string) (fto.Weight, error) {
	var wholeStr, fracStr string
	if idx := strings.Index(in, "."); idx > -1 {
		wholeStr, fracStr = in[:idx], in[idx+1:]
	} else {
		wholeStr = in
	}

	var (
		whole, frac int
		err         error
	)

	if wholeStr != "" {
		if whole, err = strconv.Atoi(wholeStr); err != nil {
			return fto.Weight{}, fmt.Errorf("failed to parse whole portion %q: %w", wholeStr, err)
		}
		if whole < 0 {
			return fto.Weight{}, fmt.Errorf("weight can't be negative, was %d", whole)
		}
	}

	if fracStr != "" {
		if frac, err = strconv.Atoi(fracStr); err != nil {
			return fto.Weight{}, fmt.Errorf("failed to parse fractional portion %q: %w", fracStr, err)
		}
		if frac < 0 {
			return fto.Weight{}, fmt.Errorf("weight can't be negative, was %d", frac)
		}
		if frac > 9 {
			return fto.Weight{}, fmt.Errorf("fractional part can only contain one digit, was %d", frac)
		}
	}

	return fto.Weight{
		Unit:  fto.DeciPounds,
		Value: whole*10 + frac,
	}, nil
}

func (s *Server) serveNextLift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	s.nextLiftResponse(w)
}

type nextLiftResp struct {
	DayNumber         int
	WeekNumber        int
	IterationNumber   int
	DayName           string
	WeekName          string
	Workout           []*fto.Movement
	NextMovementIndex int
	NextSetIndex      int
	OptionalWeek      bool
}

func (s *Server) nextLiftResponse(w http.ResponseWriter) {
	nextLift, err := s.nextLift()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, nextLift)
}

func (s *Server) nextLift() (*nextLiftResp, error) {
	// Now the tricky part - we need to figure out the last one that a user
	// actually completed. Here's our strategy for doing so
	//  1. Load the users 20 latest lifts, ordered by iteration, then week, then day.
	//  2. Correlate that with the routine, using ~~magic~~ (read: bad and hacky heuristics)
	lifts, err := s.db.RecentLifts()
	if err != nil {
		return nil, fmt.Errorf("failed to load recent lifts: %w", err)
	}

	skipWeeks, err := s.db.SkippedWeeks()
	if err != nil {
		return nil, fmt.Errorf("failed to load skipped weeks: %w", err)
	}
	type weekIter struct{ week, iteration int }
	swm := make(map[weekIter]bool)
	for _, sw := range skipWeeks {
		swm[weekIter{week: sw.Week, iteration: sw.Iteration}] = true
	}

	// Load the latest day
	var day, week, iter int
	if len(lifts) > 0 {
		latest := lifts[0]
		day, week, iter = latest.DayNumber, latest.WeekNumber, latest.IterationNumber
	}

	routine := s.routine

	// Load the day from the routine.
	if week >= len(routine.Weeks) {
		return nil, fmt.Errorf("lift was for week %d that doesn't exist in routine", week)
	}

	if day >= len(routine.Weeks[week].Days) {
		return nil, fmt.Errorf("lift was for day %d (week %d) that doesn't exist in routine", day, week)
	}

	dayRoutine := routine.Weeks[week].Days[day]

	// Now we need to figure out if we finished the day's lifts or not.
	dayLifts := filterLifts(lifts, day, week, iter)
	set := lastSetDone(day, week, iter, dayLifts, dayRoutine)

	// Go to the next set in the movement if we have one.
	// If not, go to the next movement in the routine if we have one.
	// If not, go to the next day in the week if we have one.
	// If not, go to the next week in the iteration if we have one.
	// If not, go to the next iteration, which we can always do.
	if set.SetIndex < len(dayRoutine.Movements[set.MovementIndex].Sets)-1 {
		if !set.NoneDone {
			set.SetIndex++
		}
	} else if set.MovementIndex < len(dayRoutine.Movements)-1 {
		set.SetIndex = 0
		set.MovementIndex++
	} else if day < len(routine.Weeks[week].Days)-1 {
		set.SetIndex = 0
		set.MovementIndex = 0
		day++
	} else if week < len(routine.Weeks)-1 {
		set.SetIndex = 0
		set.MovementIndex = 0
		day = 0
		week++
	} else {
		set.SetIndex = 0
		set.MovementIndex = 0
		day = 0
		week = 0
		iter++
	}

	// Update our day routine, which may very well have changed.
	dayRoutine = routine.Weeks[week].Days[day]

	// If "the next thing" is a week we skipped, go straight to the next week or iter
	if swm[weekIter{week: week, iteration: iter}] {
		if week < len(routine.Weeks)-1 {
			set.SetIndex = 0
			set.MovementIndex = 0
			day = 0
			week++
		} else {
			set.SetIndex = 0
			set.MovementIndex = 0
			day = 0
			week = 0
			iter++
		}
	}

	// Now, load the smallest denom and training maxes, to set the target weights.
	tms, err := s.db.TrainingMaxes()
	if err != nil {
		return nil, fmt.Errorf("failed to load training maxes: %w", err)
	}

	getTM := func(ex fto.Exercise) (fto.Weight, bool) {
		for _, tm := range tms {
			if tm.Exercise == ex {
				return tm.Max, true
			}
		}
		return fto.Weight{}, false
	}

	smallest, err := s.db.SmallestDenom()
	if err != nil {
		return nil, fmt.Errorf("failed to load smallest denom: %w", err)
	}

	mvmts := dayRoutine.Clone().Movements
	for _, mvmt := range mvmts {
		tm, ok := getTM(mvmt.Exercise)
		if !ok {
			// Just skip this one if we didn't set it.
			continue
		}
		for _, set := range mvmt.Sets {
			set.WeightTarget = roundWeight(tm, set.TrainingMaxPercentage, smallest)
			if !set.ToFailure {
				continue
			}
			comparables, err := s.db.ComparableLifts(mvmt.Exercise, set.WeightTarget)
			if err != nil {
				return nil, fmt.Errorf("failed to load comparables: %w", err)
			}
			set.FailureComparables = comparables
		}
	}

	// For JSON serialization
	if mvmts == nil {
		mvmts = []*fto.Movement{}
	}

	return &nextLiftResp{
		DayNumber:         day,
		WeekNumber:        week,
		IterationNumber:   iter,
		DayName:           dayRoutine.DayName,
		WeekName:          routine.Weeks[week].WeekName,
		Workout:           mvmts,
		NextMovementIndex: set.MovementIndex,
		NextSetIndex:      set.SetIndex,
		OptionalWeek:      day == 0 && set.MovementIndex == 0 && set.SetIndex == 0 && routine.Weeks[week].Optional,
	}, nil
}

// roundWeight returns the percentage of the training max rounded to the
// smallest weights you can use. If we're equally distant between two options,
// we round up to get the most jacked.
// E.g. roundWeight(1750DLB, 65%, 25DLB) = 1150DLB
func roundWeight(trainingMax fto.Weight, percent int, smallestDenom fto.Weight) fto.Weight {
	if trainingMax.Unit != smallestDenom.Unit {
		panic(fmt.Sprintf("mismatched units %q and %q for training max and smallest denom", trainingMax.Unit, smallestDenom.Unit))
	}

	v := float64(trainingMax.Value) * float64(percent) / 100

	// Find the nearest multiples above and below by dividing, truncating, and
	// multiplying.
	trunc := int(v / float64(smallestDenom.Value))
	lower := trunc * smallestDenom.Value
	upper := (trunc + 1) * smallestDenom.Value
	if v-float64(lower) < float64(upper)-v {
		return fto.Weight{Value: lower, Unit: trainingMax.Unit}
	} else {
		return fto.Weight{Value: upper, Unit: trainingMax.Unit}
	}
}

type recordReq struct {
	Exercise  fto.Exercise `json:"Exercise"`
	SetType   fto.SetType  `json:"SetType"`
	Weight    string       `json:"Weight"`
	Set       int          `json:"Set"`
	Reps      int          `json:"Reps"`
	Note      string       `json:"Note"`
	Day       int          `json:"Day"`
	Week      int          `json:"Week"`
	Iteration int          `json:"Iteration"`
	ToFailure bool         `json:"ToFailure"`
}

func (s *Server) serveRecordLift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// We assume the client returns the units in pounds.
	var req recordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	weight, err := parsePounds(req.Weight)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse weights: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.db.RecordLift(req.Exercise, req.SetType, weight, req.Set, req.Reps, req.Note, req.Day, req.Week, req.Iteration, req.ToFailure); err != nil {
		http.Error(w, fmt.Sprintf("failed to record lift: %v", err), http.StatusInternalServerError)
		return
	}

	s.nextLiftResponse(w)
}

func (s *Server) skipOptionalWeek(w http.ResponseWriter, r *http.Request) {
	nextLift, err := s.nextLift()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type skipReq struct {
		Week      int    `json:"Week"`
		Iteration int    `json:"Iteration"`
		Note      string `json:"Note"`
	}
	var req skipReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if !nextLift.OptionalWeek {
		http.Error(w, "next lift isn't the start of an optional week", http.StatusBadRequest)
		return
	}

	if err := s.db.SkipWeek(req.Note, req.Week, req.Iteration); err != nil {
		http.Error(w, fmt.Sprintf("failed to skip week: %v", err), http.StatusInternalServerError)
		return
	}

	s.nextLiftResponse(w)
}

type lastSet struct {
	MovementIndex int
	SetIndex      int
	NoneDone      bool
}

func lastSetDone(day, week, iter int, lifts []*fto.Lift, dayRoutine *fto.WorkoutDay) lastSet {
	// If we have no recorded lifts for the day, it's safe to say the first
	// movement to do is the first movement we have.
	if len(lifts) == 0 {
		return lastSet{NoneDone: true}
	}

	// We want to match up lifts with our workout to see where we are.
	idx := len(lifts) - 1
	for i, mvmt := range dayRoutine.Movements {
		// Note that we don't actually look at the set info (reps, failure, etc),
		// moreso just the number of sets because there are lots of practical
		// reasons that those things might not match up.
		for j := range mvmt.Sets {
			lift := lifts[idx]

			// See if the recorded lift matches this.
			// If it doesn't, we just skip forward to the next exercise of the day.
			if lift.Exercise != mvmt.Exercise {
				break
			}
			if lift.SetType != mvmt.SetType {
				break
			}

			// If the set type and exercise match, there's a good chance that this
			// lift corresponds to a set of this routine.
			idx--
			if idx < 0 {
				// We've gone through all recorded lifts, meaning that this is the last
				// set we did.
				return lastSet{
					MovementIndex: i,
					SetIndex:      j,
					NoneDone:      false,
				}
			}
		}
	}

	// If we're here, we had lifts that we hadn't looked at, but we went through
	// all the movements. I don't think this should happen, but I guess it means
	// we're done with the day?
	lastMvmt := dayRoutine.Movements[len(dayRoutine.Movements)-1]
	return lastSet{
		MovementIndex: len(dayRoutine.Movements) - 1,
		SetIndex:      len(lastMvmt.Sets) - 1,
		NoneDone:      false,
	}
}

func filterLifts(lifts []*fto.Lift, day, week, iter int) []*fto.Lift {
	var out []*fto.Lift
	for _, lift := range lifts {
		if lift.DayNumber == day && lift.WeekNumber == week && lift.IterationNumber == iter {
			out = append(out, lift)
		}
	}
	return out
}
