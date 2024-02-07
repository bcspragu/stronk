package server

import (
	"encoding/json"
	"errors"
	"fmt"
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

	RecordLift(ex stronk.Exercise, st stronk.SetType, weight stronk.Weight, set int, reps int, note string, day, week, iter int, toFailure bool) (stronk.LiftID, error)

	Lift(id stronk.LiftID) (*stronk.Lift, error)
	EditLift(id stronk.LiftID, note string, reps int) error
	RecentLifts() ([]*stronk.Lift, error)
	ComparableLifts(ex stronk.Exercise, weight stronk.Weight) (*stronk.ComparableLifts, error)
	RecentFailureSets() ([]*stronk.Lift, error)
}

type Server struct {
	mux *http.ServeMux

	routine *stronk.Routine
	cookies SecureCookie
	db      DB
}

func New(routine *stronk.Routine, db DB) *Server {
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
		if highestIter == -1 {
			highestIter = iter
			continue
		}
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

func exerciseForDay(mvmts []*stronk.Movement) (stronk.Exercise, bool) {
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
	// Now the tricky part - we need to figure out the last one that a user
	// actually completed. Here's our strategy for doing so
	//  1. Load the users 20 latest lifts, ordered by iteration, then week, then day.
	//  2. Correlate that with the routine, using ~~magic~~ (read: bad and hacky heuristics)
	lifts, err := s.db.RecentLifts()
	if err != nil {
		return nil, fmt.Errorf("failed to load recent lifts: %w", err)
	}

	// Map iteration -> week -> day -> set type -> lifts
	m := make(map[int]map[int]map[int]map[stronk.SetType][]*stronk.Lift)
	for _, l := range lifts {
		wm, ok := m[l.IterationNumber]
		if !ok {
			wm = make(map[int]map[int]map[stronk.SetType][]*stronk.Lift)
		}
		dm, ok := wm[l.WeekNumber]
		if !ok {
			dm = make(map[int]map[stronk.SetType][]*stronk.Lift)
		}
		stm, ok := dm[l.DayNumber]
		if !ok {
			stm = make(map[stronk.SetType][]*stronk.Lift)
		}
		lfs := stm[l.SetType]
		lfs = append(lfs, l)
		stm[l.SetType] = lfs
		dm[l.DayNumber] = stm
		wm[l.WeekNumber] = dm
		m[l.IterationNumber] = wm
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

	// Update our day routine, which may very well have changed.
	dayRoutine = routine.Weeks[week].Days[day]

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

	associatedLift := func(st stronk.SetType, ex stronk.Exercise, setNum int) (stronk.LiftID, bool) {
		wm, ok := m[iter]
		if !ok {
			return 0, false
		}
		dm, ok := wm[week]
		if !ok {
			return 0, false
		}
		stm, ok := dm[day]
		if !ok {
			return 0, false
		}
		lfs, ok := stm[st]
		if !ok {
			return 0, false
		}
		for _, l := range lfs {
			if l.Exercise != ex {
				continue
			}
			if l.SetNumber != setNum {
				continue
			}
			return l.ID, true
		}
		return 0, false
	}

	mvmts := dayRoutine.Clone().Movements
	for _, mvmt := range mvmts {
		tm, ok := getTM(mvmt.Exercise)
		if !ok {
			// Just skip this one if we didn't set it.
			continue
		}
		for i, set := range mvmt.Sets {
			set.WeightTarget = roundWeight(tm, set.TrainingMaxPercentage, smallest)
			id, ok := associatedLift(mvmt.SetType, mvmt.Exercise, i)
			if ok {
				set.AssociatedLiftID = id
			}

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
		mvmts = []*stronk.Movement{}
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

	id, err := s.db.RecordLift(req.Exercise, req.SetType, weight, req.Set, req.Reps, req.Note, req.Day, req.Week, req.Iteration, req.ToFailure)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to record lift: %v", err), http.StatusInternalServerError)
		return
	}

	nextLift, err := s.nextLift()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, recordLiftResp{id, nextLift})
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
