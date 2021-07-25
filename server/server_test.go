package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lexacali/fivethreeone/testing/testcookie"
)

const (
	loginTestReq = `{
	"password": "test"
}`
)

func TestLogin(t *testing.T) {
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
		"name": "Testy McTesterson",
	}

	if diff := cmp.Diff(wantCookieVal, cookieVal); diff != "" {
		t.Errorf("unexpected cookie val (-want +got)\n%s", diff)
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

type testEnv struct {
	users map[string]string
	sc    *testcookie.SecureCookie
}

func setup() (*Server, *testEnv) {
	env := &testEnv{
		users: map[string]string{
			"test": "Testy McTesterson",
		},
		sc: testcookie.New(),
	}

	return New(env.users, env.sc, ""), env
}
