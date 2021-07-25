package server

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	RecordLift(uID fto.UserID, ex fto.Exercise, st fto.SetType, weight fto.Weight, set int, reps int, note string) error
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
	mux.HandleFunc("/user", s.serveUser)

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

	value := map[string]string{
		"name": name,
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
	cookie, err := r.Cookie("auth")
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	value := make(map[string]string)
	if err := s.cookies.Decode("auth", cookie.Value, &value); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	fmt.Fprintf(w, "The value of name is %q", value["name"])
}
