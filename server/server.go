package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	CreateUser(name string) (fto.UserID, error)
	User(id fto.UserID) (*fto.User, error)
	UserByName(name string) (*fto.User, error)

	RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string, day, week, iter int) error
	SetTrainingMaxes(uID fto.UserID, press, squat, bench, deadlift fto.Weight) error
	TrainingMaxes(uID fto.UserID) ([]*fto.TrainingMax, error)
	RecentLifts(uID fto.UserID) ([]*fto.Lift, error)
}

type User struct {
	Name    string       `json:"name"`
	Routine *fto.Routine `json:"routine"`
}

type Server struct {
	mux *http.ServeMux

	users   map[string]*User
	cookies SecureCookie
	db      DB

	staticFrontendDir string
}

func New(users map[string]*User, sc SecureCookie, db DB, staticFrontendDir string) *Server {
	s := &Server{
		users:   users,
		cookies: sc,
		db:      db,

		staticFrontendDir: staticFrontendDir,
	}
	s.initMux()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) initMux() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.serveLogin)
	mux.HandleFunc("/api/user", s.serveUser)
	mux.HandleFunc("/api/setTrainingMaxes", s.serveSetTrainingMaxes)
	mux.HandleFunc("/api/nextLift", s.serveNextLift)

	if s.staticFrontendDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.staticFrontendDir)))
	}

	s.mux = mux
}

func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user, ok := s.users[req.Password]
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	usr, err := s.db.UserByName(user.Name)
	if errors.Is(err, fto.ErrUserNotFound) {
		uID, err := s.db.CreateUser(user.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		usr = &fto.User{ID: uID, Name: user.Name}
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("ROUTINE", user.Routine.Name)
	for _, wk := range user.Routine.Weeks {
		fmt.Println("WEEK", wk.WeekName)
		for _, dy := range wk.Days {
			fmt.Println("DAY", dy.DayOfWeek.String())
			for _, mvmt := range dy.Movements {
				fmt.Println("MOVEMENT", mvmt.Exercise, mvmt.SetType, len(mvmt.Sets))
			}
		}
	}

	value := map[string]string{
		"user_id": usr.ID.String(),
	}
	encoded, err := s.cookies.Encode("auth", value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cookie := &http.Cookie{
		Name:     "auth",
		Value:    encoded,
		Path:     "/",
		Secure:   s.cookies.UseSecure(),
		HttpOnly: true,
		Expires:  time.Now().Add(time.Hour * 24 * 365 * 100),
	}
	http.SetCookie(w, cookie)
}

func (s *Server) serveUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	uID, err := s.authFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	u, err := s.db.User(uID)
	if errors.Is(err, fto.ErrUserNotFound) {
		// Treat this as though they aren't logged in.
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tms, err := s.db.TrainingMaxes(uID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, struct {
		User          *fto.User
		TrainingMaxes []*fto.TrainingMax
	}{u, tms})
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

	u, err := s.userFromRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load user from request: %v", err), http.StatusUnauthorized)
		return
	}

	type tmReq struct {
		PressTM    string `json:"overhead_press"`
		SquatTM    string `json:"squat"`
		BenchTM    string `json:"bench_press"`
		DeadliftTM string `json:"deadlift"`
	}

	// We assume the client returns the units in deci-pounds for us
	var req tmReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	parseLocalTM := func(in string) fto.Weight {
		if err != nil {
			return fto.Weight{}
		}

		var w fto.Weight
		if w, err = parseTM(in); err != nil {
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

	if err := s.db.SetTrainingMaxes(u.ID, press, squat, bench, deadlift); err != nil {
		http.Error(w, fmt.Sprintf("failed to set training maxes: %v", err), http.StatusInternalServerError)
		return
	}
}

// parseTM takes in a string, like 177.5, and converts it to a deci-pound
// weight, like fto.Weight{Unit: fto.DeciPounds, Value: 1775}
func parseTM(in string) (fto.Weight, error) {
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

	u, err := s.userFromRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load user from request: %v", err), http.StatusUnauthorized)
		return
	}

	// Load the user's routine
	var user *User
	for _, srvUser := range s.users {
		if srvUser.Name == u.Name {
			user = srvUser
		}
	}

	if user == nil {
		http.Error(w, "No user/routine found", http.StatusBadRequest)
		return
	}

	// Now the tricky part - we need to figure out the last one that a user
	// actually completed. Here's our strategy for doing so
	//  1. Load the users 20 latest lifts, ordered by iteration, then week, then day.
	//  2. Correlate that with the routine, using ~~magic~~ (read: bad and hacky heuristics)
	lifts, err := s.db.RecentLifts(u.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load recent lifts: %v", err), http.StatusInternalServerError)
		return
	}

	// Load the latest day
	var day, week, iter int
	if len(lifts) > 0 {
		latest := lifts[0]
		day, week, iter = latest.DayNumber, latest.WeekNumber, latest.IterationNumber
	}

	// Load the day from the routine.
	if week >= len(user.Routine.Weeks) {
		http.Error(w, "Lift was for week that doesn't exist in routine", http.StatusBadRequest)
		return
	}

	if day >= len(user.Routine.Weeks[week].Days) {
		http.Error(w, "Lift was for day that doesn't exist in routine", http.StatusBadRequest)
		return
	}

	dayRoutine := user.Routine.Weeks[week].Days[day]

	type nextLiftResp struct {
		DayNumber         int
		WeekNumber        int
		IterationNumber   int
		DayName           string
		WeekName          string
		Workout           []*fto.Movement
		NextMovementIndex int
		NextSetIndex      int
	}

	// Now we need to figure out if we finished the day's lifts or not.
	dayLifts := filterLifts(lifts, day, week, iter)
	set := lastSetDone(day, week, iter, dayLifts, dayRoutine)

	// Go to the next set in the movement if we have one.
	// If not, go to the next movement in the routine if we have one.
	// If not, go to the next day in the week if we have one.
	// If not, go to the next week in the iteration if we have one.
	// If not, go to the next iteration, which we can always do.
	if set.SetIndex < len(dayRoutine.Movements[set.MovementIndex].Sets)-1 {
		set.SetIndex++
	} else if set.MovementIndex < len(dayRoutine.Movements)-1 {
		set.SetIndex = 0
		set.MovementIndex++
	} else if day < len(user.Routine.Weeks[week].Days)-1 {
		set.SetIndex = 0
		set.MovementIndex = 0
		day++
	} else if week < len(user.Routine.Weeks)-1 {
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

	jsonResp(w, nextLiftResp{
		DayNumber:         day,
		WeekNumber:        week,
		IterationNumber:   iter,
		DayName:           dayRoutine.DayName,
		WeekName:          user.Routine.Weeks[week].WeekName,
		Workout:           dayRoutine.Movements,
		NextMovementIndex: set.MovementIndex,
		NextSetIndex:      set.SetIndex,
	})
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
		for j, _ := range mvmt.Sets {
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

func (s *Server) authFromRequest(r *http.Request) (fto.UserID, error) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		return 0, fmt.Errorf("failed to load 'auth' cookie from request: %w", err)
	}

	value := make(map[string]string)
	if err := s.cookies.Decode("auth", cookie.Value, &value); err != nil {
		return 0, fmt.Errorf("failed to decode 'auth' cookie: %w", err)
	}

	idStr, ok := value["user_id"]
	if !ok {
		return 0, errors.New("no user ID was found in 'auth' cookie")
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID in cookie: %w", err)
	}

	return fto.UserID(id), nil
}

func (s *Server) userFromRequest(r *http.Request) (*fto.User, error) {
	uID, err := s.authFromRequest(r)
	if err != nil {
		return nil, fmt.Errorf("failed to load auth from request: %w", err)
	}

	u, err := s.db.User(uID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user with ID %d: %w", uID, err)
	}

	return u, nil
}
