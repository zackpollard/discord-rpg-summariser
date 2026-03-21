package pdf

import (
	"testing"
	"time"
)

func TestGenerate_ValidPDF(t *testing.T) {
	now := time.Now()
	endedAt := now.Add(2 * time.Hour)

	book := &CampaignBook{
		Campaign: Campaign{
			Name:        "The Lost Mines of Phandelver",
			Description: "A classic adventure for brave heroes.",
			CreatedAt:   now.Add(-30 * 24 * time.Hour),
		},
		Recap: "The adventurers set out from Neverwinter, hired by the dwarf Gundren Rockseeker to escort a wagon of provisions to the rough-and-tumble settlement of Phandalin.\n\nAlong the way, they were ambushed by goblins on the Triboar Trail. After defeating the creatures, they discovered that Gundren and his bodyguard Sildar Hallwinter had been captured.",
		Sessions: []Session{
			{
				ID:        1,
				StartedAt: now.Add(-14 * 24 * time.Hour),
				EndedAt:   &endedAt,
				Summary:   "The party traveled from Neverwinter along the Triboar Trail. They encountered a goblin ambush and fought bravely. After the battle, they discovered their patron Gundren had been kidnapped.",
				KeyEvents: []string{
					"Party departed Neverwinter",
					"Goblin ambush on Triboar Trail",
					"Discovered Gundren's kidnapping",
				},
			},
			{
				ID:        2,
				StartedAt: now.Add(-7 * 24 * time.Hour),
				Summary:   "The party explored the Cragmaw Hideout, rescued Sildar Hallwinter, and learned that Gundren was taken to Cragmaw Castle.",
				KeyEvents: []string{
					"Explored Cragmaw Hideout",
					"Rescued Sildar Hallwinter",
					"Learned about Cragmaw Castle",
				},
			},
		},
		Entities: []Entity{
			{Name: "Gundren Rockseeker", Type: "npc", Description: "A dwarf entrepreneur who hired the party.", Status: "alive"},
			{Name: "Sildar Hallwinter", Type: "npc", Description: "A human warrior and member of the Lords' Alliance.", Status: "alive"},
			{Name: "Klarg", Type: "npc", Description: "A bugbear leader of the Cragmaw goblins.", Status: "dead", CauseOfDeath: "Slain by the party"},
			{Name: "Phandalin", Type: "place", Description: "A small frontier town."},
			{Name: "Cragmaw Hideout", Type: "place", Description: "A cave system used as a goblin base."},
			{Name: "Cragmaw Tribe", Type: "organisation", Description: "A tribe of goblins and bugbears."},
			{Name: "Wave Echo Cave", Type: "place", Description: "A legendary mine containing a magical forge."},
			{Name: "Glass Staff", Type: "item", Description: "A magical staff made of glass."},
		},
		Quests: []Quest{
			{Name: "Deliver the Wagon", Description: "Escort Gundren's wagon of supplies to Barthen's Provisions in Phandalin.", Status: "completed", Giver: "Gundren Rockseeker"},
			{Name: "Find Gundren", Description: "Rescue Gundren Rockseeker from the Cragmaw goblins.", Status: "active", Giver: "Sildar Hallwinter"},
			{Name: "Find Wave Echo Cave", Description: "Locate the entrance to the lost mine.", Status: "active", Giver: "Gundren Rockseeker"},
		},
		Stats: &CampaignStats{
			TotalSessions:   2,
			TotalDurationMin: 240,
			AvgDurationMin:  120,
			TotalWords:      15000,
			TotalQuests:     3,
			ActiveQuests:    2,
			CompletedQuests: 1,
			FailedQuests:    0,
			TotalEncounters: 3,
			TotalDamage:     150,
			EntityCounts:    map[string]int{"npc": 3, "place": 3, "organisation": 1, "item": 1},
			NPCStatusCounts: map[string]int{"alive": 2, "dead": 1},
		},
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Verify PDF header.
	if len(data) < 5 {
		t.Fatalf("PDF too small: %d bytes", len(data))
	}
	header := string(data[:5])
	if header != "%PDF-" {
		t.Errorf("expected PDF header %%PDF-, got %q", header)
	}

	// Verify reasonable file size (should be at least a few KB).
	if len(data) < 1000 {
		t.Errorf("PDF suspiciously small: %d bytes", len(data))
	}

	t.Logf("Generated PDF: %d bytes", len(data))
}

func TestGenerate_EmptyBook(t *testing.T) {
	book := &CampaignBook{
		Campaign: Campaign{
			Name: "Empty Campaign",
		},
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Error("expected valid PDF output for empty book")
	}
}

func TestGenerate_MissingSections(t *testing.T) {
	// Test with only a recap (no sessions, entities, quests, stats).
	book := &CampaignBook{
		Campaign: Campaign{
			Name: "Recap Only Campaign",
		},
		Recap: "This is the story so far. The heroes journeyed across the land.",
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Error("expected valid PDF output for recap-only book")
	}
}

func TestSplitParagraphs(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"Hello\n\nWorld", 2},
		{"Single paragraph", 1},
		{"", 0},
		{"A\n\nB\n\nC", 3},
		{"\n\n\n", 0},
	}

	for _, tt := range tests {
		got := splitParagraphs(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitParagraphs(%q) returned %d paragraphs, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{30, "30m"},
		{60, "1h"},
		{90, "1h 30m"},
		{150, "2h 30m"},
		{0, "0m"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{500, "500"},
		{1500, "1.5k"},
		{15000, "15.0k"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
