package serve

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// cookieName is the session cookie set by the one-time ?token= bootstrap so a
// browser stays authenticated without the SPA ever handling the token in JS.
const cookieName = "tsk_token"

// authEnabled reports whether token auth is configured for this server.
func (s *Server) authEnabled() bool {
	return s.opts.Token != ""
}

// tokenMatches does a constant-time comparison of a presented token against the
// configured one. Returns false when auth is disabled (no token to match) — the
// caller only consults this when auth is enabled.
func (s *Server) tokenMatches(presented string) bool {
	if s.opts.Token == "" || presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(s.opts.Token)) == 1
}

// presentedToken extracts a token from the request, in priority order:
//
//  1. Authorization: Bearer <token>   (programmatic clients: curl, scripts)
//  2. tsk_token cookie                (browser session after bootstrap)
//
// The ?token= query param is intentionally NOT accepted on /api/* routes — it
// would leak into logs and Referer headers. It is only honored by the root
// bootstrap, which immediately swaps it for an HttpOnly cookie.
func presentedToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if tok, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(tok)
		}
	}
	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}
	return ""
}

// requireAuth wraps an API handler with token enforcement. When no token is
// configured (the loopback default) it is a transparent pass-through, so the
// local-first experience is unchanged. When a token IS set, every /api/*
// request must present it via bearer header or session cookie or it gets a
// 401 with a WWW-Authenticate challenge.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		if s.tokenMatches(presentedToken(r)) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="tsk"`)
		writeError(w, http.StatusUnauthorized, "missing or invalid token")
	})
}

// tokenBootstrap wraps the root ("/") handler. When auth is enabled and the
// request carries a valid ?token=, it sets an HttpOnly session cookie and
// redirects to a clean URL (stripping the token from the address bar / history)
// so the same-origin SPA's fetch calls authenticate via the cookie. Requests
// without a bootstrap token fall straight through — the SPA shell itself is not
// secret; only the /api/* data is gated.
func (s *Server) tokenBootstrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authEnabled() {
			if tok := r.URL.Query().Get("token"); tok != "" {
				if s.tokenMatches(tok) {
					http.SetCookie(w, &http.Cookie{
						Name:     cookieName,
						Value:    tok,
						Path:     "/",
						HttpOnly: true,
						SameSite: http.SameSiteStrictMode,
						Expires:  time.Now().Add(30 * 24 * time.Hour),
					})
				}
				// Redirect to the same path minus the token query, valid or not,
				// so the secret never lingers in the URL bar or browser history.
				clean := stripQueryParam(r.URL, "token")
				http.Redirect(w, r, clean, http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// stripQueryParam returns the URL's path plus its query with one parameter
// removed. Returns just the path when no query remains.
func stripQueryParam(u *url.URL, key string) string {
	q := u.Query()
	q.Del(key)
	if len(q) == 0 {
		if u.Path == "" {
			return "/"
		}
		return u.Path
	}
	return u.Path + "?" + q.Encode()
}
