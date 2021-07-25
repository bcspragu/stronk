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

	RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string) error
	SetTrainingMaxes(uID fto.UserID, press, squat, bench, deadlift fto.Weight) error
	TrainingMaxes(uID fto.UserID) ([]*fto.TrainingMax, error)
}

type Server struct {
	mux *http.ServeMux

	users   map[string]string
	cookies SecureCookie
	db      DB

	staticFrontendDir string
}

func New(users map[string]string, sc SecureCookie, db DB, staticFrontendDir string) *Server {
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

	name, ok := s.users[req.Password]
	if !ok {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	user, err := s.db.UserByName(name)
	if errors.Is(err, fto.ErrUserNotFound) {
		uID, err := s.db.CreateUser(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		user = &fto.User{ID: uID, Name: name}
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	value := map[string]string{
		"user_id": user.ID.String(),
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
