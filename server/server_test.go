package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lexacali/fivethreeone/fto"
	"github.com/lexacali/fivethreeone/testing/testdb"
)

func TestNextLift(t *testing.T) {
	srv, env := setup(t)

	setTMReq := `{
	"OverheadPress": "127.5",
	"Squat": "230",
	"BenchPress": "190",
	"Deadlift": "280",
	"SmallestDenom": "1.25"
}`

	// First, we set some training maxes.
	r := httptest.NewRequest(http.MethodPost, "/api/setTrainingMaxes", strings.NewReader(setTMReq))
	w := httptest.NewRecorder()
	srv.serveSetTrainingMaxes(w, r)

	resp := w.Result()
	if status := resp.StatusCode; status != http.StatusOK {
		t.Fatalf("unexpected response code from server %d, wanted OK", status)
	}

	checkLift := func(got, want nextLiftResp) {
		t.Helper()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected next lift returned (-want +got)\n%s", diff)
		}
	}

	checkAndParseNextLiftResponse := func(resp *http.Response, want nextLiftResp) {
		t.Helper()
		if status := resp.StatusCode; status != http.StatusOK {
			t.Fatalf("unexpected response code from server %d, wanted OK", status)
		}

		var got nextLiftResp
		dat, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if err := json.Unmarshal(dat, &got); err != nil {
			t.Fatalf("failed to decode next lift response: %v", err)
		}
		checkLift(got, want)
	}

	checkNextLift := func(want nextLiftResp) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/api/nextLift", nil)
		w := httptest.NewRecorder()
		srv.serveNextLift(w, r)

		resp := w.Result()
		if status := resp.StatusCode; status != http.StatusOK {
			t.Fatalf("unexpected response code from server %d, wanted OK", status)
		}

		checkAndParseNextLiftResponse(w.Result(), want)
	}

	// Confirm that our first lift looks like we're expecting.
	checkNextLift(nextLiftResp{
		DayNumber:         0,
		WeekNumber:        0,
		IterationNumber:   0,
		DayName:           "Press Day",
		WeekName:          "Week 1",
		Workout:           env.workout(t, fto.OverheadPress, 0),
		NextMovementIndex: 0,
		NextSetIndex:      0,
	})

	rec := func(ex fto.Exercise, st fto.SetType, weight string, set, reps, day, week, iteration int, toFailure bool) recordReq {
		return recordReq{
			Exercise:  ex,
			SetType:   st,
			Weight:    weight,
			Set:       set,
			Reps:      reps,
			Day:       day,
			Week:      week,
			Iteration: iteration,
			ToFailure: toFailure,
		}
	}

	var (
		pressWeekOneWorkout = env.workout(t, fto.OverheadPress, 0)
		squatWeekOneWorkout = env.workout(t, fto.Squat, 0)
		benchWeekOneWorkout = env.workout(t, fto.BenchPress, 0)
		// deadliftWeekOneWorkout = env.workout(t, fto.Deadlift, 0)

		// pressWeekTwoWorkout    = env.workout(t, fto.OverheadPress, 1)
		// squatWeekTwoWorkout    = env.workout(t, fto.Squat, 1)
		// benchWeekTwoWorkout    = env.workout(t, fto.BenchPress, 1)
		// deadliftWeekTwoWorkout = env.workout(t, fto.Deadlift, 1)

		// pressWeekThreeWorkout    = env.workout(t, fto.OverheadPress, 1)
		// squatWeekThreeWorkout    = env.workout(t, fto.Squat, 1)
		// benchWeekThreeWorkout    = env.workout(t, fto.BenchPress, 1)
		// deadliftWeekThreeWorkout = env.workout(t, fto.Deadlift, 1)
	)

	// Now, record lifts and make sure the next one looks right.
	tests := []struct {
		toRecord     recordReq
		wantNextLift nextLiftResp
	}{
		// Warmup
		{
			toRecord: rec(fto.OverheadPress, fto.Warmup, "50", 0, 5, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 0,
				NextSetIndex:      1,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Warmup, "65", 1, 5, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 0,
				NextSetIndex:      2,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Warmup, "77.5", 2, 3, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 1,
				NextSetIndex:      0,
			},
		},
		// Main
		{
			toRecord: rec(fto.OverheadPress, fto.Main, "82.5", 0, 5, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 1,
				NextSetIndex:      1,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Main, "95", 1, 5, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 1,
				NextSetIndex:      2,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Main, "107.5", 2, 7, 0, 0, 0, true),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      0,
			},
		},
		// Assistance
		{
			toRecord: rec(fto.OverheadPress, fto.Assistance, "77.5", 0, 10, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      1,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Assistance, "77.5", 1, 10, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      2,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Assistance, "65", 2, 10, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      3,
			},
		},
		{
			toRecord: rec(fto.OverheadPress, fto.Assistance, "65", 3, 10, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         0,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Press Day",
				WeekName:          "Week 1",
				Workout:           pressWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      4,
			},
		},
		// Now onto squats
		{
			toRecord: rec(fto.OverheadPress, fto.Assistance, "50", 4, 10, 0, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 0,
				NextSetIndex:      0,
			},
		},
		// Warmup
		{
			toRecord: rec(fto.Squat, fto.Warmup, "92.5", 0, 5, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 0,
				NextSetIndex:      1,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Warmup, "115", 1, 5, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 0,
				NextSetIndex:      2,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Warmup, "137.5", 2, 3, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 1,
				NextSetIndex:      0,
			},
		},
		// Main
		{
			toRecord: rec(fto.Squat, fto.Main, "150", 0, 5, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 1,
				NextSetIndex:      1,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Main, "172.5", 1, 5, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 1,
				NextSetIndex:      2,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Main, "195", 2, 7, 1, 0, 0, true),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      0,
			},
		},
		// Assistance
		{
			toRecord: rec(fto.Squat, fto.Assistance, "92.5", 0, 10, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      1,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Assistance, "92.5", 1, 10, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      2,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Assistance, "92.5", 2, 10, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      3,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Assistance, "70", 3, 10, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         1,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Squat Day",
				WeekName:          "Week 1",
				Workout:           squatWeekOneWorkout,
				NextMovementIndex: 2,
				NextSetIndex:      4,
			},
		},
		{
			toRecord: rec(fto.Squat, fto.Assistance, "70", 4, 10, 1, 0, 0, false),
			wantNextLift: nextLiftResp{
				DayNumber:         2,
				WeekNumber:        0,
				IterationNumber:   0,
				DayName:           "Bench Day",
				WeekName:          "Week 1",
				Workout:           benchWeekOneWorkout,
				NextMovementIndex: 0,
				NextSetIndex:      0,
			},
		},
		// TODO: Continue this extremely tedious process of checking the next lift, and probably also correlate the recorded weight with the expected weight for the routine.
	}

	curMvmt, curSet, dayName := 0, 0, tests[0].wantNextLift.DayName
	for _, test := range tests {
		t.Run(testName(test.toRecord), func(t *testing.T) {

			req, err := json.Marshal(test.toRecord)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}
			r := httptest.NewRequest(http.MethodPost, "/api/recordLift", bytes.NewReader(req))
			w := httptest.NewRecorder()
			srv.serveRecordLift(w, r)

			resp := w.Result()
			if status := resp.StatusCode; status != http.StatusOK {
				t.Fatalf("unexpected response code from server %d, wanted OK", status)
			}

			var got recordLiftResp
			dat, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			if err := json.Unmarshal(dat, &got); err != nil {
				t.Fatalf("failed to decode next lift response: %v", err)
			}
			// Before we check the lift, make our workout correct. When we finish a day's
			// workout, we get the next one, and we'd be setting the ID on a totally
			// unrelated lift, so only set if we're still on the same day.
			if dayName == test.wantNextLift.DayName {
				test.wantNextLift.Workout[curMvmt].Sets[curSet].AssociatedLiftID = got.LiftID
			}

			checkLift(*got.NextLift, test.wantNextLift)

			curMvmt = test.wantNextLift.NextMovementIndex
			curSet = test.wantNextLift.NextSetIndex
			dayName = test.wantNextLift.DayName
		})
	}
}

func testName(in recordReq) string {
	return fmt.Sprintf("[%s] %s %d %d %d", in.SetType, in.Exercise, in.Set, in.Day, in.Week)
}

func (e *testEnv) workout(t *testing.T, ex fto.Exercise, weekNum int) []*fto.Movement {
	var r1, r2, r3 int
	var tm1, tm2, tm3 int
	switch weekNum {
	case 0:
		r1, r2, r3 = 5, 5, 5
		tm1, tm2, tm3 = 65, 75, 85
	case 1:
		r1, r2, r3 = 3, 3, 3
		tm1, tm2, tm3 = 70, 80, 90
	case 2:
		r1, r2, r3 = 5, 3, 1
		tm1, tm2, tm3 = 75, 85, 95
	default:
		t.Fatalf("unsupported week index %d", weekNum)
	}
	var atm1, atm2, atm3, atm4, atm5 int
	if ex == fto.Squat {
		atm1, atm2, atm3, atm4, atm5 = 40, 40, 40, 30, 30
	} else {
		atm1, atm2, atm3, atm4, atm5 = 60, 60, 50, 50, 40
	}

	tm := e.trainingMax(t, ex)
	sd := e.smallestDenom(t)

	set := func(repTarget, trainingMax int, failure bool) *fto.Set {
		var comparables *fto.ComparableLifts
		if failure {
			comparables = &fto.ComparableLifts{}
		}
		return &fto.Set{
			RepTarget:             repTarget,
			TrainingMaxPercentage: trainingMax,
			ToFailure:             failure,
			WeightTarget:          roundWeight(tm, trainingMax, sd),
			FailureComparables:    comparables,
		}
	}

	return []*fto.Movement{
		{
			Exercise: ex,
			SetType:  fto.Warmup,
			Sets: []*fto.Set{
				set(5, 40, false),
				set(5, 50, false),
				set(3, 60, false),
			},
		},
		{
			Exercise: ex,
			SetType:  fto.Main,
			Sets: []*fto.Set{
				set(r1, tm1, false),
				set(r2, tm2, false),
				set(r3, tm3, true),
			},
		},
		{
			Exercise: ex,
			SetType:  fto.Assistance,
			Sets: []*fto.Set{
				set(10, atm1, false),
				set(10, atm2, false),
				set(10, atm3, false),
				set(10, atm4, false),
				set(10, atm5, false),
			},
		},
	}
}

func TestParsePounds(t *testing.T) {
	wt := func(in int) fto.Weight {
		return fto.Weight{
			Value: in,
			Unit:  fto.DeciPounds,
		}
	}

	tests := []struct {
		in      string
		want    fto.Weight
		wantErr bool
	}{
		// Good cases.
		{
			in:   "5",
			want: wt(50),
		},
		{
			in:   "150",
			want: wt(1500),
		},
		{
			in:   "150.",
			want: wt(1500),
		},
		{
			in:   "150.0",
			want: wt(1500),
		},
		{
			in:   "150.5",
			want: wt(1505),
		},
		{
			in:   ".5",
			want: wt(5),
		},
		{
			in:   "0.5",
			want: wt(5),
		},
		// Error cases
		{
			in:      "abc",
			wantErr: true,
		},
		{
			in:      "abc.5",
			wantErr: true,
		},
		{
			in:      "-1",
			wantErr: true,
		},
		{
			in:      "-100",
			wantErr: true,
		},
		{
			in:      "-100.0",
			wantErr: true,
		},
		{
			in:      "100.-9",
			wantErr: true,
		},
		{
			in:      "100.abc",
			wantErr: true,
		},
		{
			in:      "100.12",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, err := parsePounds(test.in)
			if err != nil {
				if test.wantErr {
					// Expected.
					return
				}
				t.Fatalf("parsePounds(%q): %v", test.in, err)
			}

			if test.wantErr {
				t.Fatal("parsePounds wanted an error, but none occurred")
			}

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected fto.Weight returned (-want +got)\n%s", diff)
			}
		})
	}
}

func TestRoundWeight(t *testing.T) {
	wt := func(in int) fto.Weight {
		return fto.Weight{
			Value: in,
			Unit:  fto.DeciPounds,
		}
	}

	tests := []struct {
		trainingMax   fto.Weight
		percent       int
		smallestDenom fto.Weight
		want          fto.Weight
	}{
		{
			trainingMax:   wt(1050),
			percent:       85,
			smallestDenom: wt(25),
			want:          wt(900),
		},
		{
			trainingMax:   wt(1050),
			percent:       85,
			smallestDenom: wt(50),
			want:          wt(900),
		},
		{
			trainingMax:   wt(2100),
			percent:       85,
			smallestDenom: wt(25),
			want:          wt(1775),
		},
		{
			trainingMax:   wt(2100),
			percent:       85,
			smallestDenom: wt(50),
			want:          wt(1800),
		},
		{
			trainingMax:   wt(1700),
			percent:       85,
			smallestDenom: wt(25),
			want:          wt(1450),
		},
		{
			trainingMax:   wt(1700),
			percent:       85,
			smallestDenom: wt(100),
			want:          wt(1400),
		},
		{
			trainingMax:   wt(2650),
			percent:       85,
			smallestDenom: wt(25),
			want:          wt(2250),
		},

		{
			trainingMax:   wt(1050),
			percent:       90,
			smallestDenom: wt(25),
			want:          wt(950),
		},

		{
			trainingMax:   wt(2100),
			percent:       90,
			smallestDenom: wt(25),
			want:          wt(1900),
		},

		{
			trainingMax:   wt(1700),
			percent:       90,
			smallestDenom: wt(25),
			want:          wt(1525),
		},

		{
			trainingMax:   wt(2650),
			percent:       90,
			smallestDenom: wt(25),
			want:          wt(2375),
		},

		{
			trainingMax:   wt(1050),
			percent:       95,
			smallestDenom: wt(25),
			want:          wt(1000),
		},
	}

	for _, test := range tests {
		got := roundWeight(test.trainingMax, test.percent, test.smallestDenom)
		if got != test.want {
			t.Errorf("roundWeight(%q, %d, %q) = %q, want %q", test.trainingMax, test.percent, test.smallestDenom, got, test.want)
		}
	}
}

type testEnv struct {
	db *testdb.DB
}

func (e *testEnv) trainingMax(t *testing.T, ex fto.Exercise) fto.Weight {
	tms, err := e.db.TrainingMaxes()
	if err != nil {
		t.Fatalf("failed to load training maxes: %v", err)
	}
	for _, tm := range tms {
		if tm.Exercise == ex {
			return tm.Max
		}
	}
	t.Fatalf("no training max was found for exercise %q", ex)
	return fto.Weight{}
}

func (e *testEnv) smallestDenom(t *testing.T) fto.Weight {
	w, err := e.db.SmallestDenom()
	if err != nil {
		t.Fatalf("failed to load smallest denom: %v", err)
	}
	return w
}

func setup(t *testing.T) (*Server, *testEnv) {
	env := &testEnv{db: testdb.New()}

	return New(loadRoutine(t), env.db), env
}

func loadRoutine(t *testing.T) *fto.Routine {
	root := rootDir(t)

	f, err := os.Open(filepath.Join(root, "..", "routine.json"))
	if err != nil {
		t.Fatalf("failed to open routine file: %v", err)
	}
	defer f.Close()

	var routine *fto.Routine
	if err := json.NewDecoder(f).Decode(&routine); err != nil {
		t.Fatalf("failed to parse routine file as JSON: %v", err)
	}
	return routine
}

func rootDir(t *testing.T) string {
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to load caller info")
	}
	return filepath.Dir(b)
}
