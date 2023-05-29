package server

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lexacali/fivethreeone/fto"
	"github.com/lexacali/fivethreeone/testing/testdb"
)

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

func setup() (*Server, *testEnv) {
	env := &testEnv{db: testdb.New()}

	return New(&fto.Routine{}, env.db), env
}
