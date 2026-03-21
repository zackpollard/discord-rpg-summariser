package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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

// uniqueGuild returns a guild ID unique to the current test to avoid collisions.
func uniqueGuild(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("api-test-guild-%d", time.Now().UnixNano())
}

// ---------------------------------------------------------------------------
// Existing tests (sessions, characters, status, CORS)
// ---------------------------------------------------------------------------

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
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	body := `{"user_id": "user-123", "character_name": "Gandalf"}`
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
	_ = store.DeleteCharacterMapping(context.Background(), "user-123", resp.CampaignID)
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

	// Create a real session so the transcript endpoint can look up its campaign.
	ctx := context.Background()
	campaign, err := store.GetOrCreateActiveCampaign(ctx, "test-guild")
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	sessionID, err := store.CreateSession(ctx, "test-guild", campaign.ID, "chan-1", "/tmp/audio")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/sessions/%d/transcript", sessionID), nil)
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

// ---------------------------------------------------------------------------
// Campaign handler tests
// ---------------------------------------------------------------------------

func TestHandleListCampaigns(t *testing.T) {
	store := testStore(t)
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")
	ctx := context.Background()

	store.CreateCampaign(ctx, guildID, "Camp Alpha", "")
	store.CreateCampaign(ctx, guildID, "Camp Beta", "")

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var campaigns []campaignResponse
	if err := json.NewDecoder(rec.Body).Decode(&campaigns); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(campaigns) < 2 {
		t.Fatalf("expected at least 2 campaigns, got %d", len(campaigns))
	}
}

func TestHandleCreateCampaign(t *testing.T) {
	store := testStore(t)
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	body := `{"name": "New Adventure", "description": "A brave new world"}`
	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp campaignResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "New Adventure" {
		t.Fatalf("expected name 'New Adventure', got %q", resp.Name)
	}
	if resp.Description != "A brave new world" {
		t.Fatalf("expected description 'A brave new world', got %q", resp.Description)
	}
	if resp.ID == 0 {
		t.Fatal("expected non-zero campaign id")
	}
}

func TestHandleCreateCampaign_MissingName(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	body := `{"description": "no name given"}`
	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp["error"] != "name is required" {
		t.Fatalf("expected 'name is required', got %q", errResp["error"])
	}
}

func TestHandleGetCampaign_NotFound(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns/999999", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp["error"] != "campaign not found" {
		t.Fatalf("expected 'campaign not found', got %q", errResp["error"])
	}
}

func TestHandleSetActiveCampaign(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	id, err := store.CreateCampaign(ctx, guildID, "Activate Me", "")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	url := fmt.Sprintf("/api/campaigns/%d/active", id)
	req := httptest.NewRequest(http.MethodPut, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp campaignResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != id {
		t.Fatalf("expected campaign id %d, got %d", id, resp.ID)
	}
	if !resp.IsActive {
		t.Fatal("expected campaign to be active after SetActive")
	}
}

// ---------------------------------------------------------------------------
// Quest handler tests
// ---------------------------------------------------------------------------

func TestHandleListQuests(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Quest Camp", "")
	store.UpsertQuest(ctx, campID, "Slay Goblins", "Kill 10 goblins", "active", "Mayor")

	url := fmt.Sprintf("/api/campaigns/%d/quests", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var quests []questResponse
	if err := json.NewDecoder(rec.Body).Decode(&quests); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(quests) < 1 {
		t.Fatal("expected at least 1 quest")
	}
}

func TestHandleGetQuest_NotFound(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/quests/999999", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp["error"] != "quest not found" {
		t.Fatalf("expected 'quest not found', got %q", errResp["error"])
	}
}

// ---------------------------------------------------------------------------
// Timeline handler test
// ---------------------------------------------------------------------------

func TestHandleGetTimeline(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Timeline Camp", "")

	url := fmt.Sprintf("/api/campaigns/%d/timeline", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []storage.TimelineEvent
	if err := json.NewDecoder(rec.Body).Decode(&events); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Valid JSON array (possibly empty for a new campaign).
	if events == nil {
		t.Fatal("expected non-nil timeline array")
	}
}

// ---------------------------------------------------------------------------
// Lore search handler test
// ---------------------------------------------------------------------------

func TestHandleLoreSearch(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Lore Camp", "")
	store.UpsertEntity(ctx, campID, "Moonstone Tower", "location", "An ancient wizard tower")

	url := fmt.Sprintf("/api/campaigns/%d/lore/search?q=Moonstone", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var results []storage.LoreSearchResult
	if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if results == nil {
		t.Fatal("expected non-nil results array")
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 lore search result for 'Moonstone'")
	}
}

func TestHandleLoreSearch_MissingQuery(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Lore Camp 2", "")

	url := fmt.Sprintf("/api/campaigns/%d/lore/search", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Recap handler test
// ---------------------------------------------------------------------------

func TestHandleGetRecap(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Recap Camp", "")
	_ = store.UpdateCampaignRecap(ctx, campID, "The story so far...")

	url := fmt.Sprintf("/api/campaigns/%d/recap", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp recapResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CampaignID != campID {
		t.Fatalf("expected campaign_id %d, got %d", campID, resp.CampaignID)
	}
	if resp.Recap != "The story so far..." {
		t.Fatalf("expected recap 'The story so far...', got %q", resp.Recap)
	}
}

func TestHandleGetRecap_NotFound(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns/999999/recap", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Entity handler tests
// ---------------------------------------------------------------------------

func TestHandleListEntities(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Entity Camp", "")
	store.UpsertEntity(ctx, campID, "Dark Forest", "location", "A haunted forest")
	store.UpsertEntity(ctx, campID, "Old Sage", "npc", "A wise man")

	url := fmt.Sprintf("/api/campaigns/%d/entities", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entities []entityResponse
	if err := json.NewDecoder(rec.Body).Decode(&entities); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(entities) < 2 {
		t.Fatalf("expected at least 2 entities, got %d", len(entities))
	}
}

func TestHandleListEntities_TypeFilter(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Entity Filter Camp", "")
	store.UpsertEntity(ctx, campID, "Castle", "location", "A grand castle")
	store.UpsertEntity(ctx, campID, "Knight", "npc", "A brave knight")

	url := fmt.Sprintf("/api/campaigns/%d/entities?type=npc", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entities []entityResponse
	if err := json.NewDecoder(rec.Body).Decode(&entities); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 npc entity, got %d", len(entities))
	}
	if entities[0].Name != "Knight" {
		t.Fatalf("expected 'Knight', got %q", entities[0].Name)
	}
}

func TestHandleListEntities_StatusField(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Status Camp", "")
	id, _ := store.UpsertEntity(ctx, campID, "Dead Baron", "npc", "A fallen noble")
	store.UpdateEntityStatus(ctx, id, "dead", "Poisoned at dinner")

	url := fmt.Sprintf("/api/campaigns/%d/entities", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entities []entityResponse
	if err := json.NewDecoder(rec.Body).Decode(&entities); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(entities) < 1 {
		t.Fatal("expected at least 1 entity")
	}
	found := false
	for _, e := range entities {
		if e.Name == "Dead Baron" {
			found = true
			if e.Status != "dead" {
				t.Fatalf("expected status 'dead', got %q", e.Status)
			}
			if e.CauseOfDeath != "Poisoned at dinner" {
				t.Fatalf("expected cause_of_death 'Poisoned at dinner', got %q", e.CauseOfDeath)
			}
		}
	}
	if !found {
		t.Fatal("expected to find 'Dead Baron' in response")
	}
}

func TestHandleListEntities_StatusFilter(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Status Filter Camp", "")
	id1, _ := store.UpsertEntity(ctx, campID, "Alive Hero", "npc", "A living hero")
	store.UpdateEntityStatus(ctx, id1, "alive", "")
	id2, _ := store.UpsertEntity(ctx, campID, "Dead Villain", "npc", "A slain villain")
	store.UpdateEntityStatus(ctx, id2, "dead", "Defeated in battle")

	url := fmt.Sprintf("/api/campaigns/%d/entities?status=dead", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entities []entityResponse
	if err := json.NewDecoder(rec.Body).Decode(&entities); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 dead entity, got %d", len(entities))
	}
	if entities[0].Name != "Dead Villain" {
		t.Fatalf("expected 'Dead Villain', got %q", entities[0].Name)
	}
}

func TestHandleLocationHierarchy(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Hierarchy Camp", "")
	regionID, _ := store.UpsertEntity(ctx, campID, "Barovia", "place", "A dark region")
	villageID, _ := store.UpsertEntity(ctx, campID, "Village of Barovia", "place", "A small village")
	store.UpsertEntity(ctx, campID, "Blood on the Vine Tavern", "place", "A tavern")

	store.SetEntityParent(ctx, villageID, regionID)
	tavernEntity, _ := store.GetEntityByName(ctx, campID, "Blood on the Vine Tavern", "place")
	store.SetEntityParent(ctx, tavernEntity.ID, villageID)

	// Also create an NPC that should NOT appear.
	store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire lord")

	url := fmt.Sprintf("/api/campaigns/%d/location-hierarchy", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var tree []struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Children []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Children []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"children"`
		} `json:"children"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&tree); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have exactly one root: Barovia.
	if len(tree) != 1 {
		t.Fatalf("expected 1 root location, got %d", len(tree))
	}
	if tree[0].Name != "Barovia" {
		t.Fatalf("expected root 'Barovia', got %q", tree[0].Name)
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected 1 child of Barovia, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].Name != "Village of Barovia" {
		t.Fatalf("expected child 'Village of Barovia', got %q", tree[0].Children[0].Name)
	}
	if len(tree[0].Children[0].Children) != 1 {
		t.Fatalf("expected 1 child of Village of Barovia, got %d", len(tree[0].Children[0].Children))
	}
	if tree[0].Children[0].Children[0].Name != "Blood on the Vine Tavern" {
		t.Fatalf("expected grandchild 'Blood on the Vine Tavern', got %q", tree[0].Children[0].Children[0].Name)
	}
}

func TestHandleLocationHierarchy_Empty(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Empty Hierarchy Camp", "")

	url := fmt.Sprintf("/api/campaigns/%d/location-hierarchy", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var tree []any
	if err := json.NewDecoder(rec.Body).Decode(&tree); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(tree) != 0 {
		t.Fatalf("expected empty array, got %d items", len(tree))
	}
}

// ---------------------------------------------------------------------------
// Entity Timeline handler tests
// ---------------------------------------------------------------------------

func TestHandleGetEntityTimeline(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Entity Timeline Camp", "")

	url := fmt.Sprintf("/api/campaigns/%d/entity-timeline", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []storage.EntityTimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Valid JSON array (possibly empty for a new campaign).
	if entries == nil {
		t.Fatal("expected non-nil entity timeline array")
	}
}

func TestHandleGetEntityTimeline_WithData(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	srv := NewServer(store, ":0", guildID, "")

	campID, _ := store.CreateCampaign(ctx, guildID, "Timeline Data Camp", "")
	entityID, _ := store.UpsertEntity(ctx, campID, "Test Hero", "npc", "A brave hero")
	sessID, _ := store.CreateSession(ctx, guildID, campID, "chan-1", "/tmp/audio")

	// Insert a segment for the entity reference.
	var segID int64
	store.Pool.QueryRow(ctx,
		`INSERT INTO transcript_segments (session_id, user_id, display_name, start_time, end_time, text)
		 VALUES ($1, 'u1', 'User1', 0, 10, 'text') RETURNING id`, sessID).Scan(&segID)

	store.InsertEntityReferences(ctx, []storage.EntityReference{
		{EntityID: entityID, SessionID: sessID, SegmentID: &segID, Context: "hero ref"},
	})

	url := fmt.Sprintf("/api/campaigns/%d/entity-timeline", campID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var entries []storage.EntityTimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&entries); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(entries) < 1 {
		t.Fatal("expected at least 1 entity timeline entry")
	}
	if entries[0].EntityName != "Test Hero" {
		t.Fatalf("expected entity_name 'Test Hero', got %q", entries[0].EntityName)
	}
	if entries[0].TotalMentions != 1 {
		t.Fatalf("expected total_mentions 1, got %d", entries[0].TotalMentions)
	}
}

func TestHandleGetEntityTimeline_InvalidID(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns/abc/entity-timeline", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetEntity_NotFound(t *testing.T) {
	store := testStore(t)
	srv := NewServer(store, ":0", "test-guild", "")

	req := httptest.NewRequest(http.MethodGet, "/api/entities/999999", nil)
	rec := httptest.NewRecorder()

	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
