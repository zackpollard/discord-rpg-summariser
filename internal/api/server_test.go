package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"discord-rpg-summariser/internal/storage"
)

func testStore(t *testing.T) *storage.Store {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	migrationsFS := os.DirFS("../../migrations")
	store, err := storage.New(context.Background(), dbURL, migrationsFS)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestHandleListSessions(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/sessions?limit=5&offset=0", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var sessions []sessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should be a valid JSON array (possibly empty).
	if sessions == nil {
		t.Fatal("expected non-nil sessions array")
	}
}

func TestHandleGetSession_NotFound(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/999999", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp["error"] != "session not found" {
		t.Fatalf("expected 'session not found', got %q", errResp["error"])
	}
}

func TestHandleGetSession_InvalidID(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/abc", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpsertCharacter(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	body := `{"user_id": "user-123", "guild_id": "test-guild", "character_name": "Gandalf"}`
	req := httptest.NewRequest(http.MethodPut, "/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp characterResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.UserID != "user-123" {
		t.Fatalf("expected user_id 'user-123', got %q", resp.UserID)
	}
	if resp.CharacterName != "Gandalf" {
		t.Fatalf("expected character_name 'Gandalf', got %q", resp.CharacterName)
	}

	// Clean up.
	_ = store.DeleteCharacterMapping(context.Background(), "user-123", "test-guild")
}

func TestHandleUpsertCharacter_MissingFields(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	body := `{"user_id": "", "character_name": ""}`
	req := httptest.NewRequest(http.MethodPut, "/api/characters", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleStatus(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// With no active session, recording should be false.
	if resp.Recording {
		t.Fatal("expected recording to be false")
	}
	if resp.ActiveSession != nil {
		t.Fatal("expected active_session to be nil")
	}
}

func TestHandleGetTranscript(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	// Request transcript for a non-existent session (should return empty array, not error).
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/999999/transcript", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var segments []transcriptSegmentResponse
	if err := json.NewDecoder(rec.Body).Decode(&segments); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if segments == nil {
		t.Fatal("expected non-nil segments array")
	}
}

func TestHandleDeleteCharacter(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	// Delete a mapping that may or may not exist; should succeed either way.
	req := httptest.NewRequest(http.MethodDelete, "/api/characters/user-nonexistent?guild_id=test-guild", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleListCharacters(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var chars []characterResponse
	if err := json.NewDecoder(rec.Body).Decode(&chars); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if chars == nil {
		t.Fatal("expected non-nil characters array")
	}
}

func TestCORSMiddleware(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	// Test preflight request.
	req := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for OPTIONS, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected CORS origin 'http://localhost:5173', got %q", got)
	}
}
