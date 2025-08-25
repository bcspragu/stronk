# Changelog

A list of important changes. Anything that requires user intervention is labelled as **BREAKING**. Changes labelled **NOTE** don't require intervention, but there may be things you _can_ do to improve things in some way.

## 2025-08-25

- **NOTE** [Commit fc37ad3](https://github.com/bcspragu/stronk/commit/fc37ad318ece607128f2ea77739fc57554fbf5a3) added support for modifying routines. This migration will happen transparently and automatically when the database is booted, just don't modify the `routine.json` until you've started the new version at least once (so that the migration has happened and the system is capable of supporting the new routine file).

## 2025-08-20

- **NOTE** [Commit fb9a302](https://github.com/bcspragu/stronk/commit/fb9a30226b84a7c2da0190d3acf64add276b8c9e) changed some `PRAGMA` settings for the SQLite database. It changes the `auto_vacuum` mode, which really doesn't matter because we don't have any `DELETE` calls, but to actually enable it you'll need to manually run `VACUUM` against the SQLite database after the pragmas have run (see [the SQLite docs](https://www.sqlite.org/pragma.html#pragma_auto_vacuum) for more details)
- [Commit 0246b2c](https://github.com/bcspragu/stronk/commit/0246b2cb72fb4242f2330fa3e621d83f7eeea72d) migrates the `lifts` table to use millisecond precision. This change should be transparent to users and require no action.

## 2025-07-31

- **BREAKING** [Commit 6b4f33f](https://github.com/bcspragu/stronk/commit/6b4f33f483ba57d9d64df898134b7b5088cb8933) changed how `.env` vars work for local dev + deployment, check the commit message for details if you're upgrading.
