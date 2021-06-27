package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/namsral/flag"
)

func main() {
	var (
		usersFile    = flag.String("users_file", "users.json", "Path to the JSON file containing a mapping from password -> user name")
		hashKeyFile  = flag.String("hash_key_file", "hashkey.dat", "Path to the file containing our secure cookie hash key")
		blockKeyFile = flag.String("block_key_file", "blockkey.dat", "Path to the file containing our secure cookie block key")
		dev          = flag.Bool("dev", true, "Whether or not we're running in dev mode")

		addr = flag.String("addr", ":8080", "The address to run the HTTP server on")
	)
	flag.Parse()

	users, err := loadUsers(*usersFile)
	if err != nil {
		log.Fatalf("failed to load users file: %v", err)
	}

	s, err := loadSecureCookie(*hashKeyFile, *blockKeyFile)
	if err != nil {
		log.Fatalf("failed to load secure cookie: %v", err)
	}

	if *dev {
		http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("./frontend"))))
	}

	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
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
			Secure:   !*dev,
			HttpOnly: true,
			Expires:  time.Now().Add(time.Hour * 24 * 365 * 100),
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

	log.Fatal(http.ListenAndServe(*addr, nil))
}

func loadUsers(usersFile string) (map[string]string, error) {
	f, err := os.Open(usersFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open users file: %w", err)
	}
	defer f.Close()

	var users map[string]string
	if err := json.NewDecoder(f).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to parse users file as JSON: %w", err)
	}
	return users, nil
}

func loadSecureCookie(hashKeyFile, blockKeyFile string) (*securecookie.SecureCookie, error) {
	hashKey, err := loadOrCreateKey(hashKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load or create hash key: %w", err)
	}
	blockKey, err := loadOrCreateKey(blockKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load or create block key: %w", err)
	}
	return securecookie.New(hashKey, blockKey), nil
}

func loadOrCreateKey(keyPath string) ([]byte, error) {
	f, err := os.Open(keyPath)
	if os.IsNotExist(err) {
		return createKey(keyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load key from %q: %w", keyPath, err)
	}
	defer f.Close()

	// If we're here, the key exists and we should load it.
	dat, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read existing key from %q: %w", keyPath, err)
	}

	return dat, nil
}

func createKey(keyPath string) ([]byte, error) {
	dat := securecookie.GenerateRandomKey(32)
	if err := ioutil.WriteFile(keyPath, dat, 0700); err != nil {
		return nil, fmt.Errorf("failed to write key file %q: %w", keyPath, err)
	}
	return dat, nil
}
