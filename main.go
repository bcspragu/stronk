package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/securecookie"
	"github.com/lexacali/fivethreeone/db/sqldb"
	"github.com/lexacali/fivethreeone/server"
	"github.com/namsral/flag"
)

// cookies is a thin wrapper around *securecookie.SecureCookie that implements
// the server.SecureCookie interface.
type cookies struct {
	*securecookie.SecureCookie
	dev bool
}

func (c *cookies) UseSecure() bool {
	return !c.dev
}

func main() {
	var (
		usersFile    = flag.String("users_file", "users.json", "Path to the JSON file containing a mapping from password -> user name + routine")
		hashKeyFile  = flag.String("hash_key_file", "hashkey.dat", "Path to the file containing our secure cookie hash key")
		blockKeyFile = flag.String("block_key_file", "blockkey.dat", "Path to the file containing our secure cookie block key")
		dev          = flag.Bool("dev", true, "Whether or not we're running in dev mode")

		dbFile       = flag.String("db_file", "fivethreeone.db", "Path to the SQLite database")
		migrationDir = flag.String("migration_dir", "db/sqldb/migrations", "Path to the directory containing our migration set files")

		addr = flag.String("addr", ":8080", "The address to run the HTTP server on")
	)
	flag.Parse()

	users, err := loadUsers(*usersFile)
	if err != nil {
		log.Fatalf("failed to load users file: %v", err)
	}

	sc, err := loadSecureCookie(*hashKeyFile, *blockKeyFile)
	if err != nil {
		log.Fatalf("failed to load secure cookie: %v", err)
	}

	db, err := sqldb.New(*dbFile, *migrationDir)
	if err != nil {
		log.Fatalf("failed to load SQLite db: %v", err)
	}
	defer db.Close()

	var staticFrontendDir string
	if *dev {
		staticFrontendDir = "./frontend"
	}

	srv := server.New(users, &cookies{SecureCookie: sc, dev: *dev}, db, staticFrontendDir)

	log.Printf("Starting server on %q", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv))
}

func loadUsers(usersFile string) (map[string]*server.User, error) {
	f, err := os.Open(usersFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open users file: %w", err)
	}
	defer f.Close()

	var users map[string]*server.User
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
