package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequireAuth_ValidSession(t *testing.T) {
	sm, _ := NewSessionManager("test-secret", false)

	session := &SessionData{
		UserID:    "999",
		Username:  "authed-user",
		Avatar:    "avatar123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}

	// Create a cookie.
	rec := httptest.NewRecorder()
	sm.Encode(rec, session)
	cookie := rec.Result().Cookies()[0]

	// Build a handler behind the middleware.
	var capturedUser *SessionData
	handler := RequireAuth(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user in context")
	}
	if capturedUser.UserID != "999" {
		t.Fatalf("expected UserID 999, got %q", capturedUser.UserID)
	}
	if capturedUser.Username != "authed-user" {
		t.Fatalf("expected Username authed-user, got %q", capturedUser.Username)
	}
}

func TestRequireAuth_NoCookie(t *testing.T) {
	sm, _ := NewSessionManager("test-secret", false)

	handler := RequireAuth(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without auth")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if errResp["error"] != "unauthorized" {
		t.Fatalf("expected error 'unauthorized', got %q", errResp["error"])
	}
}

func TestRequireAuth_ExpiredSession(t *testing.T) {
	sm, _ := NewSessionManager("test-secret", false)

	session := &SessionData{
		UserID:    "999",
		Username:  "expired-user",
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}

	rec := httptest.NewRecorder()
	sm.Encode(rec, session)
	cookie := rec.Result().Cookies()[0]

	handler := RequireAuth(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for expired session")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec2.Code)
	}
}

func TestRequireAuth_InvalidCookie(t *testing.T) {
	sm, _ := NewSessionManager("test-secret", false)

	handler := RequireAuth(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for invalid cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "garbage"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUserFromContext_Nil(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := UserFromContext(req.Context())
	if user != nil {
		t.Fatal("expected nil user from empty context")
	}
}
