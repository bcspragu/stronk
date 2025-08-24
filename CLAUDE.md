# stronk

Stronk is a simple web app for tracking exercise, mostly around strength training and 5/3/1 lifting, but it's flexible.

## Architecture

- Background is written in Go, entrypoint is `cmd/server/main.go`.
- Frontend is written in [SvelteKit](https://svelte.dev/docs/kit/introduction)
- Database is SQLite
  - Uses `github.com/mattn/go-sqlite3`
  - And migrations are with `github.com/golang-migrate/migrate/v4`
  - DB layer is in `db/sqldb/`
  -migrations are in `db/sqldb/migrations/`
- Domain types are in `stronk.go`
- Server endpoints are in `server/server.go`

## Commands

- Run the backend: `go run ./cmd/server`
- Run the frontend: `cd frontend && npm run dev`
- Add a new database migration: `./scripts/new_migration.sh <name_of_migration>`
