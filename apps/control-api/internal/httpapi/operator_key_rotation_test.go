package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/bitriver/studio-os/apps/control-api/internal/operatorcredentials"
)

func TestOfflineRotationRevokesSessionsImmediately(t *testing.T) {
	t.Parallel()
	const oldKey = "existing-valid-operator-key"
	server, auth := newSecureTestServer(t, oldKey)
	defer server.Close()

	login := requestWithKey(t, server, oldKey, server.URL)
	if login.StatusCode != http.StatusOK { t.Fatalf("initial login: %d", login.StatusCode) }
	cookie := login.Cookies()[0]
	login.Body.Close()

	authenticatedRead := func() int {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/system", nil)
		if err != nil { t.Fatal(err) }
		request.AddCookie(cookie)
		response, err := server.Client().Do(request)
		if err != nil { t.Fatal(err) }
		defer response.Body.Close()
		return response.StatusCode
	}
	if status := authenticatedRead(); status != http.StatusOK {
		t.Fatalf("original session rejected prematurely: %d", status)
	}

	newKey, err := operatorcredentials.Rotate(auth.credentialsPath)
	if err != nil { t.Fatal(err) }
	if newKey == oldKey { t.Fatal("rotation reused old key") }
	if status := authenticatedRead(); status != http.StatusUnauthorized {
		t.Fatalf("previous session remained valid after rotation: %d", status)
	}

	oldLogin := requestWithKey(t, server, oldKey, server.URL)
	oldLogin.Body.Close()
	if oldLogin.StatusCode != http.StatusUnauthorized {
		t.Fatalf("previous key accepted after rotation: %d", oldLogin.StatusCode)
	}

	newLogin := requestWithKey(t, server, newKey, server.URL)
	newLogin.Body.Close()
	if newLogin.StatusCode != http.StatusOK { t.Fatalf("rotated key rejected: %d", newLogin.StatusCode) }

	auth.mu.Lock()
	active := len(auth.sessions)
	auth.mu.Unlock()
	if active != 1 { t.Fatalf("expected only new session, got %d", active) }

	// Credential loss/corruption must deny existing sessions, not keep a
	// previously loaded key as a silent fallback.
	if err := os.WriteFile(auth.credentialsPath, []byte("{broken"), 0o600); err != nil { t.Fatal(err) }
	if status := authenticatedRead(); status != http.StatusServiceUnavailable {
		t.Fatalf("corrupted credentials did not fail closed: %d", status)
	}
	failLogin := requestWithKey(t, server, newKey, server.URL)
	failLogin.Body.Close()
	if failLogin.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("login must fail closed on missing credentials: %d", failLogin.StatusCode)
	}
	// Restore a valid digest; previously granted sessions may be revalidated
	// only when they have not expired or been revoked by an actual rotation.
	_ = json.Valid
}
