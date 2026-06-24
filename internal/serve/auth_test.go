package serve

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newAuthServer wires a Server with a configured token over a fresh .tsk.md.
func newAuthServer(t *testing.T, token string) *Server {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, ".tsk.md")
	if err := store.AtomicWriteFile(file, []byte("# test\n\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	return New(Options{
		Addr:  "127.0.0.1:0",
		File:  file,
		Token: token,
		Now:   func() time.Time { return now },
		TZ:    time.UTC,
	})
}

func TestAuthDisabledByDefaultAllowsAPI(t *testing.T) {
	s, _ := newTestServer(t) // no token
	rec, _ := do(t, s.Handler(), http.MethodGet, "/api/tasks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (auth should be off by default)", rec.Code)
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	s := newAuthServer(t, "s3cret")
	rec, _ := do(t, s.Handler(), http.MethodGet, "/api/tasks", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if ch := rec.Header().Get("WWW-Authenticate"); ch == "" {
		t.Fatalf("missing WWW-Authenticate challenge header")
	}
}

func TestAuthAcceptsBearerToken(t *testing.T) {
	s := newAuthServer(t, "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with valid bearer", rec.Code)
	}
}

func TestAuthRejectsWrongBearerToken(t *testing.T) {
	s := newAuthServer(t, "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with wrong token", rec.Code)
	}
}

func TestAuthAcceptsSessionCookie(t *testing.T) {
	s := newAuthServer(t, "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "s3cret"})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with valid cookie", rec.Code)
	}
}

func TestAuthQueryParamRejectedOnAPI(t *testing.T) {
	// ?token= must NOT authenticate /api/* (it would leak in logs/Referer).
	s := newAuthServer(t, "s3cret")
	rec, _ := do(t, s.Handler(), http.MethodGet, "/api/tasks?token=s3cret", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (query token not allowed on API)", rec.Code)
	}
}

func TestBootstrapSetsCookieAndRedirects(t *testing.T) {
	s := newAuthServer(t, "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/?token=s3cret", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 redirect", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("redirect Location = %q, want / (token stripped)", loc)
	}
	var found *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			found = c
		}
	}
	if found == nil {
		t.Fatal("bootstrap did not set the session cookie")
	}
	if found.Value != "s3cret" {
		t.Fatalf("cookie value = %q, want s3cret", found.Value)
	}
	if !found.HttpOnly {
		t.Fatal("session cookie should be HttpOnly")
	}
}

func TestBootstrapWrongTokenStillStripsButNoCookie(t *testing.T) {
	s := newAuthServer(t, "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/?token=wrong", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (still strip the token)", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			t.Fatal("a wrong token must not set a session cookie")
		}
	}
}

func TestPlaceholderShellServedWithoutTokenWhenAuthOn(t *testing.T) {
	// The SPA shell itself is not secret — only /api/* data is gated. A plain
	// GET / (no token) should still serve the placeholder/SPA, not 401.
	s := newAuthServer(t, "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (shell is public)", rec.Code)
	}
}

func TestStripQueryParam(t *testing.T) {
	cases := []struct {
		raw  string
		key  string
		want string
	}{
		{"/?token=abc", "token", "/"},
		{"/path?token=abc", "token", "/path"},
		{"/path?token=abc&x=1", "token", "/path?x=1"},
		{"/?x=1", "token", "/?x=1"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.raw, nil)
		got := stripQueryParam(req.URL, c.key)
		if got != c.want {
			t.Errorf("stripQueryParam(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
