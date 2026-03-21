package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	migrationsFS := os.DirFS("../../migrations")
	store, err := New(context.Background(), dbURL, migrationsFS)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// uniqueGuild returns a guild ID unique to the current test to avoid collisions.
func uniqueGuild(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-guild-%d", time.Now().UnixNano())
}

// ---------------------------------------------------------------------------
// Campaigns
// ---------------------------------------------------------------------------

func TestCreateCampaign(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	id, err := store.CreateCampaign(ctx, guildID, "Storm King's Thunder", "Giants awaken")
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero campaign id")
	}

	c, err := store.GetCampaign(ctx, id)
	if err != nil {
		t.Fatalf("GetCampaign: %v", err)
	}
	if c.Name != "Storm King's Thunder" {
		t.Fatalf("expected name 'Storm King's Thunder', got %q", c.Name)
	}
	if c.Description != "Giants awaken" {
		t.Fatalf("expected description 'Giants awaken', got %q", c.Description)
	}
	if c.GuildID != guildID {
		t.Fatalf("expected guild_id %q, got %q", guildID, c.GuildID)
	}
	if c.IsActive {
		t.Fatal("expected new campaign to not be active")
	}
}

func TestListCampaigns(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	_, err := store.CreateCampaign(ctx, guildID, "Campaign A", "")
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	_, err = store.CreateCampaign(ctx, guildID, "Campaign B", "")
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	campaigns, err := store.ListCampaigns(ctx, guildID)
	if err != nil {
		t.Fatalf("ListCampaigns: %v", err)
	}
	if len(campaigns) < 2 {
		t.Fatalf("expected at least 2 campaigns, got %d", len(campaigns))
	}

	// Ordered by created_at ascending.
	if campaigns[0].Name != "Campaign A" {
		t.Fatalf("expected first campaign 'Campaign A', got %q", campaigns[0].Name)
	}
	if campaigns[1].Name != "Campaign B" {
		t.Fatalf("expected second campaign 'Campaign B', got %q", campaigns[1].Name)
	}
}

func TestSetActiveCampaign(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	idA, _ := store.CreateCampaign(ctx, guildID, "Camp A", "")
	idB, _ := store.CreateCampaign(ctx, guildID, "Camp B", "")

	// Activate A.
	if err := store.SetActiveCampaign(ctx, guildID, idA); err != nil {
		t.Fatalf("SetActiveCampaign A: %v", err)
	}
	a, _ := store.GetCampaign(ctx, idA)
	if !a.IsActive {
		t.Fatal("expected campaign A to be active")
	}

	// Activate B; A should be deactivated.
	if err := store.SetActiveCampaign(ctx, guildID, idB); err != nil {
		t.Fatalf("SetActiveCampaign B: %v", err)
	}
	a, _ = store.GetCampaign(ctx, idA)
	b, _ := store.GetCampaign(ctx, idB)
	if a.IsActive {
		t.Fatal("expected campaign A to no longer be active")
	}
	if !b.IsActive {
		t.Fatal("expected campaign B to be active")
	}
}

func TestGetActiveCampaign(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	// No active campaign yet.
	c, err := store.GetActiveCampaign(ctx, guildID)
	if err != nil {
		t.Fatalf("GetActiveCampaign: %v", err)
	}
	if c != nil {
		t.Fatal("expected nil when no active campaign")
	}

	id, _ := store.CreateCampaign(ctx, guildID, "Active One", "")
	_ = store.SetActiveCampaign(ctx, guildID, id)

	c, err = store.GetActiveCampaign(ctx, guildID)
	if err != nil {
		t.Fatalf("GetActiveCampaign after set: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil active campaign")
	}
	if c.ID != id {
		t.Fatalf("expected active campaign id %d, got %d", id, c.ID)
	}
}

func TestGetOrCreateActiveCampaign(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	// Should auto-create a default campaign.
	c, err := store.GetOrCreateActiveCampaign(ctx, guildID)
	if err != nil {
		t.Fatalf("GetOrCreateActiveCampaign: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil campaign")
	}
	if c.Name != "Default Campaign" {
		t.Fatalf("expected 'Default Campaign', got %q", c.Name)
	}
	if !c.IsActive {
		t.Fatal("expected auto-created campaign to be active")
	}

	// Calling again should return the same campaign.
	c2, err := store.GetOrCreateActiveCampaign(ctx, guildID)
	if err != nil {
		t.Fatalf("GetOrCreateActiveCampaign second call: %v", err)
	}
	if c2.ID != c.ID {
		t.Fatalf("expected same campaign id %d, got %d", c.ID, c2.ID)
	}
}

func TestUpdateCampaignRecap(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	id, _ := store.CreateCampaign(ctx, guildID, "Recap Test", "")

	recap := "The heroes ventured into the Underdark..."
	if err := store.UpdateCampaignRecap(ctx, id, recap); err != nil {
		t.Fatalf("UpdateCampaignRecap: %v", err)
	}

	c, _ := store.GetCampaign(ctx, id)
	if c.Recap != recap {
		t.Fatalf("expected recap %q, got %q", recap, c.Recap)
	}
	if c.RecapGeneratedAt == nil {
		t.Fatal("expected recap_generated_at to be set")
	}
}

func TestSetCampaignDM(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	id, _ := store.CreateCampaign(ctx, guildID, "DM Test", "")

	if err := store.SetCampaignDM(ctx, id, "dm-user-42"); err != nil {
		t.Fatalf("SetCampaignDM: %v", err)
	}

	c, _ := store.GetCampaign(ctx, id)
	if c.DMUserID == nil || *c.DMUserID != "dm-user-42" {
		t.Fatalf("expected dm_user_id 'dm-user-42', got %v", c.DMUserID)
	}
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func createTestCampaign(t *testing.T, store *Store, guildID string) int64 {
	t.Helper()
	id, err := store.CreateCampaign(context.Background(), guildID, "Test Campaign", "")
	if err != nil {
		t.Fatalf("create test campaign: %v", err)
	}
	return id
}

func TestCreateSession(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id, err := store.CreateSession(ctx, guildID, campID, "chan-1", "/tmp/audio")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero session id")
	}

	sess, err := store.GetSession(ctx, id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.GuildID != guildID {
		t.Fatalf("expected guild_id %q, got %q", guildID, sess.GuildID)
	}
	if sess.CampaignID != campID {
		t.Fatalf("expected campaign_id %d, got %d", campID, sess.CampaignID)
	}
	if sess.Status != "recording" {
		t.Fatalf("expected status 'recording', got %q", sess.Status)
	}
	if sess.AudioDir != "/tmp/audio" {
		t.Fatalf("expected audio_dir '/tmp/audio', got %q", sess.AudioDir)
	}
}

func TestListSessions(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campA := createTestCampaign(t, store, guildID)
	campB, _ := store.CreateCampaign(ctx, guildID, "Campaign B", "")

	store.CreateSession(ctx, guildID, campA, "ch-1", "/tmp/a1")
	store.CreateSession(ctx, guildID, campA, "ch-2", "/tmp/a2")
	store.CreateSession(ctx, guildID, campB, "ch-3", "/tmp/b1")

	// Filter by campaign A.
	sessions, err := store.ListSessions(ctx, guildID, campA, 10, 0)
	if err != nil {
		t.Fatalf("ListSessions campA: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions for campA, got %d", len(sessions))
	}

	// All campaigns (campaignID = 0).
	all, err := store.ListSessions(ctx, guildID, 0, 10, 0)
	if err != nil {
		t.Fatalf("ListSessions all: %v", err)
	}
	if len(all) < 3 {
		t.Fatalf("expected at least 3 sessions total, got %d", len(all))
	}
}

func TestSessionLifecycle(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/lc")

	// End the session.
	if err := store.EndSession(ctx, id); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	sess, _ := store.GetSession(ctx, id)
	if sess.Status != "transcribing" {
		t.Fatalf("expected 'transcribing' after end, got %q", sess.Status)
	}
	if sess.EndedAt == nil {
		t.Fatal("expected ended_at to be set")
	}

	// Update status.
	if err := store.UpdateSessionStatus(ctx, id, "summarising"); err != nil {
		t.Fatalf("UpdateSessionStatus: %v", err)
	}
	sess, _ = store.GetSession(ctx, id)
	if sess.Status != "summarising" {
		t.Fatalf("expected 'summarising', got %q", sess.Status)
	}

	// Update summary.
	events := []string{"Fought the dragon", "Found the treasure"}
	if err := store.UpdateSessionSummary(ctx, id, "Epic battle", events); err != nil {
		t.Fatalf("UpdateSessionSummary: %v", err)
	}
	sess, _ = store.GetSession(ctx, id)
	if sess.Status != "complete" {
		t.Fatalf("expected 'complete', got %q", sess.Status)
	}
	if sess.Summary == nil || *sess.Summary != "Epic battle" {
		t.Fatalf("expected summary 'Epic battle', got %v", sess.Summary)
	}
	if len(sess.KeyEvents) != 2 {
		t.Fatalf("expected 2 key events, got %d", len(sess.KeyEvents))
	}
}

func TestCleanupStaleSessions(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id1, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/s1")
	id2, _ := store.CreateSession(ctx, guildID, campID, "ch-2", "/tmp/s2")

	// Both are "recording". Cleanup should mark them failed.
	affected, err := store.CleanupStaleSessions(ctx)
	if err != nil {
		t.Fatalf("CleanupStaleSessions: %v", err)
	}
	if affected < 2 {
		t.Fatalf("expected at least 2 affected, got %d", affected)
	}

	s1, _ := store.GetSession(ctx, id1)
	s2, _ := store.GetSession(ctx, id2)
	if s1.Status != "failed" {
		t.Fatalf("expected session 1 status 'failed', got %q", s1.Status)
	}
	if s2.Status != "failed" {
		t.Fatalf("expected session 2 status 'failed', got %q", s2.Status)
	}
}

func TestGetActiveSession(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// No active session.
	sess, err := store.GetActiveSession(ctx, guildID)
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if sess != nil {
		t.Fatal("expected nil when no active session")
	}

	id, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/act")
	sess, err = store.GetActiveSession(ctx, guildID)
	if err != nil {
		t.Fatalf("GetActiveSession after create: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil active session")
	}
	if sess.ID != id {
		t.Fatalf("expected session id %d, got %d", id, sess.ID)
	}

	// End it; no active session anymore.
	_ = store.EndSession(ctx, id)
	sess, _ = store.GetActiveSession(ctx, guildID)
	if sess != nil {
		t.Fatal("expected nil after ending session")
	}
}

// ---------------------------------------------------------------------------
// Characters
// ---------------------------------------------------------------------------

func TestCharacterMapping(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	m := CharacterMapping{
		UserID:        "user-char-1",
		GuildID:       guildID,
		CampaignID:    campID,
		CharacterName: "Gandalf",
	}
	if err := store.SetCharacterMapping(ctx, m); err != nil {
		t.Fatalf("SetCharacterMapping: %v", err)
	}

	name, err := store.GetCharacterName(ctx, "user-char-1", campID)
	if err != nil {
		t.Fatalf("GetCharacterName: %v", err)
	}
	if name != "Gandalf" {
		t.Fatalf("expected 'Gandalf', got %q", name)
	}

	mappings, err := store.GetCharacterMappings(ctx, campID)
	if err != nil {
		t.Fatalf("GetCharacterMappings: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
	if mappings[0].CharacterName != "Gandalf" {
		t.Fatalf("expected 'Gandalf', got %q", mappings[0].CharacterName)
	}
}

func TestCharacterMappingCampaignScope(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campA := createTestCampaign(t, store, guildID)
	campB, _ := store.CreateCampaign(ctx, guildID, "Campaign B", "")

	_ = store.SetCharacterMapping(ctx, CharacterMapping{
		UserID: "user-scope-1", GuildID: guildID, CampaignID: campA, CharacterName: "Aragorn",
	})
	_ = store.SetCharacterMapping(ctx, CharacterMapping{
		UserID: "user-scope-1", GuildID: guildID, CampaignID: campB, CharacterName: "Legolas",
	})

	nameA, _ := store.GetCharacterName(ctx, "user-scope-1", campA)
	nameB, _ := store.GetCharacterName(ctx, "user-scope-1", campB)

	if nameA != "Aragorn" {
		t.Fatalf("expected 'Aragorn' in campaign A, got %q", nameA)
	}
	if nameB != "Legolas" {
		t.Fatalf("expected 'Legolas' in campaign B, got %q", nameB)
	}
}

func TestDeleteCharacterMapping(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	_ = store.SetCharacterMapping(ctx, CharacterMapping{
		UserID: "user-del-1", GuildID: guildID, CampaignID: campID, CharacterName: "Frodo",
	})

	if err := store.DeleteCharacterMapping(ctx, "user-del-1", campID); err != nil {
		t.Fatalf("DeleteCharacterMapping: %v", err)
	}

	name, _ := store.GetCharacterName(ctx, "user-del-1", campID)
	if name != "" {
		t.Fatalf("expected empty name after delete, got %q", name)
	}
}

// ---------------------------------------------------------------------------
// Entities
// ---------------------------------------------------------------------------

func TestUpsertEntity(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id, err := store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire lord of Barovia")
	if err != nil {
		t.Fatalf("UpsertEntity create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero entity id")
	}

	e, _ := store.GetEntity(ctx, id)
	if e.Name != "Strahd" {
		t.Fatalf("expected 'Strahd', got %q", e.Name)
	}
	if e.Description != "Vampire lord of Barovia" {
		t.Fatalf("expected description 'Vampire lord of Barovia', got %q", e.Description)
	}

	// Upsert with new description should update it.
	id2, err := store.UpsertEntity(ctx, campID, "Strahd", "npc", "Ancient vampire overlord")
	if err != nil {
		t.Fatalf("UpsertEntity update: %v", err)
	}
	if id2 != id {
		t.Fatalf("expected same entity id %d on upsert, got %d", id, id2)
	}

	e, _ = store.GetEntity(ctx, id)
	if e.Description != "Ancient vampire overlord" {
		t.Fatalf("expected updated description, got %q", e.Description)
	}

	// Upsert with empty description should keep existing.
	store.UpsertEntity(ctx, campID, "Strahd", "npc", "")
	e, _ = store.GetEntity(ctx, id)
	if e.Description != "Ancient vampire overlord" {
		t.Fatalf("expected description preserved on empty upsert, got %q", e.Description)
	}
}

func TestListEntities(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	store.UpsertEntity(ctx, campID, "Barovia", "location", "A dark land")
	store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire")
	store.UpsertEntity(ctx, campID, "Sunblade", "item", "Radiant weapon")

	// All entities.
	all, err := store.ListEntities(ctx, campID, "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListEntities all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(all))
	}

	// Filter by type.
	npcs, err := store.ListEntities(ctx, campID, "npc", "", 50, 0)
	if err != nil {
		t.Fatalf("ListEntities type filter: %v", err)
	}
	if len(npcs) != 1 {
		t.Fatalf("expected 1 npc, got %d", len(npcs))
	}

	// Search by name (ILIKE).
	results, err := store.ListEntities(ctx, campID, "", "strahd", 50, 0)
	if err != nil {
		t.Fatalf("ListEntities search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'strahd', got %d", len(results))
	}
	if results[0].Name != "Strahd" {
		t.Fatalf("expected 'Strahd', got %q", results[0].Name)
	}
}

func TestEntityNotes(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/notes")
	entID, _ := store.UpsertEntity(ctx, campID, "NoteTarget", "npc", "A notable NPC")

	if err := store.AddEntityNote(ctx, entID, sessID, "First encounter in the tavern"); err != nil {
		t.Fatalf("AddEntityNote 1: %v", err)
	}
	if err := store.AddEntityNote(ctx, entID, sessID, "Revealed their true identity"); err != nil {
		t.Fatalf("AddEntityNote 2: %v", err)
	}

	notes, err := store.GetEntityNotes(ctx, entID)
	if err != nil {
		t.Fatalf("GetEntityNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	// Ordered by created_at.
	if notes[0].Content != "First encounter in the tavern" {
		t.Fatalf("expected first note content, got %q", notes[0].Content)
	}
	if notes[1].Content != "Revealed their true identity" {
		t.Fatalf("expected second note content, got %q", notes[1].Content)
	}
}

func TestEntityRelationships(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	srcID, _ := store.UpsertEntity(ctx, campID, "Alice", "npc", "A wizard")
	tgtID, _ := store.UpsertEntity(ctx, campID, "Bob", "npc", "A fighter")

	err := store.UpsertEntityRelationship(ctx, campID, srcID, tgtID, "ally", "Fought together", nil)
	if err != nil {
		t.Fatalf("UpsertEntityRelationship: %v", err)
	}

	// Get from source side.
	rels, err := store.GetEntityRelationships(ctx, srcID)
	if err != nil {
		t.Fatalf("GetEntityRelationships source: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship from source, got %d", len(rels))
	}
	if rels[0].Relationship != "ally" {
		t.Fatalf("expected 'ally', got %q", rels[0].Relationship)
	}

	// Get from target side.
	rels, err = store.GetEntityRelationships(ctx, tgtID)
	if err != nil {
		t.Fatalf("GetEntityRelationships target: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relationship from target, got %d", len(rels))
	}

	// Upsert should update description.
	err = store.UpsertEntityRelationship(ctx, campID, srcID, tgtID, "ally", "Lifelong friends", nil)
	if err != nil {
		t.Fatalf("UpsertEntityRelationship update: %v", err)
	}
	rels, _ = store.GetEntityRelationships(ctx, srcID)
	if rels[0].Description != "Lifelong friends" {
		t.Fatalf("expected updated description 'Lifelong friends', got %q", rels[0].Description)
	}
}

func TestGetEntityByName(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	store.UpsertEntity(ctx, campID, "Waterdeep", "location", "City of Splendors")

	e, err := store.GetEntityByName(ctx, campID, "Waterdeep", "location")
	if err != nil {
		t.Fatalf("GetEntityByName: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil entity")
	}
	if e.Name != "Waterdeep" {
		t.Fatalf("expected 'Waterdeep', got %q", e.Name)
	}

	// Non-existent.
	e, err = store.GetEntityByName(ctx, campID, "Nowhere", "location")
	if err != nil {
		t.Fatalf("GetEntityByName non-existent: %v", err)
	}
	if e != nil {
		t.Fatal("expected nil for non-existent entity")
	}
}

func TestEnsurePCEntities(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	names := []string{"Thordak", "Elara", "Grimjaw"}
	ids, err := store.EnsurePCEntities(ctx, campID, names)
	if err != nil {
		t.Fatalf("EnsurePCEntities: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}

	// All returned IDs should be non-zero and correspond to pc entities.
	for _, name := range names {
		id, ok := ids[name]
		if !ok {
			t.Fatalf("missing ID for %q", name)
		}
		if id == 0 {
			t.Fatalf("expected non-zero ID for %q", name)
		}
		e, err := store.GetEntity(ctx, id)
		if err != nil {
			t.Fatalf("GetEntity(%d): %v", id, err)
		}
		if e.Type != "pc" {
			t.Fatalf("expected type 'pc' for %q, got %q", name, e.Type)
		}
		if e.Name != name {
			t.Fatalf("expected name %q, got %q", name, e.Name)
		}
	}

	// Calling again should return the same IDs (idempotent).
	ids2, err := store.EnsurePCEntities(ctx, campID, names)
	if err != nil {
		t.Fatalf("EnsurePCEntities second call: %v", err)
	}
	for _, name := range names {
		if ids[name] != ids2[name] {
			t.Fatalf("expected same ID for %q on second call: %d vs %d", name, ids[name], ids2[name])
		}
	}

	// Empty list should return empty map without error.
	empty, err := store.EnsurePCEntities(ctx, campID, nil)
	if err != nil {
		t.Fatalf("EnsurePCEntities empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(empty))
	}
}

// ---------------------------------------------------------------------------
// Quests
// ---------------------------------------------------------------------------

func TestUpsertQuest(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id, err := store.UpsertQuest(ctx, campID, "Slay the Dragon", "Defeat the red dragon", "active", "King Hekaton")
	if err != nil {
		t.Fatalf("UpsertQuest create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero quest id")
	}

	q, _ := store.GetQuest(ctx, id)
	if q.Name != "Slay the Dragon" {
		t.Fatalf("expected 'Slay the Dragon', got %q", q.Name)
	}
	if q.Status != "active" {
		t.Fatalf("expected status 'active', got %q", q.Status)
	}
	if q.Giver != "King Hekaton" {
		t.Fatalf("expected giver 'King Hekaton', got %q", q.Giver)
	}

	// Upsert with updated description.
	id2, err := store.UpsertQuest(ctx, campID, "Slay the Dragon", "Defeat the ancient red dragon", "", "")
	if err != nil {
		t.Fatalf("UpsertQuest update: %v", err)
	}
	if id2 != id {
		t.Fatalf("expected same quest id on upsert, got %d", id2)
	}
	q, _ = store.GetQuest(ctx, id)
	if q.Description != "Defeat the ancient red dragon" {
		t.Fatalf("expected updated description, got %q", q.Description)
	}
	// Empty status/giver should preserve originals.
	if q.Status != "active" {
		t.Fatalf("expected status preserved, got %q", q.Status)
	}
	if q.Giver != "King Hekaton" {
		t.Fatalf("expected giver preserved, got %q", q.Giver)
	}
}

func TestListQuests(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	store.UpsertQuest(ctx, campID, "Quest Active", "desc", "active", "NPC1")
	store.UpsertQuest(ctx, campID, "Quest Complete", "desc", "completed", "NPC2")
	store.UpsertQuest(ctx, campID, "Quest Failed", "desc", "failed", "NPC3")

	// All quests.
	all, err := store.ListQuests(ctx, campID, "")
	if err != nil {
		t.Fatalf("ListQuests all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 quests, got %d", len(all))
	}

	// Filter by status.
	active, err := store.ListQuests(ctx, campID, "active")
	if err != nil {
		t.Fatalf("ListQuests active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active quest, got %d", len(active))
	}
	if active[0].Name != "Quest Active" {
		t.Fatalf("expected 'Quest Active', got %q", active[0].Name)
	}
}

func TestUpdateQuestStatus(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id, _ := store.UpsertQuest(ctx, campID, "Status Quest", "test", "active", "")

	if err := store.UpdateQuestStatus(ctx, id, "completed"); err != nil {
		t.Fatalf("UpdateQuestStatus: %v", err)
	}

	q, _ := store.GetQuest(ctx, id)
	if q.Status != "completed" {
		t.Fatalf("expected 'completed', got %q", q.Status)
	}
}

func TestQuestUpdates(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/qu")
	questID, _ := store.UpsertQuest(ctx, campID, "Updated Quest", "test", "active", "")

	newStatus := "completed"
	if err := store.AddQuestUpdate(ctx, questID, sessID, "Dragon was slain", &newStatus); err != nil {
		t.Fatalf("AddQuestUpdate: %v", err)
	}
	if err := store.AddQuestUpdate(ctx, questID, sessID, "Treasure recovered", nil); err != nil {
		t.Fatalf("AddQuestUpdate 2: %v", err)
	}

	updates, err := store.GetQuestUpdates(ctx, questID)
	if err != nil {
		t.Fatalf("GetQuestUpdates: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if updates[0].Content != "Dragon was slain" {
		t.Fatalf("expected first update 'Dragon was slain', got %q", updates[0].Content)
	}
	if updates[0].NewStatus == nil || *updates[0].NewStatus != "completed" {
		t.Fatalf("expected first update new_status 'completed', got %v", updates[0].NewStatus)
	}
	if updates[1].NewStatus != nil {
		t.Fatalf("expected second update new_status nil, got %v", updates[1].NewStatus)
	}
}

// ---------------------------------------------------------------------------
// Timeline
// ---------------------------------------------------------------------------

func TestGetCampaignTimeline(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Create some data to populate the timeline.
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/tl")
	_ = store.EndSession(ctx, sessID)
	_ = store.UpdateSessionSummary(ctx, sessID, "A grand adventure", []string{"event1"})

	store.UpsertEntity(ctx, campID, "Timeline NPC", "npc", "A character in the timeline")
	store.UpsertQuest(ctx, campID, "Timeline Quest", "A quest", "active", "NPC")

	events, err := store.GetCampaignTimeline(ctx, campID, 50, 0)
	if err != nil {
		t.Fatalf("GetCampaignTimeline: %v", err)
	}

	// Should have at least: 1 session + 1 entity + 1 quest = 3 events.
	if len(events) < 3 {
		t.Fatalf("expected at least 3 timeline events, got %d", len(events))
	}

	// Check that we have the expected types.
	typeSet := make(map[string]bool)
	for _, e := range events {
		typeSet[e.Type] = true
	}
	for _, typ := range []string{"session", "entity", "quest_new"} {
		if !typeSet[typ] {
			t.Fatalf("expected timeline type %q present", typ)
		}
	}
}

// ---------------------------------------------------------------------------
// Lore Search
// ---------------------------------------------------------------------------

func TestSearchLore(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Create searchable data.
	entID, _ := store.UpsertEntity(ctx, campID, "Silverymoon", "location", "A beautiful city of magic")
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/ls")
	store.AddEntityNote(ctx, entID, sessID, "The party arrived at Silverymoon")
	_ = store.EndSession(ctx, sessID)
	_ = store.UpdateSessionSummary(ctx, sessID, "The heroes explored Silverymoon and met the archmage", []string{})
	store.UpsertQuest(ctx, campID, "Defend Silverymoon", "Protect the city from orcs", "active", "Archmage")

	results, err := store.SearchLore(ctx, campID, "Silverymoon", 20)
	if err != nil {
		t.Fatalf("SearchLore: %v", err)
	}

	// Should find hits across entity, note, summary, and quest.
	if len(results) < 3 {
		t.Fatalf("expected at least 3 lore results for 'Silverymoon', got %d", len(results))
	}

	typeSet := make(map[string]bool)
	for _, r := range results {
		typeSet[r.Type] = true
	}
	if !typeSet["entity"] {
		t.Fatal("expected entity in lore search results")
	}
	if !typeSet["note"] {
		t.Fatal("expected note in lore search results")
	}
	if !typeSet["summary"] {
		t.Fatal("expected summary in lore search results")
	}
}

// ---------------------------------------------------------------------------
// Discord Users
// ---------------------------------------------------------------------------

func TestUpsertDiscordUsers(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	users := []DiscordUser{
		{UserID: "u1", GuildID: guildID, Username: "alice", DisplayName: "Alice A"},
		{UserID: "u2", GuildID: guildID, Username: "bob", DisplayName: "Bob B"},
	}

	if err := store.UpsertDiscordUsers(ctx, users); err != nil {
		t.Fatalf("UpsertDiscordUsers: %v", err)
	}

	all, err := store.GetDiscordUsers(ctx, guildID)
	if err != nil {
		t.Fatalf("GetDiscordUsers: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 users, got %d", len(all))
	}

	// Upsert again with updated display name.
	users[0].DisplayName = "Alice Updated"
	if err := store.UpsertDiscordUsers(ctx, users); err != nil {
		t.Fatalf("UpsertDiscordUsers update: %v", err)
	}

	u, err := store.GetDiscordUser(ctx, "u1", guildID)
	if err != nil {
		t.Fatalf("GetDiscordUser: %v", err)
	}
	if u.DisplayName != "Alice Updated" {
		t.Fatalf("expected 'Alice Updated', got %q", u.DisplayName)
	}
}

func TestGetDiscordUser(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)

	if err := store.UpsertDiscordUser(ctx, DiscordUser{
		UserID: "single-u", GuildID: guildID, Username: "charlie", DisplayName: "Charlie C",
	}); err != nil {
		t.Fatalf("UpsertDiscordUser: %v", err)
	}

	u, err := store.GetDiscordUser(ctx, "single-u", guildID)
	if err != nil {
		t.Fatalf("GetDiscordUser: %v", err)
	}
	if u.Username != "charlie" {
		t.Fatalf("expected 'charlie', got %q", u.Username)
	}
	if u.DisplayName != "Charlie C" {
		t.Fatalf("expected 'Charlie C', got %q", u.DisplayName)
	}
}

// ---------------------------------------------------------------------------
// Transcripts
// ---------------------------------------------------------------------------

func TestInsertAndGetTranscript(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/tr")

	segments := []TranscriptSegment{
		{SessionID: sessID, UserID: "u1", StartTime: 0.0, EndTime: 3.5, Text: "Hello everyone"},
		{SessionID: sessID, UserID: "u2", StartTime: 4.0, EndTime: 7.0, Text: "Let us begin"},
		{SessionID: sessID, UserID: "u1", StartTime: 8.0, EndTime: 12.0, Text: "I cast fireball"},
	}

	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("InsertSegments: %v", err)
	}

	result, err := store.GetTranscript(ctx, sessID)
	if err != nil {
		t.Fatalf("GetTranscript: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(result))
	}

	// Ordered by start_time.
	if result[0].Text != "Hello everyone" {
		t.Fatalf("expected first segment 'Hello everyone', got %q", result[0].Text)
	}
	if result[1].Text != "Let us begin" {
		t.Fatalf("expected second segment 'Let us begin', got %q", result[1].Text)
	}
	if result[2].Text != "I cast fireball" {
		t.Fatalf("expected third segment 'I cast fireball', got %q", result[2].Text)
	}
	if result[0].UserID != "u1" {
		t.Fatalf("expected user_id 'u1', got %q", result[0].UserID)
	}

	// Getting transcript for non-existent session should return empty.
	empty, err := store.GetTranscript(ctx, 999999)
	if err != nil {
		t.Fatalf("GetTranscript non-existent: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 segments for non-existent session, got %d", len(empty))
	}
}

func TestInsertEntityReferences(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/er")

	// Insert transcript segments so we have segment IDs.
	segments := []TranscriptSegment{
		{SessionID: sessID, UserID: "u1", StartTime: 0.0, EndTime: 5.0, Text: "We met Strahd"},
		{SessionID: sessID, UserID: "u2", StartTime: 6.0, EndTime: 10.0, Text: "In Barovia"},
	}
	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("InsertSegments: %v", err)
	}
	segs, _ := store.GetTranscript(ctx, sessID)

	entID, _ := store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire lord")

	seg0ID := segs[0].ID
	seg1ID := segs[1].ID
	refs := []EntityReference{
		{EntityID: entID, SessionID: sessID, SegmentID: &seg0ID, Context: "We met Strahd"},
		{EntityID: entID, SessionID: sessID, SegmentID: &seg1ID, Context: "In Barovia"},
	}

	if err := store.InsertEntityReferences(ctx, refs); err != nil {
		t.Fatalf("InsertEntityReferences: %v", err)
	}

	// Insert again — should not fail (ON CONFLICT DO NOTHING).
	if err := store.InsertEntityReferences(ctx, refs); err != nil {
		t.Fatalf("InsertEntityReferences duplicate: %v", err)
	}

	// Verify retrieval.
	got, err := store.GetEntityReferences(ctx, entID, 50, 0)
	if err != nil {
		t.Fatalf("GetEntityReferences: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 references, got %d", len(got))
	}
	if got[0].Context != "We met Strahd" {
		t.Fatalf("expected context 'We met Strahd', got %q", got[0].Context)
	}
}

func TestGetLatestCompleteSessions(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	var completeIDs []int64
	for i := 0; i < 3; i++ {
		id, err := store.CreateSession(ctx, guildID, campID, "ch-1", fmt.Sprintf("/tmp/latest-%d", i))
		if err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
		summary := fmt.Sprintf("Summary for session %d", i+1)
		if err := store.UpdateSessionSummary(ctx, id, summary, []string{"event"}); err != nil {
			t.Fatalf("UpdateSessionSummary %d: %v", i, err)
		}
		completeIDs = append(completeIDs, id)
		// Small sleep to ensure distinct started_at ordering.
		time.Sleep(10 * time.Millisecond)
	}

	// Session 4: complete but no summary (just status change).
	id4, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/latest-nosummary")
	store.UpdateSessionStatus(ctx, id4, "complete")

	// Session 5: still recording.
	store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/latest-recording")

	// Request last 2 — should return the 2 most recent complete sessions with summaries.
	sessions, err := store.GetLatestCompleteSessions(ctx, campID, 2)
	if err != nil {
		t.Fatalf("GetLatestCompleteSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Should be in chronological order (oldest first).
	if sessions[0].ID != completeIDs[1] {
		t.Errorf("expected first session ID %d, got %d", completeIDs[1], sessions[0].ID)
	}
	if sessions[1].ID != completeIDs[2] {
		t.Errorf("expected second session ID %d, got %d", completeIDs[2], sessions[1].ID)
	}
	// Summaries should be non-nil.
	for i, s := range sessions {
		if s.Summary == nil || *s.Summary == "" {
			t.Errorf("session %d: expected non-empty summary", i)
		}
	}

	// Request more than available — should return all 3 complete sessions with summaries.
	all, err := store.GetLatestCompleteSessions(ctx, campID, 100)
	if err != nil {
		t.Fatalf("GetLatestCompleteSessions all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(all))
	}
	// Chronological order check.
	if all[0].ID >= all[1].ID || all[1].ID >= all[2].ID {
		t.Error("sessions should be in chronological order (oldest first)")
	}

	// Request 0 — edge case, should return empty.
	zero, err := store.GetLatestCompleteSessions(ctx, campID, 0)
	if err != nil {
		t.Fatalf("GetLatestCompleteSessions zero: %v", err)
	}
	if len(zero) != 0 {
		t.Fatalf("expected 0 sessions for n=0, got %d", len(zero))
	}
}

func TestGetEntitySessionAppearances(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	sess1ID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/esa1")
	sess2ID, _ := store.CreateSession(ctx, guildID, campID, "ch-2", "/tmp/esa2")

	// Insert segments for both sessions.
	segs1 := []TranscriptSegment{
		{SessionID: sess1ID, UserID: "u1", StartTime: 0.0, EndTime: 5.0, Text: "seg1"},
		{SessionID: sess1ID, UserID: "u1", StartTime: 6.0, EndTime: 10.0, Text: "seg2"},
	}
	segs2 := []TranscriptSegment{
		{SessionID: sess2ID, UserID: "u1", StartTime: 0.0, EndTime: 5.0, Text: "seg3"},
	}
	store.InsertSegments(ctx, segs1)
	store.InsertSegments(ctx, segs2)

	got1, _ := store.GetTranscript(ctx, sess1ID)
	got2, _ := store.GetTranscript(ctx, sess2ID)

	entID, _ := store.UpsertEntity(ctx, campID, "TestEntity", "npc", "Test")

	seg1aID := got1[0].ID
	seg1bID := got1[1].ID
	seg2aID := got2[0].ID
	refs := []EntityReference{
		{EntityID: entID, SessionID: sess1ID, SegmentID: &seg1aID, Context: "ref1"},
		{EntityID: entID, SessionID: sess1ID, SegmentID: &seg1bID, Context: "ref2"},
		{EntityID: entID, SessionID: sess2ID, SegmentID: &seg2aID, Context: "ref3"},
	}
	store.InsertEntityReferences(ctx, refs)

	appearances, err := store.GetEntitySessionAppearances(ctx, entID)
	if err != nil {
		t.Fatalf("GetEntitySessionAppearances: %v", err)
	}
	if len(appearances) != 2 {
		t.Fatalf("expected 2 session appearances, got %d", len(appearances))
	}

	// Sessions should be ordered by started_at. Both were created close together,
	// so we just check the counts.
	totalMentions := 0
	for _, a := range appearances {
		totalMentions += a.MentionCount
	}
	if totalMentions != 3 {
		t.Fatalf("expected 3 total mentions, got %d", totalMentions)
	}

	// Find the appearance for sess1 and verify count.
	for _, a := range appearances {
		if a.SessionID == sess1ID {
			if a.MentionCount != 2 {
				t.Fatalf("expected 2 mentions in session 1, got %d", a.MentionCount)
			}
		}
		if a.SessionID == sess2ID {
			if a.MentionCount != 1 {
				t.Fatalf("expected 1 mention in session 2, got %d", a.MentionCount)
			}
		}
	}
}

func TestDeleteEntityReferencesForSession(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/dere")

	segments := []TranscriptSegment{
		{SessionID: sessID, UserID: "u1", StartTime: 0.0, EndTime: 5.0, Text: "test"},
	}
	store.InsertSegments(ctx, segments)
	segs, _ := store.GetTranscript(ctx, sessID)

	entID, _ := store.UpsertEntity(ctx, campID, "DelTest", "npc", "Test")
	segID := segs[0].ID
	refs := []EntityReference{
		{EntityID: entID, SessionID: sessID, SegmentID: &segID, Context: "test context"},
	}
	store.InsertEntityReferences(ctx, refs)

	// Verify it exists.
	got, _ := store.GetEntityReferences(ctx, entID, 50, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 reference before delete, got %d", len(got))
	}

	// Delete.
	if err := store.DeleteEntityReferencesForSession(ctx, sessID); err != nil {
		t.Fatalf("DeleteEntityReferencesForSession: %v", err)
	}

	// Verify deleted.
	got, _ = store.GetEntityReferences(ctx, entID, 50, 0)
	if len(got) != 0 {
		t.Fatalf("expected 0 references after delete, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Transcript Full-Text Search
// ---------------------------------------------------------------------------

func TestSearchTranscripts(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/fts")

	segments := []TranscriptSegment{
		{SessionID: sessID, UserID: "u1", StartTime: 0.0, EndTime: 3.5, Text: "The dragon swooped down from the mountain and attacked the village"},
		{SessionID: sessID, UserID: "u2", StartTime: 4.0, EndTime: 7.0, Text: "I draw my sword and charge towards the goblin horde"},
		{SessionID: sessID, UserID: "u1", StartTime: 8.0, EndTime: 12.0, Text: "The wizard casts a fireball at the dragon destroying it completely"},
	}

	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("InsertSegments: %v", err)
	}

	// Search for "dragon" — should match 2 segments.
	results, total, err := store.SearchTranscripts(ctx, campID, "dragon", 20, 0)
	if err != nil {
		t.Fatalf("SearchTranscripts dragon: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 results for 'dragon', got %d", total)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results returned, got %d", len(results))
	}

	// Check that headlines contain <mark> tags.
	for _, r := range results {
		if r.Headline == "" {
			t.Fatal("expected non-empty headline")
		}
		if r.SessionID != sessID {
			t.Fatalf("expected session_id %d, got %d", sessID, r.SessionID)
		}
	}

	// Search for "goblin" — should match 1 segment.
	results, total, err = store.SearchTranscripts(ctx, campID, "goblin", 20, 0)
	if err != nil {
		t.Fatalf("SearchTranscripts goblin: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 result for 'goblin', got %d", total)
	}
	if results[0].UserID != "u2" {
		t.Fatalf("expected user_id 'u2', got %q", results[0].UserID)
	}
}

func TestSearchTranscripts_EmptyQuery(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	results, total, err := store.SearchTranscripts(ctx, campID, "", 20, 0)
	if err != nil {
		t.Fatalf("SearchTranscripts empty: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected 0 total for empty query, got %d", total)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestSearchTranscripts_NoResults(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/fts-nr")

	segments := []TranscriptSegment{
		{SessionID: sessID, UserID: "u1", StartTime: 0.0, EndTime: 3.5, Text: "The party enters the tavern"},
	}
	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("InsertSegments: %v", err)
	}

	results, total, err := store.SearchTranscripts(ctx, campID, "spaceship", 20, 0)
	if err != nil {
		t.Fatalf("SearchTranscripts no results: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected 0 total for 'spaceship', got %d", total)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for 'spaceship', got %d", len(results))
	}
}

func TestSearchTranscripts_Pagination(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/fts-pg")

	// Insert many segments containing the keyword "adventure".
	segments := make([]TranscriptSegment, 5)
	for i := range segments {
		segments[i] = TranscriptSegment{
			SessionID: sessID,
			UserID:    "u1",
			StartTime: float64(i * 10),
			EndTime:   float64(i*10 + 5),
			Text:      fmt.Sprintf("The adventure continues in chapter %d of the great adventure", i+1),
		}
	}
	if err := store.InsertSegments(ctx, segments); err != nil {
		t.Fatalf("InsertSegments: %v", err)
	}

	// Page 1: limit 2, offset 0.
	results, total, err := store.SearchTranscripts(ctx, campID, "adventure", 2, 0)
	if err != nil {
		t.Fatalf("SearchTranscripts page 1: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results on page 1, got %d", len(results))
	}

	// Page 2: limit 2, offset 2.
	results2, total2, err := store.SearchTranscripts(ctx, campID, "adventure", 2, 2)
	if err != nil {
		t.Fatalf("SearchTranscripts page 2: %v", err)
	}
	if total2 != 5 {
		t.Fatalf("expected total still 5, got %d", total2)
	}
	if len(results2) != 2 {
		t.Fatalf("expected 2 results on page 2, got %d", len(results2))
	}

	// Page 3: limit 2, offset 4.
	results3, _, err := store.SearchTranscripts(ctx, campID, "adventure", 2, 4)
	if err != nil {
		t.Fatalf("SearchTranscripts page 3: %v", err)
	}
	if len(results3) != 1 {
		t.Fatalf("expected 1 result on page 3, got %d", len(results3))
	}
}

func TestSearchTranscripts_CampaignIsolation(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campA := createTestCampaign(t, store, guildID)
	campB, _ := store.CreateCampaign(ctx, guildID, "Campaign B", "")

	sessA, _ := store.CreateSession(ctx, guildID, campA, "ch-1", "/tmp/fts-a")
	sessB, _ := store.CreateSession(ctx, guildID, campB, "ch-1", "/tmp/fts-b")

	store.InsertSegments(ctx, []TranscriptSegment{
		{SessionID: sessA, UserID: "u1", StartTime: 0, EndTime: 5, Text: "The unicorn galloped across the meadow"},
	})
	store.InsertSegments(ctx, []TranscriptSegment{
		{SessionID: sessB, UserID: "u1", StartTime: 0, EndTime: 5, Text: "A unicorn appeared in the forest"},
	})

	// Search in campaign A only.
	results, total, err := store.SearchTranscripts(ctx, campA, "unicorn", 20, 0)
	if err != nil {
		t.Fatalf("SearchTranscripts campA: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 result in campaign A, got %d", total)
	}
	if results[0].SessionID != sessA {
		t.Fatalf("expected session_id %d, got %d", sessA, results[0].SessionID)
	}
}

// ---------------------------------------------------------------------------
// Entity Merge
// ---------------------------------------------------------------------------

func TestMergeEntities(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)
	sessID, _ := store.CreateSession(ctx, guildID, campID, "ch-1", "/tmp/merge")

	// Create two entities that represent the same thing.
	keepID, _ := store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire lord")
	mergeID, _ := store.UpsertEntity(ctx, campID, "Strahd von Zarovich", "npc", "Ancient vampire overlord of Barovia")

	// Create a third entity for relationship targets.
	thirdID, _ := store.UpsertEntity(ctx, campID, "Ireena", "npc", "Barovian noble")

	// Add notes to both entities.
	store.AddEntityNote(ctx, keepID, sessID, "Encountered in Castle Ravenloft")
	store.AddEntityNote(ctx, mergeID, sessID, "Revealed his full name")

	// Add relationships to both.
	store.UpsertEntityRelationship(ctx, campID, keepID, thirdID, "enemy_of", "Strahd hunts Ireena", nil)
	store.UpsertEntityRelationship(ctx, campID, mergeID, thirdID, "obsessed_with", "He wants to make her his bride", nil)

	// Add entity references.
	store.InsertSegments(ctx, []TranscriptSegment{
		{SessionID: sessID, UserID: "u1", StartTime: 0, EndTime: 5, Text: "Strahd appeared"},
		{SessionID: sessID, UserID: "u1", StartTime: 5, EndTime: 10, Text: "Strahd von Zarovich revealed himself"},
	})
	// Get the segment IDs.
	segments, _ := store.GetTranscript(ctx, sessID)
	seg1ID := segments[0].ID
	seg2ID := segments[1].ID

	store.InsertEntityReferences(ctx, []EntityReference{
		{EntityID: keepID, SessionID: sessID, SegmentID: &seg1ID, Context: "Strahd appeared"},
	})
	store.InsertEntityReferences(ctx, []EntityReference{
		{EntityID: mergeID, SessionID: sessID, SegmentID: &seg2ID, Context: "Strahd von Zarovich revealed himself"},
	})

	// Perform the merge.
	err := store.MergeEntities(ctx, campID, keepID, mergeID)
	if err != nil {
		t.Fatalf("MergeEntities: %v", err)
	}

	// Verify notes moved to keepID.
	notes, err := store.GetEntityNotes(ctx, keepID)
	if err != nil {
		t.Fatalf("GetEntityNotes after merge: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes on kept entity, got %d", len(notes))
	}

	// Verify references moved to keepID.
	refs, err := store.GetEntityReferences(ctx, keepID, 100, 0)
	if err != nil {
		t.Fatalf("GetEntityReferences after merge: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 references on kept entity, got %d", len(refs))
	}

	// Verify relationships moved to keepID.
	rels, err := store.GetEntityRelationships(ctx, keepID)
	if err != nil {
		t.Fatalf("GetEntityRelationships after merge: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("expected 2 relationships on kept entity, got %d", len(rels))
	}

	// Verify description was appended.
	kept, err := store.GetEntity(ctx, keepID)
	if err != nil {
		t.Fatalf("GetEntity after merge: %v", err)
	}
	if kept.Description != "Vampire lord\n\nAncient vampire overlord of Barovia" {
		t.Fatalf("expected merged description, got %q", kept.Description)
	}

	// Verify merged entity is deleted.
	_, err = store.GetEntity(ctx, mergeID)
	if err == nil {
		t.Fatal("expected error when getting deleted merged entity")
	}

	// Verify audit row.
	var auditCount int
	err = store.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entity_merges WHERE campaign_id = $1 AND kept_id = $2 AND merged_id = $3`,
		campID, keepID, mergeID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query entity_merges: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit row, got %d", auditCount)
	}
}

func TestMergeEntitiesConflictingRelationships(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	keepID, _ := store.UpsertEntity(ctx, campID, "Keep", "npc", "")
	mergeID, _ := store.UpsertEntity(ctx, campID, "Merge", "npc", "")
	thirdID, _ := store.UpsertEntity(ctx, campID, "Third", "npc", "")

	// Both entities have the same relationship type to the same target — a conflict.
	store.UpsertEntityRelationship(ctx, campID, keepID, thirdID, "ally", "Old friends", nil)
	store.UpsertEntityRelationship(ctx, campID, mergeID, thirdID, "ally", "New friends", nil)

	// Also test target-side conflicts.
	store.UpsertEntityRelationship(ctx, campID, thirdID, keepID, "serves", "Serves the keep entity", nil)
	store.UpsertEntityRelationship(ctx, campID, thirdID, mergeID, "serves", "Serves the merge entity", nil)

	err := store.MergeEntities(ctx, campID, keepID, mergeID)
	if err != nil {
		t.Fatalf("MergeEntities with conflicts: %v", err)
	}

	// Should have the kept entity's relationships without duplicates.
	rels, err := store.GetEntityRelationships(ctx, keepID)
	if err != nil {
		t.Fatalf("GetEntityRelationships after merge: %v", err)
	}

	// Should have exactly 2: keepID->thirdID (ally) and thirdID->keepID (serves).
	if len(rels) != 2 {
		t.Fatalf("expected 2 relationships (no duplicates), got %d", len(rels))
	}

	// Verify merged entity deleted.
	_, err = store.GetEntity(ctx, mergeID)
	if err == nil {
		t.Fatal("expected error when getting deleted merged entity")
	}

	// Verify audit row exists.
	var auditCount int
	err = store.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entity_merges WHERE campaign_id = $1 AND kept_id = $2 AND merged_id = $3`,
		campID, keepID, mergeID,
	).Scan(&auditCount)
	if err != nil {
		t.Fatalf("query entity_merges: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit row, got %d", auditCount)
	}
}

// ---------------------------------------------------------------------------
// Combat Encounters
// ---------------------------------------------------------------------------

func TestCombatEncounterCRUD(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	sessionID, err := store.CreateSession(ctx, guildID, campID, "chan-combat", "/tmp/audio")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Insert encounter.
	encID, err := store.InsertCombatEncounter(ctx, CombatEncounter{
		SessionID:  sessionID,
		CampaignID: campID,
		Name:       "Goblin Ambush",
		StartTime:  120.0,
		EndTime:    360.0,
		Summary:    "The party defeated the goblins.",
	})
	if err != nil {
		t.Fatalf("InsertCombatEncounter: %v", err)
	}
	if encID == 0 {
		t.Fatal("expected non-zero encounter id")
	}

	// Insert actions.
	dmg := 12
	round := 1
	ts := 130.0
	actions := []CombatAction{
		{Actor: "Thordak", ActionType: "attack", Target: "Goblin", Detail: "Swings greatsword", Damage: &dmg, Round: &round, Timestamp: &ts},
		{Actor: "Elara", ActionType: "spell", Target: "Goblin Boss", Detail: "Casts fireball", Damage: nil, Round: &round},
	}
	if err := store.InsertCombatActions(ctx, encID, actions); err != nil {
		t.Fatalf("InsertCombatActions: %v", err)
	}

	// Retrieve encounters.
	encounters, err := store.GetCombatEncounters(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCombatEncounters: %v", err)
	}
	if len(encounters) != 1 {
		t.Fatalf("expected 1 encounter, got %d", len(encounters))
	}
	if encounters[0].Name != "Goblin Ambush" {
		t.Fatalf("expected name 'Goblin Ambush', got %q", encounters[0].Name)
	}
	if encounters[0].StartTime != 120.0 {
		t.Fatalf("expected start_time 120.0, got %f", encounters[0].StartTime)
	}

	// Retrieve actions.
	got, err := store.GetCombatActions(ctx, encID)
	if err != nil {
		t.Fatalf("GetCombatActions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(got))
	}
	if got[0].Actor != "Thordak" {
		t.Fatalf("expected actor 'Thordak', got %q", got[0].Actor)
	}
	if got[0].Damage == nil || *got[0].Damage != 12 {
		t.Fatal("expected damage 12 for first action")
	}
	if got[1].Actor != "Elara" {
		t.Fatalf("expected actor 'Elara', got %q", got[1].Actor)
	}
}

func TestDeleteCombatForSession(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	sessionID, err := store.CreateSession(ctx, guildID, campID, "chan-combat-del", "/tmp/audio")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	encID, err := store.InsertCombatEncounter(ctx, CombatEncounter{
		SessionID:  sessionID,
		CampaignID: campID,
		Name:       "Orc Raid",
		StartTime:  60.0,
		EndTime:    180.0,
		Summary:    "The orcs were defeated.",
	})
	if err != nil {
		t.Fatalf("InsertCombatEncounter: %v", err)
	}
	dmg := 8
	if err := store.InsertCombatActions(ctx, encID, []CombatAction{
		{Actor: "Grimjaw", ActionType: "attack", Target: "Orc", Detail: "Axe swing", Damage: &dmg},
	}); err != nil {
		t.Fatalf("InsertCombatActions: %v", err)
	}

	// Delete all combat for session.
	if err := store.DeleteCombatForSession(ctx, sessionID); err != nil {
		t.Fatalf("DeleteCombatForSession: %v", err)
	}

	// Verify encounters are gone.
	encounters, err := store.GetCombatEncounters(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetCombatEncounters after delete: %v", err)
	}
	if len(encounters) != 0 {
		t.Fatalf("expected 0 encounters after delete, got %d", len(encounters))
	}

	// Verify actions are cascade-deleted.
	actions, err := store.GetCombatActions(ctx, encID)
	if err != nil {
		t.Fatalf("GetCombatActions after delete: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions after delete, got %d", len(actions))
	}
}

// ---------------------------------------------------------------------------
// Relationship Graph
// ---------------------------------------------------------------------------

func TestGetCampaignRelationshipGraph(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Create entities.
	id1, err := store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire lord")
	if err != nil {
		t.Fatalf("UpsertEntity Strahd: %v", err)
	}
	id2, err := store.UpsertEntity(ctx, campID, "Ireena", "npc", "Burgomasters daughter")
	if err != nil {
		t.Fatalf("UpsertEntity Ireena: %v", err)
	}
	id3, err := store.UpsertEntity(ctx, campID, "Barovia", "place", "Cursed land")
	if err != nil {
		t.Fatalf("UpsertEntity Barovia: %v", err)
	}

	// Create relationships.
	if err := store.UpsertEntityRelationship(ctx, campID, id1, id2, "obsessed_with", "Strahd seeks Ireena", nil); err != nil {
		t.Fatalf("UpsertEntityRelationship 1->2: %v", err)
	}
	if err := store.UpsertEntityRelationship(ctx, campID, id1, id3, "rules", "Strahd rules Barovia", nil); err != nil {
		t.Fatalf("UpsertEntityRelationship 1->3: %v", err)
	}

	// Fetch graph.
	entities, rels, err := store.GetCampaignRelationshipGraph(ctx, campID)
	if err != nil {
		t.Fatalf("GetCampaignRelationshipGraph: %v", err)
	}

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}
	if len(rels) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(rels))
	}

	// Entities should be ordered by name.
	if entities[0].Name != "Barovia" {
		t.Fatalf("expected first entity 'Barovia', got %q", entities[0].Name)
	}
	if entities[1].Name != "Ireena" {
		t.Fatalf("expected second entity 'Ireena', got %q", entities[1].Name)
	}
	if entities[2].Name != "Strahd" {
		t.Fatalf("expected third entity 'Strahd', got %q", entities[2].Name)
	}

	// Relationships should be ordered by created_at.
	if rels[0].Relationship != "obsessed_with" {
		t.Fatalf("expected first relationship 'obsessed_with', got %q", rels[0].Relationship)
	}
	if rels[1].Relationship != "rules" {
		t.Fatalf("expected second relationship 'rules', got %q", rels[1].Relationship)
	}

	// Verify IDs.
	if rels[0].SourceID != id1 || rels[0].TargetID != id2 {
		t.Fatalf("expected relationship source=%d target=%d, got source=%d target=%d", id1, id2, rels[0].SourceID, rels[0].TargetID)
	}

	// Verify a campaign with no data returns empty slices (no error).
	campID2, err := store.CreateCampaign(ctx, guildID, "Empty Campaign", "")
	if err != nil {
		t.Fatalf("CreateCampaign empty: %v", err)
	}
	entities2, rels2, err := store.GetCampaignRelationshipGraph(ctx, campID2)
	if err != nil {
		t.Fatalf("GetCampaignRelationshipGraph empty: %v", err)
	}
	if len(entities2) != 0 {
		t.Fatalf("expected 0 entities for empty campaign, got %d", len(entities2))
	}
	if len(rels2) != 0 {
		t.Fatalf("expected 0 relationships for empty campaign, got %d", len(rels2))
	}
}

// ---------------------------------------------------------------------------
// Shared Mics
// ---------------------------------------------------------------------------

func TestSetSharedMic(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Insert a new shared mic.
	if err := store.SetSharedMic(ctx, campID, "discord-user-1", "partner-user-1"); err != nil {
		t.Fatalf("SetSharedMic insert: %v", err)
	}

	mics, err := store.GetSharedMics(ctx, campID)
	if err != nil {
		t.Fatalf("GetSharedMics after insert: %v", err)
	}
	if len(mics) != 1 {
		t.Fatalf("expected 1 mic, got %d", len(mics))
	}
	if mics[0].DiscordUserID != "discord-user-1" {
		t.Fatalf("expected discord_user_id 'discord-user-1', got %q", mics[0].DiscordUserID)
	}
	if mics[0].PartnerUserID != "partner-user-1" {
		t.Fatalf("expected partner_user_id 'partner-user-1', got %q", mics[0].PartnerUserID)
	}
	if mics[0].CampaignID != campID {
		t.Fatalf("expected campaign_id %d, got %d", campID, mics[0].CampaignID)
	}

	// Upsert (update) the same discord user with a new partner.
	if err := store.SetSharedMic(ctx, campID, "discord-user-1", "partner-user-2"); err != nil {
		t.Fatalf("SetSharedMic upsert: %v", err)
	}

	mics, err = store.GetSharedMics(ctx, campID)
	if err != nil {
		t.Fatalf("GetSharedMics after upsert: %v", err)
	}
	if len(mics) != 1 {
		t.Fatalf("expected 1 mic after upsert, got %d", len(mics))
	}
	if mics[0].PartnerUserID != "partner-user-2" {
		t.Fatalf("expected partner_user_id 'partner-user-2' after upsert, got %q", mics[0].PartnerUserID)
	}
}

func TestGetSharedMics(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Empty campaign should return empty slice.
	mics, err := store.GetSharedMics(ctx, campID)
	if err != nil {
		t.Fatalf("GetSharedMics empty: %v", err)
	}
	if len(mics) != 0 {
		t.Fatalf("expected 0 mics for empty campaign, got %d", len(mics))
	}

	// Add multiple mics.
	if err := store.SetSharedMic(ctx, campID, "user-a", "partner-a"); err != nil {
		t.Fatalf("SetSharedMic A: %v", err)
	}
	if err := store.SetSharedMic(ctx, campID, "user-b", "partner-b"); err != nil {
		t.Fatalf("SetSharedMic B: %v", err)
	}

	mics, err = store.GetSharedMics(ctx, campID)
	if err != nil {
		t.Fatalf("GetSharedMics: %v", err)
	}
	if len(mics) != 2 {
		t.Fatalf("expected 2 mics, got %d", len(mics))
	}

	// Mics for a different campaign should not appear.
	campID2 := createTestCampaign(t, store, guildID)
	mics2, err := store.GetSharedMics(ctx, campID2)
	if err != nil {
		t.Fatalf("GetSharedMics other campaign: %v", err)
	}
	if len(mics2) != 0 {
		t.Fatalf("expected 0 mics for other campaign, got %d", len(mics2))
	}
}

func TestDeleteSharedMic(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Set up a mic to delete.
	if err := store.SetSharedMic(ctx, campID, "user-del", "partner-del"); err != nil {
		t.Fatalf("SetSharedMic: %v", err)
	}

	mics, _ := store.GetSharedMics(ctx, campID)
	if len(mics) != 1 {
		t.Fatalf("expected 1 mic before delete, got %d", len(mics))
	}

	// Delete it.
	if err := store.DeleteSharedMic(ctx, campID, "user-del"); err != nil {
		t.Fatalf("DeleteSharedMic: %v", err)
	}

	mics, err := store.GetSharedMics(ctx, campID)
	if err != nil {
		t.Fatalf("GetSharedMics after delete: %v", err)
	}
	if len(mics) != 0 {
		t.Fatalf("expected 0 mics after delete, got %d", len(mics))
	}

	// Deleting a non-existent mic should not error.
	if err := store.DeleteSharedMic(ctx, campID, "nonexistent"); err != nil {
		t.Fatalf("DeleteSharedMic nonexistent: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Speaker Enrollments
// ---------------------------------------------------------------------------

func TestUpsertSpeakerEnrollment(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	embedding1 := []float32{0.1, 0.2, 0.3, 0.4}

	// Insert.
	if err := store.UpsertSpeakerEnrollment(ctx, campID, "speaker-1", embedding1); err != nil {
		t.Fatalf("UpsertSpeakerEnrollment insert: %v", err)
	}

	got, err := store.GetSpeakerEnrollment(ctx, campID, "speaker-1")
	if err != nil {
		t.Fatalf("GetSpeakerEnrollment after insert: %v", err)
	}
	if got.UserID != "speaker-1" {
		t.Fatalf("expected user_id 'speaker-1', got %q", got.UserID)
	}
	if got.CampaignID != campID {
		t.Fatalf("expected campaign_id %d, got %d", campID, got.CampaignID)
	}
	if len(got.Embedding) != 4 {
		t.Fatalf("expected embedding length 4, got %d", len(got.Embedding))
	}
	if got.Embedding[0] != 0.1 || got.Embedding[3] != 0.4 {
		t.Fatalf("unexpected embedding values: %v", got.Embedding)
	}

	// Update (upsert with new embedding).
	embedding2 := []float32{0.5, 0.6, 0.7, 0.8}
	if err := store.UpsertSpeakerEnrollment(ctx, campID, "speaker-1", embedding2); err != nil {
		t.Fatalf("UpsertSpeakerEnrollment update: %v", err)
	}

	got, err = store.GetSpeakerEnrollment(ctx, campID, "speaker-1")
	if err != nil {
		t.Fatalf("GetSpeakerEnrollment after update: %v", err)
	}
	if got.Embedding[0] != 0.5 || got.Embedding[3] != 0.8 {
		t.Fatalf("expected updated embedding, got %v", got.Embedding)
	}
}

func TestGetSpeakerEnrollment(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Not found case: should return error.
	_, err := store.GetSpeakerEnrollment(ctx, campID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent enrollment, got nil")
	}

	// Found case.
	embedding := []float32{1.0, 2.0, 3.0}
	if err := store.UpsertSpeakerEnrollment(ctx, campID, "found-user", embedding); err != nil {
		t.Fatalf("UpsertSpeakerEnrollment: %v", err)
	}

	got, err := store.GetSpeakerEnrollment(ctx, campID, "found-user")
	if err != nil {
		t.Fatalf("GetSpeakerEnrollment found: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil enrollment")
	}
	if got.UserID != "found-user" {
		t.Fatalf("expected user_id 'found-user', got %q", got.UserID)
	}
	if len(got.Embedding) != 3 {
		t.Fatalf("expected embedding length 3, got %d", len(got.Embedding))
	}
}

func TestGetSpeakerEnrollments(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Empty campaign should return empty slice.
	enrollments, err := store.GetSpeakerEnrollments(ctx, campID)
	if err != nil {
		t.Fatalf("GetSpeakerEnrollments empty: %v", err)
	}
	if len(enrollments) != 0 {
		t.Fatalf("expected 0 enrollments, got %d", len(enrollments))
	}

	// Add multiple enrollments.
	if err := store.UpsertSpeakerEnrollment(ctx, campID, "user-x", []float32{0.1, 0.2}); err != nil {
		t.Fatalf("UpsertSpeakerEnrollment X: %v", err)
	}
	if err := store.UpsertSpeakerEnrollment(ctx, campID, "user-y", []float32{0.3, 0.4}); err != nil {
		t.Fatalf("UpsertSpeakerEnrollment Y: %v", err)
	}

	enrollments, err = store.GetSpeakerEnrollments(ctx, campID)
	if err != nil {
		t.Fatalf("GetSpeakerEnrollments: %v", err)
	}
	if len(enrollments) != 2 {
		t.Fatalf("expected 2 enrollments, got %d", len(enrollments))
	}

	// Enrollments for a different campaign should not appear.
	campID2 := createTestCampaign(t, store, guildID)
	enrollments2, err := store.GetSpeakerEnrollments(ctx, campID2)
	if err != nil {
		t.Fatalf("GetSpeakerEnrollments other campaign: %v", err)
	}
	if len(enrollments2) != 0 {
		t.Fatalf("expected 0 enrollments for other campaign, got %d", len(enrollments2))
	}
}

// ---------------------------------------------------------------------------
// Entity status
// ---------------------------------------------------------------------------

func TestUpdateEntityStatus(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Create an entity — default status should be 'unknown'.
	id, err := store.UpsertEntity(ctx, campID, "Lord Soth", "npc", "Death knight")
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}

	e, err := store.GetEntity(ctx, id)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if e.Status != "unknown" {
		t.Fatalf("expected default status 'unknown', got %q", e.Status)
	}
	if e.CauseOfDeath != "" {
		t.Fatalf("expected empty cause_of_death, got %q", e.CauseOfDeath)
	}

	// Update to alive.
	if err := store.UpdateEntityStatus(ctx, id, "alive", ""); err != nil {
		t.Fatalf("UpdateEntityStatus alive: %v", err)
	}
	e, _ = store.GetEntity(ctx, id)
	if e.Status != "alive" {
		t.Fatalf("expected status 'alive', got %q", e.Status)
	}

	// Update to dead with cause.
	if err := store.UpdateEntityStatus(ctx, id, "dead", "Slain by the party in combat"); err != nil {
		t.Fatalf("UpdateEntityStatus dead: %v", err)
	}
	e, _ = store.GetEntity(ctx, id)
	if e.Status != "dead" {
		t.Fatalf("expected status 'dead', got %q", e.Status)
	}
	if e.CauseOfDeath != "Slain by the party in combat" {
		t.Fatalf("expected cause_of_death 'Slain by the party in combat', got %q", e.CauseOfDeath)
	}

	// UpsertEntity should NOT reset status back to 'unknown'.
	store.UpsertEntity(ctx, campID, "Lord Soth", "npc", "Death knight lord")
	e, _ = store.GetEntity(ctx, id)
	if e.Status != "dead" {
		t.Fatalf("expected status preserved after upsert, got %q", e.Status)
	}
	if e.CauseOfDeath != "Slain by the party in combat" {
		t.Fatalf("expected cause_of_death preserved after upsert, got %q", e.CauseOfDeath)
	}
}

func TestListEntities_StatusFilter(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	id1, _ := store.UpsertEntity(ctx, campID, "Living Knight", "npc", "A brave knight")
	store.UpdateEntityStatus(ctx, id1, "alive", "")

	id2, _ := store.UpsertEntity(ctx, campID, "Dead Dragon", "npc", "Ancient dragon")
	store.UpdateEntityStatus(ctx, id2, "dead", "Killed by adventurers")

	store.UpsertEntity(ctx, campID, "Mysterious Figure", "npc", "Unknown stranger")

	// Filter by alive.
	alive, err := store.ListEntities(ctx, campID, "", "", 50, 0, "alive")
	if err != nil {
		t.Fatalf("ListEntities alive: %v", err)
	}
	if len(alive) != 1 {
		t.Fatalf("expected 1 alive entity, got %d", len(alive))
	}
	if alive[0].Name != "Living Knight" {
		t.Fatalf("expected 'Living Knight', got %q", alive[0].Name)
	}

	// Filter by dead.
	dead, err := store.ListEntities(ctx, campID, "", "", 50, 0, "dead")
	if err != nil {
		t.Fatalf("ListEntities dead: %v", err)
	}
	if len(dead) != 1 {
		t.Fatalf("expected 1 dead entity, got %d", len(dead))
	}
	if dead[0].Name != "Dead Dragon" {
		t.Fatalf("expected 'Dead Dragon', got %q", dead[0].Name)
	}

	// No filter returns all.
	all, err := store.ListEntities(ctx, campID, "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListEntities all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// Location Hierarchy
// ---------------------------------------------------------------------------

func TestSetEntityParent(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	parentID, err := store.UpsertEntity(ctx, campID, "Barovia", "place", "A dark land")
	if err != nil {
		t.Fatalf("UpsertEntity parent: %v", err)
	}
	childID, err := store.UpsertEntity(ctx, campID, "Village of Barovia", "place", "A small village")
	if err != nil {
		t.Fatalf("UpsertEntity child: %v", err)
	}

	// Set parent.
	if err := store.SetEntityParent(ctx, childID, parentID); err != nil {
		t.Fatalf("SetEntityParent: %v", err)
	}

	// Verify.
	child, err := store.GetEntity(ctx, childID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if child.ParentEntityID == nil {
		t.Fatal("expected non-nil parent_entity_id")
	}
	if *child.ParentEntityID != parentID {
		t.Fatalf("expected parent_entity_id %d, got %d", parentID, *child.ParentEntityID)
	}
}

func TestGetLocationHierarchy(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	regionID, _ := store.UpsertEntity(ctx, campID, "Barovia", "place", "A dark land")
	villageID, _ := store.UpsertEntity(ctx, campID, "Village of Barovia", "place", "A small village")
	tavernID, _ := store.UpsertEntity(ctx, campID, "Blood on the Vine Tavern", "place", "A tavern")

	// Also create an NPC — should NOT appear in the hierarchy.
	store.UpsertEntity(ctx, campID, "Strahd", "npc", "Vampire lord")

	store.SetEntityParent(ctx, villageID, regionID)
	store.SetEntityParent(ctx, tavernID, villageID)

	places, err := store.GetLocationHierarchy(ctx, campID)
	if err != nil {
		t.Fatalf("GetLocationHierarchy: %v", err)
	}

	if len(places) != 3 {
		t.Fatalf("expected 3 place entities, got %d", len(places))
	}

	// Verify parent relationships.
	parentMap := make(map[string]*int64)
	for _, p := range places {
		parentMap[p.Name] = p.ParentEntityID
	}

	if parentMap["Barovia"] != nil {
		t.Fatal("expected Barovia to have no parent")
	}
	if parentMap["Village of Barovia"] == nil || *parentMap["Village of Barovia"] != regionID {
		t.Fatalf("expected Village of Barovia parent to be %d", regionID)
	}
	if parentMap["Blood on the Vine Tavern"] == nil || *parentMap["Blood on the Vine Tavern"] != villageID {
		t.Fatalf("expected Blood on the Vine Tavern parent to be %d", villageID)
	}
}

func TestMergeEntities_ReparentsChildren(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	// Parent that will be merged away.
	mergeID, _ := store.UpsertEntity(ctx, campID, "Barovia Region", "place", "The region")
	// Parent that will be kept.
	keepID, _ := store.UpsertEntity(ctx, campID, "Barovia", "place", "The dark domain")
	// Child pointing to the entity that will be merged.
	childID, _ := store.UpsertEntity(ctx, campID, "Village of Barovia", "place", "Village")
	store.SetEntityParent(ctx, childID, mergeID)

	// Merge: mergeID into keepID.
	if err := store.MergeEntities(ctx, campID, keepID, mergeID); err != nil {
		t.Fatalf("MergeEntities: %v", err)
	}

	// Verify child is now parented under the kept entity.
	child, err := store.GetEntity(ctx, childID)
	if err != nil {
		t.Fatalf("GetEntity child: %v", err)
	}
	if child.ParentEntityID == nil {
		t.Fatal("expected child to still have a parent after merge")
	}
	if *child.ParentEntityID != keepID {
		t.Fatalf("expected child parent to be %d (kept), got %d", keepID, *child.ParentEntityID)
	}
}

func TestGetChildEntities(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	guildID := uniqueGuild(t)
	campID := createTestCampaign(t, store, guildID)

	parentID, _ := store.UpsertEntity(ctx, campID, "Barovia", "place", "A dark land")
	child1ID, _ := store.UpsertEntity(ctx, campID, "Village of Barovia", "place", "A village")
	child2ID, _ := store.UpsertEntity(ctx, campID, "Castle Ravenloft", "place", "A castle")
	store.UpsertEntity(ctx, campID, "Unrelated Place", "place", "Somewhere else")

	store.SetEntityParent(ctx, child1ID, parentID)
	store.SetEntityParent(ctx, child2ID, parentID)

	children, err := store.GetChildEntities(ctx, parentID)
	if err != nil {
		t.Fatalf("GetChildEntities: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	names := make(map[string]bool)
	for _, c := range children {
		names[c.Name] = true
	}
	if !names["Village of Barovia"] || !names["Castle Ravenloft"] {
		t.Fatalf("expected children to be 'Village of Barovia' and 'Castle Ravenloft', got %v", names)
	}
}
