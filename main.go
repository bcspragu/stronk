package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/securecookie"
)

func main() {
	file, err := os.Open("users.json")
	if err != nil {
		log.Fatal(err)
	}

	var users map[string]string
	if err := json.NewDecoder(file).Decode(&users); err != nil {
		log.Fatal(err)
	}

	hashKey := securecookie.GenerateRandomKey(32)
	blockKey := securecookie.GenerateRandomKey(32)
	s := securecookie.New(hashKey, blockKey)

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
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

		name, ok := users[req.Password]
		if !ok {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		value := map[string]string{
			"name": name,
		}
		encoded, err := s.Encode("auth", value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cookie := &http.Cookie{
			Name:     "auth",
			Value:    encoded,
			Path:     "/",
			Secure:   false,
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
	})

	http.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth")
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		value := make(map[string]string)
		if err := s.Decode("auth", cookie.Value, &value); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		fmt.Fprintf(w, "The value of name is %q", value["name"])
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
