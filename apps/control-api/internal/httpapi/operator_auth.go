package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const operatorCookie = "bitriver_operator"
const sessionLifetime = 8 * time.Hour

type operatorSession struct {
	CSRF    string
	Expires time.Time
}

// OperatorAuth implements a pre-release, local-only operator gate. Keys are
// generated from cryptographic randomness during firstboot (not user passwords).
// The strong key is compared by digest, and raw credentials never enter logs.
type OperatorAuth struct {
	keyDigest [32]byte
	mu        sync.Mutex
	sessions  map[string]operatorSession
	failures  []time.Time
}

func NewOperatorAuth(credentialsPath string) (*OperatorAuth, error) {
	content, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, err
	}
	var credential struct {
		ControlKeySHA256 string `json:"controlKeySha256"`
	}
	if err := json.Unmarshal(content, &credential); err != nil {
		return nil, err
	}
	digest, err := hex.DecodeString(credential.ControlKeySHA256)
	if err != nil || len(digest) != sha256.Size {
		return nil, errors.New("operator credentials missing a valid controlKeySha256; rerun firstboot")
	}
	auth := &OperatorAuth{sessions: make(map[string]operatorSession)}
	copy(auth.keyDigest[:], digest)
	return auth, nil
}

func randomKey() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true // non-browser clients still require session + CSRF
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return false
	}
	if !strings.EqualFold(parsed.Host, request.Host) || parsed.User != nil {
		return false
	}
	if request.TLS != nil {
		return parsed.Scheme == "https"
	}
	return parsed.Scheme == "http"
}

func (auth *OperatorAuth) session(request *http.Request) (operatorSession, string, bool) {
	cookie, err := request.Cookie(operatorCookie)
	if err != nil || cookie.Value == "" {
		return operatorSession{}, "", false
	}
	auth.mu.Lock()
	defer auth.mu.Unlock()
	item, ok := auth.sessions[cookie.Value]
	if !ok || time.Now().After(item.Expires) {
		delete(auth.sessions, cookie.Value)
		return operatorSession{}, "", false
	}
	return item, cookie.Value, true
}

func (auth *OperatorAuth) gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		if !strings.HasPrefix(path, "/api/") ||
			path == "/api/v1/auth/login" || path == "/api/v1/auth/session" {
			next.ServeHTTP(writer, request)
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
		session, _, ok := auth.session(request)
		if !ok {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "operator login required"})
			return
		}
		if !sameOrigin(request) {
			writeJSON(writer, http.StatusForbidden, map[string]string{"error": "request origin rejected"})
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead &&
			subtle.ConstantTimeCompare([]byte(request.Header.Get("X-BitRiver-CSRF")), []byte(session.CSRF)) != 1 {
			writeJSON(writer, http.StatusForbidden, map[string]string{"error": "CSRF token missing or invalid"})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (auth *OperatorAuth) handleLogin(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "request origin rejected"})
		return
	}
	var input struct {
		Key string `json:"key"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Key == "" || len(input.Key) > 256 {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid operator key"})
		return
	}
	digest := sha256.Sum256([]byte(input.Key))
	now := time.Now()

	auth.mu.Lock()
	recent := auth.failures[:0]
	for _, attempt := range auth.failures {
		if now.Sub(attempt) < time.Minute {
			recent = append(recent, attempt)
		}
	}
	auth.failures = recent
	if len(auth.failures) >= 8 {
		auth.mu.Unlock()
		writer.Header().Set("Retry-After", "60")
		writeJSON(writer, http.StatusTooManyRequests, map[string]string{"error": "too many sign-in attempts"})
		return
	}
	if subtle.ConstantTimeCompare(digest[:], auth.keyDigest[:]) != 1 {
		auth.failures = append(auth.failures, now)
		auth.mu.Unlock()
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid operator key"})
		return
	}
	auth.failures = nil
	auth.mu.Unlock()

	token, err := randomKey()
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "cannot create session"})
		return
	}
	csrf, err := randomKey()
	if err != nil {
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "cannot create session"})
		return
	}
	auth.mu.Lock()
	for id, item := range auth.sessions {
		if now.After(item.Expires) {
			delete(auth.sessions, id)
		}
	}
	if len(auth.sessions) >= 128 {
		auth.mu.Unlock()
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "session limit reached"})
		return
	}
	auth.sessions[token] = operatorSession{CSRF: csrf, Expires: now.Add(sessionLifetime)}
	auth.mu.Unlock()

	auth.writeCookie(writer, request, token, int(sessionLifetime.Seconds()))
	writeJSON(writer, http.StatusOK, map[string]any{"enabled": true, "authenticated": true, "csrfToken": csrf})
}

func (auth *OperatorAuth) handleSession(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	session, _, ok := auth.session(request)
	if !ok || !sameOrigin(request) {
		writeJSON(writer, http.StatusOK, map[string]bool{"enabled": true, "authenticated": false})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"enabled": true, "authenticated": true, "csrfToken": session.CSRF})
}

func (auth *OperatorAuth) handleLogout(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer)
		return
	}
	_, token, ok := auth.session(request)
	if ok {
		auth.mu.Lock()
		delete(auth.sessions, token)
		auth.mu.Unlock()
	}
	auth.writeCookie(writer, request, "", -1)
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, map[string]bool{"authenticated": false})
}

func (auth *OperatorAuth) writeCookie(writer http.ResponseWriter, request *http.Request, token string, age int) {
	http.SetCookie(writer, &http.Cookie{
		Name: operatorCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: request.TLS != nil,
		MaxAge: age,
	})
}
