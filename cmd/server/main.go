package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/bcspragu/stronk/db/sqldb"
	"github.com/bcspragu/stronk"
	"github.com/bcspragu/stronk/server"
	"github.com/namsral/flag"
	"github.com/rs/cors"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var (
		routineFile = flag.String("routine_file", "routine.json", "Path to the JSON file containing a routine")

		dbFile       = flag.String("db_file", "stronk.db", "Path to the SQLite database")
		migrationDir = flag.String("migration_dir", "db/sqldb/migrations", "Path to the directory containing our migration set files")

		addr = flag.String("addr", ":8080", "The address to run the HTTP server on")
	)
	flag.Parse()

	routine, err := loadRoutine(*routineFile)
	if err != nil {
		return fmt.Errorf("failed to load users file: %v", err)
	}

	db, err := sqldb.New(*dbFile, *migrationDir)
	if err != nil {
		return fmt.Errorf("failed to load SQLite db: %v", err)
	}
	defer db.Close()

	srv := server.New(routine, db)

	errChan := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		s := <-c
		log.Printf("Received signal: %s", s)
		errChan <- nil
	}()

	go func() {
		log.Printf("Starting server on %q", *addr)
		errChan <- http.ListenAndServe(*addr, cors.Default().Handler(srv))
	}()

	return <-errChan
}

func loadRoutine(usersFile string) (*stronk.Routine, error) {
	f, err := os.Open(usersFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open routine file: %w", err)
	}
	defer f.Close()

	var routine *stronk.Routine
	if err := json.NewDecoder(f).Decode(&routine); err != nil {
		return nil, fmt.Errorf("failed to parse routine file as JSON: %w", err)
	}
	return routine, nil
}
