package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"slices"

	"github.com/bcspragu/stronk"
)

// SecureCookie represents anything that knows how to encode and decode cookies
// intended to be stored in a user's browser.
type SecureCookie interface {
	Encode(name string, value interface{}) (string, error)
	Decode(name, value string, dst interface{}) error
	UseSecure() bool
}

type DB interface {
	SkippedWeeks() ([]stronk.SkippedWeek, error)
	SkipWeek(note string, week, iter int) error

	SetTrainingMaxes(press, squat, bench, deadlift stronk.Weight) error
	TrainingMaxes() ([]*stronk.TrainingMax, error)

	SetSmallestDenom(small stronk.Weight) error
	SmallestDenom() (stronk.Weight, error)

	// Routine management
	GetCurrentRoutine() (*stronk.StoredRoutine, error)
	StoreRoutine(routine *stronk.Routine) (*stronk.StoredRoutine, error)
	GetRoutineSet(id stronk.RoutineSetID) (*stronk.StoredRoutineSet, error)
	MigrateLiftsToNewFormat(storedRoutine *stronk.StoredRoutine) error

	// New lift methods
	RecordLift(routineSetID stronk.RoutineSetID, reps int, note string, weight stronk.Weight) (stronk.LiftID, error)
	EditLift(id stronk.LiftID, note string, reps int) error

	Lift(id stronk.LiftID) (*stronk.Lift, error)
	RecentLifts() ([]*stronk.Lift, error)
	LatestLift() (*stronk.Lift, error)
	LatestLiftPerSetID(setIDs []stronk.RoutineSetID) (map[stronk.RoutineSetID]*stronk.Lift, error)
	ComparableLifts(ex stronk.Exercise, weight stronk.Weight) (*stronk.ComparableLifts, error)
	RecentFailureSets() ([]*stronk.Lift, error)
}

type Server struct {
	mux *http.ServeMux

	routine *stronk.StoredRoutine
	cookies SecureCookie
	db      DB
}

func New(routine *stronk.Routine, db DB) *Server {
	s := &Server{
		db: db,
	}

	// Initialize the routine system
	storedRoutine, err := s.initRoutine(routine)
	if err != nil {
		// For now, we'll continue with the old system if initialization fails
		// In a real deployment, you might want to handle this differently
		log.Printf("Warning: Failed to initialize routine system: %v\n", err)
	}
	s.routine = storedRoutine

	s.initMux()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// initRoutine initializes the routine system by checking if the current routine
// needs to be stored in the database
func (s *Server) initRoutine(routine *stronk.Routine) (*stronk.StoredRoutine, error) {
	// Get the current stored routine
	storedRoutine, err := s.db.GetCurrentRoutine()
	if err != nil {
		return nil, fmt.Errorf("failed to get current routine: %w", err)
	}

	// If we have no stored routine, something went wrong with the DB migration.
	if storedRoutine == nil {
		return nil, errors.New("no routine stored in database, this shouldn't be possible")
	}

	// Check if the current routine differs from the stored one
	currentHash, err := routine.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to hash current routine: %w", err)
	}

	storedHash, err := storedRoutine.ToRoutine().Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to hash stored routine: %w", err)
	}

	// If these don't match, it means the user has uploaded a new routine, and we should store that.
	if currentHash != storedHash {
		log.Println("Detected change in routine, storing new one in DB")
		storedRoutine, err := s.db.StoreRoutine(routine)
		if err != nil {
			return nil, fmt.Errorf("failed to store updated routine: %w", err)
		}
		return storedRoutine, nil
	}

	// Use the existing stored routine
	return storedRoutine, nil
}

func (s *Server) initMux() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/trainingMaxes", s.serveTrainingMaxes)
	mux.HandleFunc("/api/setTrainingMaxes", s.serveSetTrainingMaxes)

	mux.HandleFunc("/api/nextLift", s.serveNextLift)
	mux.HandleFunc("/api/recordLift", s.serveRecordLift)
	mux.HandleFunc("/api/lift", s.serveLoadLift)
	mux.HandleFunc("/api/editLift", s.serveEditLift)

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
		tms = []*stronk.TrainingMax{}
	}

	var sd *stronk.Weight
	tmpSD, err := s.db.SmallestDenom()
	if err == nil {
		sd = &tmpSD
	} else if errors.Is(err, stronk.ErrNoSmallestDenom) {
		// This is fine, just means we don't have one yet.
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if sd != nil && sd.Unit != stronk.DeciPounds {
		http.Error(w, fmt.Sprintf("unexpected unit %q", sd.Unit), http.StatusInternalServerError)
		return
	}

	var sdStr string
	if sd != nil {
		switch sd.Value {
		case 25:
			sdStr = "1.25"
		case 50:
			sdStr = "2.5"
		case 100:
			sdStr = "5"
		default:
			http.Error(w, fmt.Sprintf("unexpected smallest denom %d", sd.Value), http.StatusInternalServerError)
			return
		}
	}

	// Load the most recent full cycle of failure sets.
	failureSets, err := s.db.RecentFailureSets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(failureSets) == 0 {
		// No data, just return what we've got I guess
		jsonResp(w, trainingMaxResp{TrainingMaxes: tms, SmallestDenom: sdStr})
		return
	}

	// Group by iteration number
	byIter := make(map[int][]*stronk.Lift)
	for _, set := range failureSets {
		byIter[set.IterationNumber] = append(byIter[set.IterationNumber], set)
	}

	highestIter := -1
	for iter := range byIter {
		if iter > highestIter {
			highestIter = iter
		}
	}

	minFailSet, maxFailSet := s.numFailureSets()

	// Means that iteration hasn't been completed yet
	if len(byIter[highestIter]) < minFailSet {
		// Try the previous one
		if len(byIter[highestIter-1]) >= minFailSet {
			highestIter = highestIter - 1
		} else {
			// Something is wonky
			http.Error(w, fmt.Sprintf("neither of last two iterations (%d, %d) was in range of expected number of failure sets (%d, %d)", highestIter, highestIter-1, minFailSet, maxFailSet), http.StatusInternalServerError)
			return
		}
	}

	// Use failures from the `highestIter`, group by week
	byWeek := make(map[int][]*stronk.Lift)
	for _, set := range byIter[highestIter] {
		byWeek[set.WeekNumber] = append(byWeek[set.WeekNumber], set)
	}

	type withWeek struct {
		weekNum int
		lifts   []*stronk.Lift
	}

	var wwks []withWeek
	for weekNum, lifts := range byWeek {
		wwks = append(wwks, withWeek{weekNum: weekNum, lifts: lifts})
	}

	slices.SortFunc(wwks, func(a, b withWeek) int { return a.weekNum - b.weekNum })

	liftOrder := s.liftOrder()
	// Now pull the weeks out of that slice.
	var fatigueWeeks [][]*stronk.Lift
	for _, wwk := range wwks {
		// Order each week by our usual lift order
		slices.SortFunc(wwk.lifts, func(a, b *stronk.Lift) int { return liftOrder[a.Exercise] - liftOrder[b.Exercise] })
		fatigueWeeks = append(fatigueWeeks, wwk.lifts)
	}

	jsonResp(w, trainingMaxResp{TrainingMaxes: tms, SmallestDenom: sdStr, LatestFailureSets: fatigueWeeks})
}

func (s *Server) liftOrder() map[stronk.Exercise]int {
	if len(s.routine.Weeks) == 0 {
		return make(map[stronk.Exercise]int)
	}

	// Only look at the first week
	week := s.routine.Weeks[0]

	m := make(map[stronk.Exercise]int)

	cnt := 0
	for _, day := range week.Days {
		// Only look at the main movement on each day.
		ex, ok := exerciseForDay(day.Movements)
		if !ok {
			return make(map[stronk.Exercise]int)
		}
		m[ex] = cnt
		cnt++
	}

	return m
}

func exerciseForDay(mvmts []*stronk.StoredRoutineMovement) (stronk.Exercise, bool) {
	for _, mvmt := range mvmts {
		if mvmt.SetType != stronk.Main {
			continue
		}
		return mvmt.Exercise, true
	}

	return "", false
}

func (s *Server) numFailureSets() (int, int) {
	v, opt := 0, 0
	for _, w := range s.routine.Weeks {
		for _, d := range w.Days {
			for _, m := range d.Movements {
				for _, s := range m.Sets {
					if s.ToFailure {
						if w.Optional {
							opt++
							continue
						}
						v++
					}
				}
			}
		}
	}
	return v, v + opt
}

type trainingMaxResp struct {
	TrainingMaxes []*stronk.TrainingMax
	SmallestDenom string
	// Grouped by week
	LatestFailureSets [][]*stronk.Lift
}

func (s *Server) serveLoadLift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()

	id, err := strconv.Atoi(q.Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lift, err := s.db.Lift(stronk.LiftID(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, lift)
}

func (s *Server) serveEditLift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	type editReq struct {
		ID   stronk.LiftID `json:"id"`
		Note string        `json:"note"`
		Reps int           `json:"reps"`
	}

	var req editReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	err := s.db.EditLift(req.ID, req.Note, req.Reps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
	parseLocalTM := func(in string) stronk.Weight {
		if err != nil {
			return stronk.Weight{}
		}

		var w stronk.Weight
		if w, err = parsePounds(in); err != nil {
			return stronk.Weight{}
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

	var smallestDenom stronk.Weight
	switch req.SmallestDenom {
	case "1.25":
		smallestDenom = stronk.Weight{Value: 25, Unit: stronk.DeciPounds}
	case "2.5":
		smallestDenom = stronk.Weight{Value: 50, Unit: stronk.DeciPounds}
	case "5":
		smallestDenom = stronk.Weight{Value: 100, Unit: stronk.DeciPounds}
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
// weight, like stronk.Weight{Unit: stronk.DeciPounds, Value: 1775}
func parsePounds(in string) (stronk.Weight, error) {
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
			return stronk.Weight{}, fmt.Errorf("failed to parse whole portion %q: %w", wholeStr, err)
		}
		if whole < 0 {
			return stronk.Weight{}, fmt.Errorf("weight can't be negative, was %d", whole)
		}
	}

	if fracStr != "" {
		if frac, err = strconv.Atoi(fracStr); err != nil {
			return stronk.Weight{}, fmt.Errorf("failed to parse fractional portion %q: %w", fracStr, err)
		}
		if frac < 0 {
			return stronk.Weight{}, fmt.Errorf("weight can't be negative, was %d", frac)
		}
		if frac > 9 {
			return stronk.Weight{}, fmt.Errorf("fractional part can only contain one digit, was %d", frac)
		}
	}

	return stronk.Weight{
		Unit:  stronk.DeciPounds,
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
	Workout           []*stronk.Movement
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
	// Just load the latest lift we've done.
	ll, err := s.db.LatestLift()
	if err != nil {
		return nil, fmt.Errorf("failed to load the latest lift: %w", err)
	}

	var (
		setIndex, movementIndex, dayIndex, weekIndex, iterIndex int
		setType                                                 stronk.SetType
	)
	if ll != nil {
		setIndex = ll.SetNumber
		dayIndex = ll.DayNumber
		weekIndex = ll.WeekNumber
		iterIndex = ll.IterationNumber
		setType = ll.SetType
	} else {
		setType = s.routine.Weeks[0].Days[0].Movements[0].SetType
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

	// Now, we calulate the next lift in our routine based on the latest one we've done.
	week := s.routine.Weeks[weekIndex]
	day := week.Days[dayIndex]
	var (
		smvmt *stronk.StoredRoutineMovement
	)
	for idx, mvmt := range day.Movements {
		if mvmt.SetType == setType {
			smvmt = mvmt
			movementIndex = idx
			break
		}
	}
	if smvmt == nil {
		return nil, fmt.Errorf("no movement found with set type %q on day %d in week %d", setType, dayIndex, weekIndex)
	}

	// We only increment if we have some previous lift, otherwise we want the actual
	// zeroth one.
	if ll != nil {
		// Go to the next set in the movement if we have one.
		// If not, go to the next movement in the routine if we have one.
		// If not, go to the next day in the week if we have one.
		// If not, go to the next week in the iteration if we have one.
		// If not, go to the next iteration, which we can always do.

		if setIndex < len(smvmt.Sets)-1 {
			setIndex++
		} else if movementIndex < len(day.Movements)-1 {
			setIndex = 0
			movementIndex++
		} else if dayIndex < len(week.Days)-1 {
			setIndex = 0
			movementIndex = 0
			dayIndex++
		} else if weekIndex < len(s.routine.Weeks)-1 {
			setIndex = 0
			movementIndex = 0
			dayIndex = 0
			weekIndex++
		} else {
			setIndex = 0
			movementIndex = 0
			dayIndex = 0
			weekIndex = 0
			iterIndex++
		}
	}

	// If "the next thing" is a week we skipped, go straight to the next week or iter
	if swm[weekIter{week: weekIndex, iteration: iterIndex}] {
		if weekIndex < len(s.routine.Weeks)-1 {
			setIndex = 0
			movementIndex = 0
			dayIndex = 0
			weekIndex++
		} else {
			setIndex = 0
			movementIndex = 0
			dayIndex = 0
			weekIndex = 0
			iterIndex++
		}
	}

	// Update our day + week bits, which may very well have changed.
	week = s.routine.Weeks[weekIndex]
	day = week.Days[dayIndex]

	// Now, load the smallest denom and training maxes, to set the target weights.
	tms, err := s.db.TrainingMaxes()
	if err != nil {
		return nil, fmt.Errorf("failed to load training maxes: %w", err)
	}

	getTM := func(ex stronk.Exercise) (stronk.Weight, bool) {
		for _, tm := range tms {
			if tm.Exercise == ex {
				return tm.Max, true
			}
		}
		return stronk.Weight{}, false
	}

	smallest, err := s.db.SmallestDenom()
	if err != nil {
		return nil, fmt.Errorf("failed to load smallest denom: %w", err)
	}

	var setIDs []stronk.RoutineSetID
	for _, mvmt := range day.Movements {
		for _, set := range mvmt.Sets {
			setIDs = append(setIDs, set.ID)
		}
	}

	liftPerSetID, err := s.db.LatestLiftPerSetID(setIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load latest lift per set ID: %w", err)
	}

	associatedLiftID := func(id stronk.RoutineSetID) stronk.LiftID {
		lift, ok := liftPerSetID[id]
		if !ok {
			return 0
		}
		if lift.IterationNumber != iterIndex {
			return 0
		}
		return lift.ID
	}

	var workoutMvmts []*stronk.Movement
	for _, mvmt := range day.Movements {
		tm, ok := getTM(mvmt.Exercise)
		if !ok {
			// Just skip this one if we didn't set it.
			continue
		}
		var sets []*stronk.Set
		for _, set := range mvmt.Sets {
			weightTarget := roundWeight(tm, set.TrainingMaxPercentage, smallest)

			var comparables *stronk.ComparableLifts
			if set.ToFailure {
				if comparables, err = s.db.ComparableLifts(mvmt.Exercise, weightTarget); err != nil {
					return nil, fmt.Errorf("failed to load comparables: %w", err)
				}
			}
			sets = append(sets, &stronk.Set{
				RepTarget:             set.RepTarget,
				ToFailure:             set.ToFailure,
				TrainingMaxPercentage: set.TrainingMaxPercentage,
				WeightTarget:          weightTarget,
				FailureComparables:    comparables,
				AssociatedLiftID:      associatedLiftID(set.ID),
			})
		}
		workoutMvmts = append(workoutMvmts, &stronk.Movement{
			Exercise: mvmt.Exercise,
			SetType:  mvmt.SetType,
			Sets:     sets,
		})
	}

	return &nextLiftResp{
		DayNumber:         dayIndex,
		WeekNumber:        weekIndex,
		IterationNumber:   iterIndex,
		DayName:           day.DayName,
		WeekName:          week.WeekName,
		Workout:           workoutMvmts,
		NextMovementIndex: movementIndex,
		NextSetIndex:      setIndex,
		OptionalWeek:      dayIndex == 0 && movementIndex == 0 && setIndex == 0 && week.Optional,
	}, nil
}

// roundWeight returns the percentage of the training max rounded to the
// smallest weights you can use. If we're equally distant between two options,
// we round up to get the most jacked.
// E.g. roundWeight(1750DLB, 65%, 25DLB) = 1150DLB
func roundWeight(trainingMax stronk.Weight, percent int, smallestDenom stronk.Weight) stronk.Weight {
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
		return stronk.Weight{Value: lower, Unit: trainingMax.Unit}
	} else {
		return stronk.Weight{Value: upper, Unit: trainingMax.Unit}
	}
}

type recordReq struct {
	Exercise  stronk.Exercise `json:"Exercise"`
	SetType   stronk.SetType  `json:"SetType"`
	Weight    string          `json:"Weight"`
	Set       int             `json:"Set"`
	Reps      int             `json:"Reps"`
	Note      string          `json:"Note"`
	Day       int             `json:"Day"`
	Week      int             `json:"Week"`
	Iteration int             `json:"Iteration"`
	ToFailure bool            `json:"ToFailure"`
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

	// Find the routine set that matches this lift
	routineSet := s.routine.FindRoutineSet(req.Week, req.Day,
		s.findMovementIndex(req.Week, req.Day, req.Exercise, req.SetType),
		req.Set)

	if routineSet == nil {
		http.Error(w, "failed to load the relevant routine set", http.StatusInternalServerError)
		return
	}

	liftID, err := s.db.RecordLift(routineSet.ID, req.Reps, req.Note, weight)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to record lift: %v", err), http.StatusInternalServerError)
		return
	}

	nextLift, err := s.nextLift()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, recordLiftResp{liftID, nextLift})
}

// findMovementIndex finds the index of the movement within a day that matches the given exercise and set type
func (s *Server) findMovementIndex(weekIdx, dayIdx int, exercise stronk.Exercise, setType stronk.SetType) int {
	if weekIdx >= len(s.routine.Weeks) {
		return -1
	}
	week := s.routine.Weeks[weekIdx]

	if dayIdx >= len(week.Days) {
		return -1
	}
	day := week.Days[dayIdx]

	for i, movement := range day.Movements {
		if movement.Exercise == exercise && movement.SetType == setType {
			return i
		}
	}
	return -1
}

type recordLiftResp struct {
	LiftID   stronk.LiftID
	NextLift *nextLiftResp
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

func lastSetDone(day, week, iter int, lifts []*stronk.Lift, dayRoutine *stronk.WorkoutDay) lastSet {
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

func filterLifts(lifts []*stronk.Lift, day, week, iter int) []*stronk.Lift {
	var out []*stronk.Lift
	for _, lift := range lifts {
		if lift.DayNumber == day && lift.WeekNumber == week && lift.IterationNumber == iter {
			out = append(out, lift)
		}
	}
	return out
}
