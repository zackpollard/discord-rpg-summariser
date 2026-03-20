package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionEncryptDecrypt(t *testing.T) {
	sm, err := NewSessionManager("test-secret-key", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	original := &SessionData{
		UserID:    "123456789",
		Username:  "testuser",
		Avatar:    "abc123",
		ExpiresAt: time.Now().Add(sessionMaxAge).Unix(),
	}

	rec := httptest.NewRecorder()
	if err := sm.Encode(rec, original); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Extract the cookie and put it on a request.
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != cookieName {
		t.Fatalf("expected cookie name %q, got %q", cookieName, cookies[0].Name)
	}
	if !cookies[0].HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])

	decoded, err := sm.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded == nil {
		t.Fatal("expected non-nil session data")
	}
	if decoded.UserID != original.UserID {
		t.Fatalf("expected UserID %q, got %q", original.UserID, decoded.UserID)
	}
	if decoded.Username != original.Username {
		t.Fatalf("expected Username %q, got %q", original.Username, decoded.Username)
	}
	if decoded.Avatar != original.Avatar {
		t.Fatalf("expected Avatar %q, got %q", original.Avatar, decoded.Avatar)
	}
}

func TestSessionDecrypt_NoCookie(t *testing.T) {
	sm, err := NewSessionManager("test-secret", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	data, err := sm.Decode(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatal("expected nil data for missing cookie")
	}
}

func TestSessionDecrypt_Expired(t *testing.T) {
	sm, err := NewSessionManager("test-secret", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	expired := &SessionData{
		UserID:    "123",
		Username:  "expired-user",
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}

	rec := httptest.NewRecorder()
	if err := sm.Encode(rec, expired); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])

	data, err := sm.Decode(req)
	if err == nil {
		t.Fatal("expected error for expired session")
	}
	if data != nil {
		t.Fatal("expected nil data for expired session")
	}
}

func TestSessionDecrypt_TamperedCookie(t *testing.T) {
	sm, err := NewSessionManager("test-secret", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: "dGhpcyBpcyBub3QgYSB2YWxpZCBzZXNzaW9u",
	})

	data, err := sm.Decode(req)
	if err == nil {
		t.Fatal("expected error for tampered cookie")
	}
	if data != nil {
		t.Fatal("expected nil data for tampered cookie")
	}
}

func TestSessionDecrypt_WrongKey(t *testing.T) {
	sm1, _ := NewSessionManager("secret-one", false)
	sm2, _ := NewSessionManager("secret-two", false)

	original := &SessionData{
		UserID:    "123",
		Username:  "test",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}

	rec := httptest.NewRecorder()
	sm1.Encode(rec, original)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])

	data, err := sm2.Decode(req)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
	if data != nil {
		t.Fatal("expected nil data when decrypting with wrong key")
	}
}

func TestSessionClear(t *testing.T) {
	sm, _ := NewSessionManager("test-secret", false)
	rec := httptest.NewRecorder()
	sm.Clear(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", cookies[0].MaxAge)
	}
}

func TestSessionManager_RandomKey(t *testing.T) {
	sm, err := NewSessionManager("", false)
	if err != nil {
		t.Fatalf("NewSessionManager with empty secret: %v", err)
	}

	data := &SessionData{
		UserID:    "456",
		Username:  "randomkey-user",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}

	rec := httptest.NewRecorder()
	if err := sm.Encode(rec, data); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(rec.Result().Cookies()[0])

	decoded, err := sm.Decode(req)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.UserID != "456" {
		t.Fatalf("expected UserID 456, got %q", decoded.UserID)
	}
}
