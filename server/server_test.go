package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lexacali/fivethreeone/fto"
	"github.com/lexacali/fivethreeone/testing/testcookie"
	"github.com/lexacali/fivethreeone/testing/testdb"
)

const (
	loginTestReq = `{
	"password": "test"
}`
)

func TestAuth(t *testing.T) {
	srv, env := setup()

	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(loginTestReq))
	w := httptest.NewRecorder()
	srv.serveLogin(w, r)

	resp := w.Result()
	if status := resp.StatusCode; status != http.StatusOK {
		t.Fatalf("unexpected response code from server %d, wanted OK", status)
	}

	cookie := getCookie(t, "auth", resp.Cookies())

	var cookieVal map[string]string
	if err := env.sc.Decode("auth", cookie.Value, &cookieVal); err != nil {
		t.Fatalf("failed to decode auth cookie: %v", err)
	}

	wantCookieVal := map[string]string{
		"user_id": "0",
	}

	if diff := cmp.Diff(wantCookieVal, cookieVal); diff != "" {
		t.Errorf("unexpected cookie val (-want +got)\n%s", diff)
	}

	// Now, use that auth to get user information.
	r2 := httptest.NewRequest(http.MethodGet, "/api/user", nil)
	r2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	srv.serveUser(w2, r2)

	resp2 := w.Result()
	if status := resp2.StatusCode; status != http.StatusOK {
		t.Fatalf("unexpected response code from server %d, wanted OK", status)
	}

	// Check the DB and make sure we find our user.
	gotUser, err := env.db.User(0)
	if err != nil {
		t.Fatalf("failed to load user: %v", err)
	}

	wantUser := &fto.User{ID: 0, Name: "Testy McTesterson"}

	if diff := cmp.Diff(wantUser, gotUser); diff != "" {
		t.Errorf("unexpected user returned (-want +got)\n%s", diff)
	}
}

func getCookie(t *testing.T, name string, cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie with name %q was not found", name)
	return nil
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
	users map[string]*User
	sc    *testcookie.SecureCookie
	db    *testdb.DB
}

func setup() (*Server, *testEnv) {
	env := &testEnv{
		users: map[string]*User{
			"test": &User{
				Name: "Testy McTesterson",
			},
		},
		sc: testcookie.New(),
		db: testdb.New(),
	}

	return New(env.users, env.sc, env.db, ""), env
}
