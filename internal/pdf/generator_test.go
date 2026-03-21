package pdf

import (
	"strings"
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
			TotalSessions:    2,
			TotalDurationMin: 240,
			AvgDurationMin:   120,
			TotalWords:       15000,
			TotalQuests:      3,
			ActiveQuests:     2,
			CompletedQuests:  1,
			FailedQuests:     0,
			TotalEncounters:  3,
			TotalDamage:      150,
			EntityCounts:     map[string]int{"npc": 3, "place": 3, "organisation": 1, "item": 1},
			NPCStatusCounts:  map[string]int{"alive": 2, "dead": 1},
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

// ---- sanitizeText tests ----

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "em dash",
			input: "The hero\u2014a brave warrior\u2014charged forward",
			want:  "The hero--a brave warrior--charged forward",
		},
		{
			name:  "en dash",
			input: "pages 10\u201320",
			want:  "pages 10-20",
		},
		{
			name:  "curly double quotes",
			input: "\u201CHello,\u201D said the wizard",
			want:  "\"Hello,\" said the wizard",
		},
		{
			name:  "curly single quotes / apostrophes",
			input: "The dragon\u2019s lair was Theron\u2019s destination",
			want:  "The dragon's lair was Theron's destination",
		},
		{
			name:  "left single quote",
			input: "\u2018Twas the night",
			want:  "'Twas the night",
		},
		{
			name:  "bullet point",
			input: "\u2022 First item\n\u2022 Second item",
			want:  "* First item\n* Second item",
		},
		{
			name:  "ellipsis",
			input: "And then\u2026 silence",
			want:  "And then... silence",
		},
		{
			name:  "non-breaking space",
			input: "100\u00A0gold pieces",
			want:  "100 gold pieces",
		},
		{
			name:  "trademark and copyright",
			input: "D&D\u2122 and \u00A9 Wizards",
			want:  "D&D(TM) and (C) Wizards",
		},
		{
			name:  "arrows",
			input: "go \u2192 north \u2190 south",
			want:  "go -> north <- south",
		},
		{
			name:  "math symbols",
			input: "DC \u2265 15 and HP \u2260 0",
			want:  "DC >= 15 and HP != 0",
		},
		{
			name:  "BOM stripped",
			input: "\uFEFFHello world",
			want:  "Hello world",
		},
		{
			name:  "plain ASCII unchanged",
			input: "Just normal text with numbers 123 and symbols !@#",
			want:  "Just normal text with numbers 123 and symbols !@#",
		},
		{
			name:  "mixed unicode D&D text",
			input: "The wizard\u2019s \u201Cfireball\u201D dealt 8d6 \u00D7 2 damage\u2014devastating the ogre\u2019s camp\u2026",
			want:  "The wizard's \"fireball\" dealt 8d6 x 2 damage--devastating the ogre's camp...",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeText(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeText(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---- Comprehensive e2e test ----

func TestGeneratePDF_Comprehensive(t *testing.T) {
	now := time.Now()
	sess1End := now.Add(-60*24*time.Hour + 3*time.Hour + 30*time.Minute)
	sess2End := now.Add(-53*24*time.Hour + 2*time.Hour + 45*time.Minute)
	sess3End := now.Add(-46*24*time.Hour + 4*time.Hour)

	book := &CampaignBook{
		Campaign: Campaign{
			Name:        "Curse of the \u201CBloodmoon\u201D Prophecy",
			Description: "A dark tale of heroes facing an ancient evil\u2014where shadows whisper and the moon bleeds\u2026",
			CreatedAt:   now.Add(-90 * 24 * time.Hour),
		},
		Recap: "The adventurers arrived in the cursed town of Barovia\u2014a place shrouded in perpetual mist. " +
			"Their leader, Ser Aldric, bore the mark of the Bloodmoon\u2019s chosen.\n\n" +
			"In the weeks that followed, they uncovered the truth behind the vampire lord\u2019s dominion. " +
			"The \u201CBloodmoon Prophecy\u201D foretold the return of an ancient evil\u2026 one that would consume all light.\n\n" +
			"Now, with allies gathered from across the land\u2014dwarves from Ironforge, elves from the Silverwood, " +
			"and halflings from the Greenshire\u2014they prepare for the final confrontation. " +
			"The stakes could not be higher: fail, and darkness covers the world forever.\n\n" +
			"Each hero carries their own burden. Theren\u2019s guilt over his fallen brother drives him. " +
			"Miri\u2019s thirst for vengeance against the vampire who destroyed her village fuels her blade. " +
			"And old Beldric\u2019s ancient knowledge may be the key to unlocking the prophecy\u2019s true meaning.",
		Sessions: []Session{
			{
				ID:        1,
				StartedAt: now.Add(-60 * 24 * time.Hour),
				EndedAt:   &sess1End,
				Summary: "The party arrived in Barovia through a mysterious fog. They found the village deserted " +
					"except for a few frightened souls. At the Blood on the Vine tavern, they met Ismark, " +
					"who begged them to help his sister Ireena escape the vampire lord\u2019s attention. " +
					"The group agreed and set out toward the burgomaster\u2019s mansion\u2014only to find it " +
					"under siege by undead. They fought through waves of zombies and skeletons, " +
					"finally reaching Ireena who was frightened but unharmed.",
				KeyEvents: []string{
					"Arrived in Barovia through the mists",
					"Met Ismark at the Blood on the Vine tavern",
					"Fought through undead besieging the burgomaster\u2019s mansion",
					"Rescued Ireena from the vampire lord\u2019s minions",
				},
			},
			{
				ID:        2,
				StartedAt: now.Add(-53 * 24 * time.Hour),
				EndedAt:   &sess2End,
				Summary: "Escorting Ireena to the fortified town of Vallaki, the group encountered " +
					"a pack of dire wolves on the Svalich Road. Theren\u2019s divine smite proved " +
					"invaluable against the fiends. In Vallaki, they discovered the town\u2019s " +
					"burgomaster was a paranoid tyrant who forced the citizens to attend \u201Cfestivals of joy\u201D " +
					"under threat of punishment. The party also found the Tome of Strahd " +
					"hidden in the burgomaster\u2019s attic\u2014an ancient journal detailing the " +
					"vampire lord\u2019s tragic past.",
				KeyEvents: []string{
					"Dire wolf ambush on Svalich Road",
					"Arrived in Vallaki and met the mad burgomaster",
					"Discovered the Tome of Strahd\u2014a key artifact",
					"Learned about the \u201CSunsword\u201D and \u201CHoly Symbol of Ravenkind\u201D",
				},
			},
			{
				ID:        3,
				StartedAt: now.Add(-46 * 24 * time.Hour),
				EndedAt:   &sess3End,
				Summary: "The party ventured to the Amber Temple, a place of dark power high in the mountains. " +
					"Inside, they faced corrupted guardians and tempting dark vestiges. Miri nearly " +
					"succumbed to a vestige\u2019s offer of power, but Beldric\u2019s intervention saved her. " +
					"They recovered the Sunsword\u2014a sentient blade of radiant energy\u2014and learned " +
					"that Strahd\u2019s power was tied to three ancient fanes scattered across Barovia. " +
					"Destroying the fanes would weaken the vampire lord enough to defeat him. " +
					"The session ended with a dramatic confrontation with a flameskull guardian " +
					"that nearly killed Theren before Miri\u2019s quick thinking saved him.",
				KeyEvents: []string{
					"Explored the Amber Temple",
					"Miri resisted a dark vestige\u2019s temptation",
					"Recovered the Sunsword",
					"Learned about the three ancient fanes",
					"Nearly lost Theren to the flameskull guardian",
				},
			},
		},
		Entities: []Entity{
			// PCs
			{Name: "Theren Brightblade", Type: "pc", Description: "A human paladin of the Morninglord, driven by guilt over his brother\u2019s death. Wields the Sunsword."},
			{Name: "Miri Shadowstep", Type: "pc", Description: "A halfling rogue with a vendetta against the undead. Quick-witted and deadly with her twin daggers."},
			{Name: "Beldric the Wise", Type: "pc", Description: "An elderly human wizard specialising in abjuration. His knowledge of ancient lore is unmatched."},
			// NPCs
			{Name: "Strahd von Zarovich", Type: "npc", Description: "The ancient vampire lord who rules over Barovia\u2014a tragic figure bound by his own dark choices.", Status: "alive"},
			{Name: "Ireena Kolyana", Type: "npc", Description: "A brave young woman and the reincarnation of Strahd\u2019s lost love, Tatyana.", Status: "alive"},
			{Name: "Ismark the Lesser", Type: "npc", Description: "Ireena\u2019s brother and the burgomaster\u2019s son. A capable but overwhelmed warrior.", Status: "alive"},
			{Name: "Madam Eva", Type: "npc", Description: "A mysterious Vistani fortune-teller who guides the heroes through cryptic card readings.", Status: "alive"},
			{Name: "Baron Vallakovich", Type: "npc", Description: "The paranoid burgomaster of Vallaki who enforces mandatory \u201Cfestivals of joy.\u201D", Status: "dead", CauseOfDeath: "Overthrown and killed during the festival riots"},
			// Places
			{Name: "Village of Barovia", Type: "place", Description: "A fog-shrouded village at the base of Castle Ravenloft\u2014haunted by perpetual gloom."},
			{Name: "Vallaki", Type: "place", Description: "A fortified town ruled by a paranoid burgomaster. The citizens live in fear."},
			{Name: "Castle Ravenloft", Type: "place", Description: "Strahd\u2019s ancient fortress\u2014a sprawling gothic castle perched atop a cliff."},
			{Name: "Amber Temple", Type: "place", Description: "An ancient mountain temple containing dark vestiges and forbidden knowledge."},
			// Organisations
			{Name: "The Order of the Silver Dragon", Type: "organisation", Description: "An ancient order dedicated to fighting undead\u2014thought to be extinct."},
			{Name: "The Vistani", Type: "organisation", Description: "Nomadic people who freely travel the mists of Barovia. Their loyalty is uncertain."},
			// Items
			{Name: "The Sunsword", Type: "item", Description: "A sentient longsword of radiant energy\u2014the bane of all undead. It whispers encouragement to its wielder."},
			{Name: "Tome of Strahd", Type: "item", Description: "Strahd\u2019s personal journal\u2014contains the tragic history of his fall and clues to his weaknesses."},
			// Events
			{Name: "The Bloodmoon Eclipse", Type: "event", Description: "A rare celestial event that amplifies Strahd\u2019s power\u2014prophesied to bring his ultimate triumph or downfall."},
		},
		Quests: []Quest{
			{Name: "Escape Barovia", Description: "Find a way to lift the mists and escape the Demiplane of Dread.", Status: "active", Giver: "Madam Eva"},
			{Name: "Defeat Strahd von Zarovich", Description: "Confront and destroy the vampire lord in Castle Ravenloft\u2014the only way to truly free Barovia.", Status: "active", Giver: "Ismark the Lesser"},
			{Name: "Protect Ireena", Description: "Keep Ireena safe from Strahd\u2019s minions and his obsessive pursuit.", Status: "active", Giver: "Ismark the Lesser"},
			{Name: "Find the Sunsword", Description: "Locate the legendary blade of radiant energy, prophesied to be Strahd\u2019s bane.", Status: "completed", Giver: "Madam Eva"},
			{Name: "Recover the Tome of Strahd", Description: "Find Strahd\u2019s personal journal to learn his weaknesses.", Status: "completed", Giver: "Madam Eva"},
			{Name: "Destroy the Three Fanes", Description: "Destroy the three ancient fanes that anchor Strahd\u2019s power to the land.", Status: "active", Giver: "Beldric the Wise"},
			{Name: "Avenge Miri\u2019s Village", Description: "Find and destroy the vampire spawn that razed Miri\u2019s home village.", Status: "failed", Giver: "Miri Shadowstep"},
		},
		Stats: &CampaignStats{
			TotalSessions:    3,
			TotalDurationMin: 615,
			AvgDurationMin:   205,
			TotalWords:       45000,
			TotalQuests:      7,
			ActiveQuests:     4,
			CompletedQuests:  2,
			FailedQuests:     1,
			TotalEncounters:  12,
			TotalDamage:      890,
			EntityCounts: map[string]int{
				"pc":           3,
				"npc":          5,
				"place":        4,
				"organisation": 2,
				"item":         2,
				"event":        1,
			},
			NPCStatusCounts: map[string]int{
				"alive": 4,
				"dead":  1,
			},
		},
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// 1. Valid PDF header
	if len(data) < 5 {
		t.Fatalf("PDF too small: %d bytes", len(data))
	}
	if string(data[:5]) != "%PDF-" {
		t.Errorf("expected PDF header %%PDF-, got %q", string(data[:5]))
	}

	// 2. Reasonable file size (multi-page PDF with lots of content should be > 5KB)
	if len(data) < 5000 {
		t.Errorf("PDF suspiciously small for comprehensive content: %d bytes (expected > 5000)", len(data))
	}

	// 3. Count pages by looking for "/Type /Page" entries in the raw PDF.
	// Each page object has "/Type /Page" (not "/Type /Pages").
	pageCount := countPDFPages(data)
	if pageCount < 4 {
		t.Errorf("expected at least 4 pages (title + TOC + content), got %d", pageCount)
	}

	t.Logf("Comprehensive PDF: %d bytes, %d pages", len(data), pageCount)
}

// ---- SKT seed data stress test ----

func TestGeneratePDF_FromSeedData(t *testing.T) {
	// This test creates a CampaignBook with representative data from
	// Storm King's Thunder to stress test the PDF layout with long summaries,
	// many entities, and quests.

	now := time.Now()
	baseDate := time.Date(2026, 1, 5, 19, 0, 0, 0, time.UTC)

	makeEnd := func(start time.Time, hours, mins int) *time.Time {
		end := start.Add(time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute)
		return &end
	}

	sessions := []Session{
		{
			ID: 1, StartedAt: baseDate, EndedAt: makeEnd(baseDate, 3, 30),
			Summary: "The party arrived at Nightstone to find it devastated by a cloud giant bombardment. " +
				"Massive boulders had crushed buildings, and the Nightstone\u2014an obsidian monolith the town was named after\u2014had been " +
				"ripped from the ground and taken by the giants. Goblins and worgs had moved in to loot the ruins. " +
				"The party cleared out the goblin infestation in a swift battle, with Bramble's sneak attacks and " +
				"Elara's Thunderwave proving devastating. They found Lady Nandar dead under rubble and her journal " +
				"mentioning strange cloud formations. The townspeople had fled northeast.",
			KeyEvents: []string{
				"Arrived at devastated Nightstone",
				"Cleared goblins and worgs from the town",
				"Discovered the Nightstone monolith was stolen by cloud giants",
				"Found Lady Nandar\u2019s body and journal",
				"Tracked fleeing townspeople northeast",
			},
		},
		{
			ID: 2, StartedAt: baseDate.Add(7 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(7*24*time.Hour), 3, 15),
			Summary: "Following tracks to the Dripping Caves, the party found the Nightstone villagers held captive by " +
				"a goblin boss named Hark. Lyra's incredible persuasion (natural 20) allowed them to negotiate the " +
				"hostages' release for 80 gold and a silver dagger. The innkeeper Morak Urgray told them about giant " +
				"sightings across the Sword Coast\u2014hill giants at Goldenfields, frost giants near northern towns, " +
				"fire giants near Triboar. Elara deduced that the Ordning\u2014the giants' social hierarchy\u2014had broken down. " +
				"Morak directed them to Sheriff Markham Southwell in Bryn Shander for more information.",
			KeyEvents: []string{
				"Entered the Dripping Caves",
				"Negotiated with goblin boss Hark for hostage release",
				"Rescued fifteen Nightstone villagers",
				"Learned about giant sightings across the Sword Coast",
				"Elara identified the collapse of the Ordning",
			},
		},
		{
			ID: 3, StartedAt: baseDate.Add(14 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(14*24*time.Hour), 3, 0),
			Summary: "The journey north through the Dessarin Valley was harsh. On the road, they encountered two frost " +
				"giants ransacking a merchant\u2019s wagon, searching for the Ring of Winter. Using Elara\u2019s Major Image to " +
				"create an illusory caravan, Bramble rescued the halfling merchant Felgolos. He gave them potions and " +
				"told them about Harshnag, a frost giant friendly to small folk. After seven days of travel through " +
				"increasingly frozen terrain, they reached Bryn Shander\u2014a walled town preparing for a giant assault.",
			KeyEvents: []string{
				"Traveled north through the Dessarin Valley",
				"Encountered frost giants hunting the Ring of Winter",
				"Used illusion magic to rescue merchant Felgolos",
				"Learned about the friendly frost giant Harshnag",
				"Arrived at Bryn Shander",
			},
		},
		{
			ID: 4, StartedAt: baseDate.Add(21 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(21*24*time.Hour), 3, 45),
			Summary: "In Bryn Shander, Sheriff Southwell revealed that twelve frost giants were two days away. " +
				"The sage Beldora explained that King Hekaton\u2014the storm giant ruler\u2014had vanished after his queen Neri\u2019s " +
				"murder, causing the Ordning to collapse. She contacted Artus Cimber via sending scroll to confirm the " +
				"Ring of Winter was in Chult. Lyra climbed the walls to negotiate with Jarl Drufi in Giant, convincing " +
				"her the ring was not in Bryn Shander. Drufi smashed a wall section but withdrew. Beldora directed " +
				"them to find Harshnag and reach the Eye of the All-Father temple.",
			KeyEvents: []string{
				"Met Sheriff Southwell and sage Beldora",
				"Learned about King Hekaton\u2019s disappearance",
				"Beldora contacted Artus Cimber about the Ring of Winter",
				"Lyra negotiated with frost giant Jarl Drufi",
				"Frost giant warband withdrew from Bryn Shander",
				"Directed to find the Eye of the All-Father",
			},
		},
		{
			ID: 5, StartedAt: baseDate.Add(28 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(28*24*time.Hour), 3, 0),
			Summary: "Deep in the Spine of the World mountains, the party survived a blizzard and a yeti attack before " +
				"encountering Harshnag\u2014a massive frost giant who aided small folk. He saved them from the yetis and " +
				"agreed to guide them to the Eye of the All-Father. Along the way, he explained the giant hierarchy " +
				"and confirmed that finding King Hekaton was the key to restoring order. He carried the party's gear " +
				"through impassable snow drifts and proved to be a surprisingly gentle companion despite his fearsome appearance.",
			KeyEvents: []string{
				"Survived a mountain blizzard",
				"Fought and defeated yetis",
				"Met the friendly frost giant Harshnag",
				"Harshnag agreed to guide them to the Eye of the All-Father",
				"Learned details of giant politics from Harshnag",
			},
		},
		{
			ID: 6, StartedAt: baseDate.Add(35 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(35*24*time.Hour), 4, 0),
			Summary: "The Eye of the All-Father was a colossal temple carved into a mountain, with statues of the giant gods " +
				"lining the entrance hall. Inside, they activated a lesser oracle using a frost giant relic Harshnag carried. " +
				"The oracle revealed that King Hekaton was imprisoned by the Kraken Society and that his daughter Serissa " +
				"was struggling to maintain order in the Maelstrom. It also warned of a traitor among the storm giants. " +
				"The temple began to collapse as they activated the oracle, and Harshnag sacrificed himself holding up " +
				"the ceiling while the party escaped\u2014a deeply emotional moment for the group.",
			KeyEvents: []string{
				"Entered the Eye of the All-Father temple",
				"Activated the lesser oracle",
				"Learned Hekaton is imprisoned by the Kraken Society",
				"Oracle warned of a traitor among storm giants",
				"Temple collapsed\u2014Harshnag sacrificed himself",
			},
		},
		{
			ID: 7, StartedAt: baseDate.Add(42 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(42*24*time.Hour), 3, 30),
			Summary: "Grieving Harshnag, the party traveled to the coast to find passage to the Maelstrom. They encountered " +
				"a Kraken Society outpost disguised as a shipping company. Bramble infiltrated the building and discovered " +
				"documents revealing that Hekaton was being held on a ship called the Morkoth. The party also learned about " +
				"side crises: hill giants kidnapping farmers near Goldenfields, fire giants building a war machine called the " +
				"Vonindod, and frost giants still hunting the Ring of Winter. They decided to stay focused on the main quest.",
			KeyEvents: []string{
				"Traveled to the coast seeking passage",
				"Discovered Kraken Society outpost",
				"Bramble infiltrated and found documents about the Morkoth",
				"Learned about the Vonindod war machine",
				"Decided to focus on rescuing Hekaton",
			},
		},
		{
			ID: 8, StartedAt: baseDate.Add(49 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(49*24*time.Hour), 4, 15),
			Summary: "Using a magical conch shell found at the Kraken Society outpost, the party teleported to the Maelstrom\u2014" +
				"the storm giant court deep beneath the sea, protected by a massive air bubble. Princess Serissa was barely " +
				"holding her court together, with her advisors Uthor and Iymrith pulling her in different directions. Lyra\u2019s " +
				"diplomacy convinced Serissa to trust them. They revealed the Kraken Society\u2019s involvement and the oracle\u2019s " +
				"warning about a traitor. Iymrith\u2019s reaction was suspiciously defensive.",
			KeyEvents: []string{
				"Teleported to the Maelstrom via conch shell",
				"Met Princess Serissa of the storm giants",
				"Navigated storm giant court politics",
				"Revealed Kraken Society involvement to Serissa",
				"Noticed Iymrith\u2019s suspicious behavior",
			},
		},
		{
			ID: 9, StartedAt: baseDate.Add(56 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(56*24*time.Hour), 3, 45),
			Summary: "The party tracked the Morkoth through treacherous seas. Boarding the ship under cover of a magical fog, " +
				"they fought through Kraken Society cultists and their pet sea creatures. Deep in the hold, they found " +
				"King Hekaton chained with enchanted adamantine shackles. Elara's dispel magic freed him. Hekaton was " +
				"weakened but furious\u2014he confirmed that Iymrith, his own advisor, had orchestrated his kidnapping and " +
				"Neri\u2019s murder. She was actually an ancient blue dragon in disguise. The revelation shook everyone.",
			KeyEvents: []string{
				"Boarded the Morkoth prison ship",
				"Fought through Kraken Society cultists",
				"Freed King Hekaton from enchanted chains",
				"Hekaton revealed Iymrith as the traitor",
				"Iymrith exposed as an ancient blue dragon",
			},
		},
		{
			ID: 10, StartedAt: baseDate.Add(63 * 24 * time.Hour), EndedAt: makeEnd(baseDate.Add(63*24*time.Hour), 4, 30),
			Summary: "The final battle took place in the desert ruins of Ascore, where Iymrith had her lair. Hekaton rallied " +
				"a force of storm giants, and even frost and fire giants answered the call\u2014the first time all giant types " +
				"had united in millennia. The battle was epic: Theron used the Sunsword against Iymrith\u2019s lightning breath, " +
				"Grimnir held a chokepoint against gargoyle minions, Lyra\u2019s bardic inspiration kept the party fighting " +
				"through devastating attacks, Bramble landed a critical sneak attack on Iymrith\u2019s wing, and Elara\u2019s " +
				"counterspell stopped the dragon\u2019s last desperate teleportation attempt. Hekaton delivered the killing blow. " +
				"With Iymrith destroyed, the Ordning was restored and the giants returned to their proper hierarchy. " +
				"Hekaton thanked the party and declared them friends of all giantkind forever.",
			KeyEvents: []string{
				"Traveled to Iymrith\u2019s lair in the ruins of Ascore",
				"All giant types united for the battle",
				"Epic battle against the ancient blue dragon Iymrith",
				"Each party member played a crucial role in the fight",
				"King Hekaton delivered the killing blow",
				"The Ordning was restored",
				"Party declared friends of giantkind",
			},
		},
	}

	entities := []Entity{
		// PCs
		{Name: "Theron Brightblade", Type: "pc", Description: "Human paladin of Tyr, devoted to justice. Wields a longsword with divine smite."},
		{Name: "Bramble Thornwick", Type: "pc", Description: "Halfling rogue, master of stealth and sneak attacks. Always carries caltrops."},
		{Name: "Elara Moonwhisper", Type: "pc", Description: "Elf wizard specialising in abjuration and illusion. Scholar of giant lore."},
		{Name: "Grimnir Stonefist", Type: "pc", Description: "Dwarf fighter, at home in the frozen north. Battleaxe and shield are his best friends."},
		{Name: "Lyra Stormwind", Type: "pc", Description: "Half-elf bard of the College of Lore. Speaks Giant and has unmatched Charisma."},
		// NPCs alive
		{Name: "King Hekaton", Type: "npc", Description: "The storm giant king, ruler of the Maelstrom. Was imprisoned by the Kraken Society but freed by the party.", Status: "alive"},
		{Name: "Princess Serissa", Type: "npc", Description: "Hekaton\u2019s daughter, struggling to hold the storm giant court together in her father\u2019s absence.", Status: "alive"},
		{Name: "Morak Urgray", Type: "npc", Description: "The innkeeper of Nightstone. Provided the party with supplies and a letter of introduction.", Status: "alive"},
		{Name: "Felgolos", Type: "npc", Description: "A halfling merchant rescued from frost giants on the road. Gave the party potions and information about Harshnag.", Status: "alive"},
		{Name: "Beldora", Type: "npc", Description: "An elderly gnome sage in Bryn Shander who studies giant lore. Contacted Artus Cimber via sending scroll.", Status: "alive"},
		{Name: "Sheriff Markham Southwell", Type: "npc", Description: "The no-nonsense sheriff of Bryn Shander. Rewarded the party for saving the town.", Status: "alive"},
		// NPCs dead
		{Name: "Harshnag", Type: "npc", Description: "A friendly frost giant who guided the party to the Eye of the All-Father. Sacrificed himself holding up the collapsing temple.", Status: "dead", CauseOfDeath: "Sacrificed himself to save the party in the collapsing temple"},
		{Name: "Lady Velrosa Nandar", Type: "npc", Description: "The leader of Nightstone. Kept a journal about the Nightstone monolith and the strange clouds.", Status: "dead", CauseOfDeath: "Crushed by a boulder during the cloud giant bombardment"},
		{Name: "Iymrith", Type: "npc", Description: "An ancient blue dragon disguised as a storm giant advisor. Orchestrated Hekaton\u2019s kidnapping and Queen Neri\u2019s murder.", Status: "dead", CauseOfDeath: "Slain by King Hekaton in the ruins of Ascore"},
		{Name: "Hark", Type: "npc", Description: "A goblin boss in the Dripping Caves who held Nightstone villagers captive. Negotiated their release for gold.", Status: "alive"},
		{Name: "Jarl Drufi", Type: "npc", Description: "A frost giant war leader who besieged Bryn Shander seeking the Ring of Winter.", Status: "alive"},
		// Places
		{Name: "Nightstone", Type: "place", Description: "A small settlement devastated by a cloud giant bombardment. Named after an obsidian monolith stolen by the giants."},
		{Name: "The Dripping Caves", Type: "place", Description: "A foul-smelling cave system where goblins held Nightstone\u2019s villagers captive."},
		{Name: "Bryn Shander", Type: "place", Description: "A walled town in Icewind Dale. Nearly besieged by frost giants but saved by the party\u2019s diplomacy."},
		{Name: "The Eye of the All-Father", Type: "place", Description: "An ancient giant temple in the Spine of the World containing an oracle. Collapsed after activation."},
		{Name: "The Maelstrom", Type: "place", Description: "The storm giant court deep beneath the sea, protected by a massive air bubble."},
		{Name: "Ruins of Ascore", Type: "place", Description: "Desert ruins where Iymrith made her lair. Site of the final battle."},
		// Organisations
		{Name: "The Kraken Society", Type: "organisation", Description: "A shadowy organization that imprisoned King Hekaton and conspired with Iymrith."},
		{Name: "The Lords\u2019 Alliance", Type: "organisation", Description: "A coalition of rulers across the Sword Coast. Concerned about the giant threat."},
		// Items
		{Name: "The Nightstone Monolith", Type: "item", Description: "An ancient obsidian monolith stolen by cloud giants. Its purpose remains mysterious."},
		{Name: "Conch of Teleportation", Type: "item", Description: "A magical shell that teleports the holder to the Maelstrom. Found at the Kraken Society outpost."},
		{Name: "Ring of Winter", Type: "item", Description: "A legendary artifact granting power over cold and ice. Sought by frost giant Jarl Storvald. Held by Artus Cimber in Chult."},
		// Events
		{Name: "The Shattering of the Ordning", Type: "event", Description: "The collapse of the giant hierarchy after Hekaton\u2019s disappearance, causing chaos across the Sword Coast."},
		{Name: "The Siege of Bryn Shander", Type: "event", Description: "A frost giant warband\u2019s attempted siege of Bryn Shander, averted through Lyra\u2019s diplomacy."},
		{Name: "The Battle of Ascore", Type: "event", Description: "The final united assault by all giant types against Iymrith\u2019s lair in the desert ruins."},
	}

	quests := []Quest{
		{Name: "Investigate Nightstone", Description: "Discover what happened to the town of Nightstone and why it was attacked by cloud giants.", Status: "completed", Giver: "Gundren (quest board)"},
		{Name: "Rescue the Villagers", Description: "Track and rescue the Nightstone villagers from the Dripping Caves.", Status: "completed", Giver: "Self-motivated"},
		{Name: "Reach Bryn Shander", Description: "Travel north to Bryn Shander to meet Sheriff Southwell and learn more about the giant threat.", Status: "completed", Giver: "Morak Urgray"},
		{Name: "Defend Bryn Shander", Description: "Help defend the town against the approaching frost giant warband.", Status: "completed", Giver: "Sheriff Markham Southwell"},
		{Name: "Find Harshnag", Description: "Locate the friendly frost giant Harshnag in the Spine of the World mountains.", Status: "completed", Giver: "Felgolos / Beldora"},
		{Name: "Reach the Eye of the All-Father", Description: "Travel to the ancient giant temple and consult the oracle about King Hekaton\u2019s location.", Status: "completed", Giver: "Beldora"},
		{Name: "Discover Hekaton\u2019s Prison", Description: "Find out where the Kraken Society is holding King Hekaton.", Status: "completed", Giver: "The Oracle"},
		{Name: "Infiltrate the Kraken Society", Description: "Locate and infiltrate the Kraken Society\u2019s coastal outpost.", Status: "completed", Giver: "Self-motivated"},
		{Name: "Free King Hekaton", Description: "Board the Morkoth and rescue King Hekaton from his enchanted chains.", Status: "completed", Giver: "Princess Serissa"},
		{Name: "Defeat Iymrith", Description: "Confront and destroy the ancient blue dragon Iymrith in her lair at the ruins of Ascore.", Status: "completed", Giver: "King Hekaton"},
		{Name: "Restore the Ordning", Description: "Help restore the giant hierarchy to end the chaos across the Sword Coast.", Status: "completed", Giver: "Harshnag"},
		{Name: "Find the Ring of Winter", Description: "A side quest to locate the Ring of Winter before the frost giants. Deprioritized in favor of the main quest.", Status: "failed", Giver: "Beldora"},
		{Name: "Stop the Vonindod", Description: "Investigate the fire giants\u2019 war machine construction near Ironslag. Never pursued.", Status: "failed", Giver: "The Oracle"},
	}

	book := &CampaignBook{
		Campaign: Campaign{
			Name:        "Storm King\u2019s Thunder",
			Description: "The giants have gone mad. With the Ordning shattered and King Hekaton missing, " +
				"five heroes must journey across the Sword Coast to uncover a conspiracy, unite unlikely allies, " +
				"and confront an ancient dragon to restore order to the world.",
			CreatedAt: now.Add(-90 * 24 * time.Hour),
		},
		Recap: "It began with boulders falling from the sky. The party arrived at the small town of Nightstone " +
			"to find it devastated by a cloud giant bombardment\u2014the obsidian monolith the town was named after had been " +
			"ripped from the earth and carried away.\n\n" +
			"From those humble beginnings, the heroes uncovered a crisis spanning the entire Sword Coast. " +
			"The Ordning\u2014the ancient hierarchy governing all giants\u2014had shattered when King Hekaton vanished. " +
			"Every giant type scrambled for dominance: hill giants raided farms, frost giants hunted legendary artifacts, " +
			"fire giants built weapons of war, and cloud giants collected mysterious relics.\n\n" +
			"Guided by the sage Beldora and the friendly frost giant Harshnag, the party reached the Eye of the All-Father, " +
			"an ancient oracle temple. There they learned that Hekaton was imprisoned by the Kraken Society, and that a " +
			"traitor lurked among his own advisors. Harshnag gave his life to ensure they escaped the collapsing temple.\n\n" +
			"Through infiltration, diplomacy, and no small amount of combat, the heroes freed Hekaton and exposed the traitor: " +
			"Iymrith, an ancient blue dragon disguised as a storm giant advisor. The final battle in the ruins of Ascore saw " +
			"all giant types unite for the first time in millennia. With Iymrith destroyed and Hekaton restored, the Ordning " +
			"was reestablished and peace returned to the Sword Coast.\n\n" +
			"The five heroes\u2014Theron, Bramble, Elara, Grimnir, and Lyra\u2014were declared friends of all giantkind, " +
			"a title that would echo through history.",
		Sessions: sessions,
		Entities: entities,
		Quests:   quests,
		Stats: &CampaignStats{
			TotalSessions:    10,
			TotalDurationMin: 2175,
			AvgDurationMin:   217.5,
			TotalWords:       185000,
			TotalQuests:      13,
			ActiveQuests:     0,
			CompletedQuests:  11,
			FailedQuests:     2,
			TotalEncounters:  28,
			TotalDamage:      4250,
			EntityCounts: map[string]int{
				"pc":           5,
				"npc":          11,
				"place":        6,
				"organisation": 2,
				"item":         3,
				"event":        3,
			},
			NPCStatusCounts: map[string]int{
				"alive": 8,
				"dead":  3,
			},
		},
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Valid PDF
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatal("output is not a valid PDF")
	}

	// Size check: with 10 sessions, 30 entities, 13 quests, long recap, and stats
	// this should produce a substantial PDF.
	if len(data) < 10000 {
		t.Errorf("SKT PDF suspiciously small: %d bytes (expected > 10KB for this amount of content)", len(data))
	}

	// Page count: title + TOC + recap + sessions (multiple pages) + entities + quests + stats
	// Should be well over 5 pages.
	pageCount := countPDFPages(data)
	if pageCount < 6 {
		t.Errorf("expected at least 6 pages for SKT data, got %d", pageCount)
	}

	t.Logf("SKT seed data PDF: %d bytes, %d pages", len(data), pageCount)
}

// ---- Test empty content handling ----

func TestGeneratePDF_EmptySections(t *testing.T) {
	// All sections are present but empty -- the PDF should still render
	// with "No data available" style messages and not error.
	book := &CampaignBook{
		Campaign: Campaign{
			Name:        "Campaign with No Content",
			Description: "This campaign has just started.",
			CreatedAt:   time.Now(),
		},
		// No recap, sessions, entities, quests, or stats
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error for empty sections: %v", err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatal("output is not a valid PDF")
	}

	// Should still have multiple pages: title + TOC + empty recap + empty sessions + empty entities + empty quests
	pageCount := countPDFPages(data)
	if pageCount < 2 {
		t.Errorf("expected at least 2 pages for empty sections PDF, got %d", pageCount)
	}

	t.Logf("Empty sections PDF: %d bytes, %d pages", len(data), pageCount)
}

// ---- Test Unicode content does not cause errors ----

func TestGeneratePDF_UnicodeContent(t *testing.T) {
	now := time.Now()
	end := now.Add(2 * time.Hour)

	book := &CampaignBook{
		Campaign: Campaign{
			Name:        "The Wizard\u2019s \u201CUltimate\u201D Challenge\u2014A Tale of \u2026Mystery",
			Description: "An adventure featuring em-dashes\u2014curly quotes\u201C\u201D\u2014and ellipses\u2026",
			CreatedAt:   now,
		},
		Recap: "The heroes\u2019 journey began with a prophecy\u2026 one that spoke of \u201Cthe chosen ones.\u201D\n\n" +
			"They traveled far\u2014across mountains and seas\u2014to fulfill their destiny. " +
			"The wizard\u2019s tower loomed ahead, its spire piercing the clouds like a needle\u2026",
		Sessions: []Session{
			{
				ID: 1, StartedAt: now, EndedAt: &end,
				Summary:   "The party entered the wizard\u2019s tower\u2014a place of \u201Cwonders\u201D and \u201Chorrors.\u201D Each room tested them\u2026 body, mind, and soul.",
				KeyEvents: []string{"\u2022 Solved the riddle of the \u201Cmirror room\u201D", "Defeated the \u201Cshadow guardian\u201D\u2014a fearsome construct", "Found a mysterious artifact\u2026"},
			},
		},
		Entities: []Entity{
			{Name: "Aldric the \u201CUnbreakable\u201D", Type: "npc", Description: "A warrior whose strength is legendary\u2014none have defeated him in single combat\u2026 yet.", Status: "alive"},
			{Name: "The Shadow Guardian", Type: "npc", Description: "A construct of pure darkness\u2014destroyed by the party\u2019s combined might.", Status: "dead", CauseOfDeath: "Slain by the party\u2019s combined radiant magic"},
		},
		Quests: []Quest{
			{Name: "The Wizard\u2019s Challenge", Description: "Complete the \u201Cultimate trial\u201D within the tower\u2014failure means eternal imprisonment\u2026", Status: "active", Giver: "The Wizard Zephyr"},
		},
	}

	data, err := Generate(book)
	if err != nil {
		t.Fatalf("Generate returned error with Unicode content: %v", err)
	}

	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatal("output is not a valid PDF with Unicode content")
	}

	if len(data) < 2000 {
		t.Errorf("Unicode PDF suspiciously small: %d bytes", len(data))
	}

	t.Logf("Unicode content PDF: %d bytes", len(data))
}

// ---- Helper ----

// countPDFPages counts the approximate number of pages in a raw PDF by looking
// for "/Type /Page" entries that are NOT "/Type /Pages" (the page tree node).
func countPDFPages(data []byte) int {
	// We look for "/Type /Page\n" or "/Type /Page " or "/Type /Page/"
	// but NOT "/Type /Pages".
	content := string(data)
	count := 0
	needle := "/Type /Page"
	idx := 0
	for {
		pos := strings.Index(content[idx:], needle)
		if pos < 0 {
			break
		}
		absPos := idx + pos
		afterPos := absPos + len(needle)
		if afterPos < len(content) {
			nextChar := content[afterPos]
			// If the next char is 's' or 'S', this is "/Type /Pages" -- skip it.
			if nextChar != 's' && nextChar != 'S' {
				count++
			}
		}
		idx = afterPos
	}
	return count
}
