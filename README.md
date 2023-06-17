# stronk

Stronk is a simple web app for tracking your exercise. It was built with [Jim Wendler's 5/3/1](https://www.amazon.com/Simplest-Effective-Training-System-Strength/dp/B00686OYGQ) in mind, but is decently flexible (at least in terms of routines, the UI is pretty heavily based around the main four 5/3/1 lifts).

Your exercises are configured via a `routine.json` file that specifies lifts in a rough hierarchy:

* Week - A week of workouts. A week can be 'optional'. This is used for deload weeks.
* Day - A day of workouts
* Movement - A set of lifts, all having the same exercise (e.g. squat, bench) and set type (e.g. warmup, assistance, etc)
* Set - A number of target reps at a target percentage of the training max for that movement's exercise. Can optionally be 'to failure', meaning the rep target is a minimum

An example `routine.example.json` is included, which implements a fairly standard 5/3/1 using "Big but Boring" for the assistance work. It includes an optional deload week.

## Screenshots

The training max page, where you enter your initial training maxes, which all subsequent sets will be based on.

![Screenshot of the training max page, showing four inputs corresponding to the four lifts of 5/3/1, along with a selector for the smallest plate you have available at your gym](/screenshots/training-maxes.png)

The lift/main page, where you get an overview of the day's lifts, and record them as you do them, adding rep counts and notes as relevant.

![Screenshot of the lifts page, showing a series of sets broken into warmup, main, and assistance. The bottom half of the page shows buttons for recording lifts, adding notes, skipping, and more](/screenshots/lifts.png)


## Local Development

To run locally, you'll need a recent version of Go + some version of NPM. Install frontend dependencies (namely Svelte) with `cd frontend && npm install`.

Then, to run the infrastructure.

```bash
# Run the backend
go run .

# In another terminal
cd frontend
npm run dev
```

The server stores lift info in a SQLite database, which will be created + migrated on the first boot.

Frontend is available at `localhost:5173`, backend is `localhost:8080`.

## TODO

- [ ] Make a "back"/edit button
  - The button exists, but doesn't currently work
- [x] Make record button bigger, and farther away from skip
- [x] Add a feature to see similar "to failure" sets
- [x] Highlight personal records
  - Not done directly, but 'failure comparables' make it clear when you've made a new PR.
- [x] Make weight amount more prominent
- [x] Add support for offering to skip deload week
  - Will require adding new `routine.json` metadata