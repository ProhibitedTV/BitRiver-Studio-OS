package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newSecureTestServer(t *testing.T, key string) (*httptest.Server, *OperatorAuth) {
	t.Helper()
	dir := t.TempDir()
	digest := sha256.Sum256([]byte(key))
	credentials := []byte(fmt.Sprintf(`{"controlKeySha256":%q}`, hex.EncodeToString(digest[:])))
	path := filepath.Join(dir, "admin-credentials.json")
	if err := os.WriteFile(path, credentials, 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := NewOperatorAuth(path)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewSecureHandler(
		slog.New(slog.NewTextHandler(io.Discard, nil)), newTestController(t), "", "", auth,
	))
	return server, auth
}

func requestWithKey(t *testing.T, server *httptest.Server, key string, origin string) *http.Response {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"key": key})
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestOperatorAuthGatesControlPlane(t *testing.T) {
	t.Parallel()
	const key = "550c747a5c52f7adf4db137ced24a6c5"
	server, auth := newSecureTestServer(t, key)
	defer server.Close()

	for _, target := range []string{
		"/api/v1/stream", "/api/v1/settings/active", "/api/v1/creator-assets/catalog",
		"/api/v1/show", "/api/v1/ws", "/api/v1/openapi.yaml",
	} {
		response, err := http.Get(server.URL + target)
		if err != nil { t.Fatal(err) }
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("unauthenticated GET %s: %d", target, response.StatusCode)
		}
	}
	withoutAuth, err := http.Post(server.URL+"/api/v1/stream/start", "application/json", nil)
	if err != nil { t.Fatal(err) }
	withoutAuth.Body.Close()
	if withoutAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated stream start: %d", withoutAuth.StatusCode)
	}

	health, err := http.Get(server.URL+"/healthz")
	if err != nil { t.Fatal(err) }
	health.Body.Close()
	if health.StatusCode != http.StatusOK {
		t.Fatalf("liveness endpoint must remain available: %d", health.StatusCode)
	}

	bad := requestWithKey(t, server, "wrong-secret", server.URL)
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected wrong key rejection, got %d", bad.StatusCode)
	}

	foreign := requestWithKey(t, server, key, "https://untrusted.example")
	foreign.Body.Close()
	if foreign.StatusCode != http.StatusForbidden {
		t.Fatalf("expected foreign origin rejection, got %d", foreign.StatusCode)
	}

	login := requestWithKey(t, server, key, server.URL)
	if login.StatusCode != http.StatusOK {
		t.Fatalf("valid login: %d", login.StatusCode)
	}
	var session struct {
		Enabled bool `json:"enabled"`
		Authenticated bool `json:"authenticated"`
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.NewDecoder(login.Body).Decode(&session); err != nil { t.Fatal(err) }
	login.Body.Close()
	if !session.Enabled || !session.Authenticated || len(session.CSRFToken) < 32 {
		t.Fatalf("invalid session response: %#v", session)
	}
	cookies := login.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe operator cookie: %#v", cookies)
	}

	authenticated := func(method, path, csrf, origin string) int {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil { t.Fatal(err) }
		req.AddCookie(cookies[0])
		if csrf != "" { req.Header.Set("X-BitRiver-CSRF", csrf) }
		if origin != "" { req.Header.Set("Origin", origin) }
		result, err := server.Client().Do(req)
		if err != nil { t.Fatal(err) }
		result.Body.Close()
		return result.StatusCode
	}

	if result := authenticated(http.MethodGet, "/api/v1/stream", "", ""); result != http.StatusOK {
		t.Fatalf("authorized read: %d", result)
	}
	if result := authenticated(http.MethodPost, "/api/v1/stream/start", "", ""); result != http.StatusForbidden {
		t.Fatalf("missing CSRF: %d", result)
	}
	if result := authenticated(http.MethodPost, "/api/v1/stream/start", session.CSRFToken, "https://untrusted.example"); result != http.StatusForbidden {
		t.Fatalf("foreign origin on mutation: %d", result)
	}
	if result := authenticated(http.MethodPost, "/api/v1/stream/start", session.CSRFToken, server.URL); result != http.StatusOK {
		t.Fatalf("authorized action: %d", result)
	}
	if result := authenticated(http.MethodPost, "/api/v1/auth/logout", session.CSRFToken, server.URL); result != http.StatusOK {
		t.Fatalf("logout: %d", result)
	}
	if result := authenticated(http.MethodGet, "/api/v1/stream", "", ""); result != http.StatusUnauthorized {
		t.Fatalf("session should be revoked after logout: %d", result)
	}

	if len(auth.sessions) != 0 {
		t.Fatalf("logout did not revoke operator session")
	}
}

func TestOperatorAuthFailureCases(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "invalid.json")
	if _, err := NewOperatorAuth(path); err == nil { t.Fatal("missing credentials should fail closed") }
	if err := os.WriteFile(path, []byte(`{"controlKeySha256":""}`), 0o600); err != nil { t.Fatal(err) }
	if _, err := NewOperatorAuth(path); err == nil { t.Fatal("missing key digest should fail closed") }
}

func TestOperatorLoginRateLimitAndExpiredSession(t *testing.T) {
	t.Parallel()
	server, auth := newSecureTestServer(t, "e92ff6b3e26510fa960b4420f1377af1")
	defer server.Close()
	for i:=0;i<8;i++ {
		res := requestWithKey(t, server, "wrong", "")
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized { t.Fatalf("attempt %d: %d",i,res.StatusCode) }
	}
	res := requestWithKey(t, server, "e92ff6b3e26510fa960b4420f1377af1", "")
	res.Body.Close()
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rate limit did not block after failures: %d", res.StatusCode)
	}
	auth.mu.Lock()
	auth.failures = nil
	auth.mu.Unlock()

	success := requestWithKey(t, server, "e92ff6b3e26510fa960b4420f1377af1", "")
	var payload struct { CSRFToken string `json:"csrfToken"` }
	if err := json.NewDecoder(success.Body).Decode(&payload); err != nil { t.Fatal(err) }
	success.Body.Close()
	cookie := success.Cookies()[0]
	auth.mu.Lock()
	item := auth.sessions[cookie.Value]
	item.Expires = time.Now().Add(-time.Minute)
	auth.sessions[cookie.Value] = item
	auth.mu.Unlock()
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/system", nil)
	req.AddCookie(cookie)
	response, err := server.Client().Do(req)
	if err != nil { t.Fatal(err) }
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired session unexpectedly accepted: %d", response.StatusCode)
	}
}

func TestWebsocketRejectsCrossOriginEvenWithValidSession(t *testing.T) {
	t.Parallel()
	server, _ := newSecureTestServer(t, "7af49f6a8fc2f49c6aa9e7b01ab995cc")
	defer server.Close()
	login := requestWithKey(t, server, "7af49f6a8fc2f49c6aa9e7b01ab995cc", "")
	login.Body.Close()
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/ws", nil)
	req.AddCookie(login.Cookies()[0])
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", strings.Repeat("a", 24))
	res, err := server.Client().Do(req)
	if err != nil { t.Fatal(err) }
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden { t.Fatalf("WebSocket cross-origin: %d", res.StatusCode) }
}
