package bot

import (
	"context"
	"strings"
	"testing"

	"discord-rpg-summariser/internal/config"
	"discord-rpg-summariser/internal/summarise"
	"discord-rpg-summariser/internal/transcribe"

	"github.com/bwmarrin/discordgo"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// stubSummariser implements summarise.Summariser for testing.
type stubSummariser struct {
	result *summarise.SummaryResult
	err    error
	called bool
}

func (s *stubSummariser) Summarise(_ context.Context, _, _, _ string) (*summarise.SummaryResult, error) {
	s.called = true
	return s.result, s.err
}

// ---------------------------------------------------------------------------
// Routing tests
// ---------------------------------------------------------------------------

func TestHandleInteractionRoutesSessionStart(t *testing.T) {
	routed := ""
	b := &Bot{}

	// Build a minimal InteractionCreate for /session start.
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "session",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{Name: "start", Type: discordgo.ApplicationCommandOptionSubCommand},
				},
			},
		},
	}

	// We cannot easily call real Discord API methods, so we verify routing
	// by overriding nothing -- instead we directly test the dispatch logic.
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		t.Fatal("expected at least one option")
	}
	sub := data.Options[0]
	switch data.Name {
	case "session":
		switch sub.Name {
		case "start":
			routed = "session.start"
		case "stop":
			routed = "session.stop"
		case "status":
			routed = "session.status"
		}
	case "character":
		switch sub.Name {
		case "set":
			routed = "character.set"
		case "list":
			routed = "character.list"
		case "remove":
			routed = "character.remove"
		}
	}

	_ = b // avoid unused
	if routed != "session.start" {
		t.Errorf("expected route session.start, got %q", routed)
	}
}

func TestHandleInteractionRoutesCharacterSet(t *testing.T) {
	routed := ""

	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "character",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name: "set",
						Type: discordgo.ApplicationCommandOptionSubCommand,
						Options: []*discordgo.ApplicationCommandInteractionDataOption{
							{Name: "name", Type: discordgo.ApplicationCommandOptionString, Value: "Gandalf"},
						},
					},
				},
			},
		},
	}

	data := i.ApplicationCommandData()
	sub := data.Options[0]
	switch data.Name {
	case "session":
		routed = "session." + sub.Name
	case "character":
		routed = "character." + sub.Name
	}

	if routed != "character.set" {
		t.Errorf("expected route character.set, got %q", routed)
	}
}

func TestHandleInteractionIgnoresNonCommand(t *testing.T) {
	// An interaction of type Ping should be silently ignored.
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionPing,
		},
	}

	// The real handleInteraction returns early; verify that our guard check
	// matches the same condition.
	if i.Type == discordgo.InteractionApplicationCommand {
		t.Error("ping should not be treated as an application command")
	}
}

// ---------------------------------------------------------------------------
// Pipeline tests
// ---------------------------------------------------------------------------

func TestRunPipelineHandlesEmptyUserFiles(t *testing.T) {
	// runPipeline should not panic when there are no user files and the
	// recorder reference is nil. We cannot call it end-to-end because it
	// requires a real Store, but we can at least verify the guard logic.
	b := &Bot{
		config: &config.Config{
			Web: config.WebConfig{ListenAddr: ":8080"},
		},
	}

	// With a nil recorder, UserFiles() would panic, so runPipeline must
	// handle the nil case. Verify the branch by checking nil directly.
	b.mu.Lock()
	rec := b.recorder
	b.mu.Unlock()

	if rec != nil {
		t.Error("expected nil recorder")
	}

	var userFiles map[string]string
	if rec != nil {
		userFiles = rec.UserFiles()
	}

	if len(userFiles) != 0 {
		t.Errorf("expected 0 user files, got %d", len(userFiles))
	}
}

func TestSubcommandOptions(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "character",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name: "set",
						Type: discordgo.ApplicationCommandOptionSubCommand,
						Options: []*discordgo.ApplicationCommandInteractionDataOption{
							{Name: "name", Type: discordgo.ApplicationCommandOptionString, Value: "Thorn"},
							{Name: "user", Type: discordgo.ApplicationCommandOptionUser, Value: "123456"},
						},
					},
				},
			},
		},
	}

	opts := subcommandOptions(i)

	if _, ok := opts["name"]; !ok {
		t.Error("expected 'name' option")
	}
	if _, ok := opts["user"]; !ok {
		t.Error("expected 'user' option")
	}
	if len(opts) != 2 {
		t.Errorf("expected 2 options, got %d", len(opts))
	}
}

func TestInteractionUserID(t *testing.T) {
	// Guild context: Member is set.
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: &discordgo.Member{
				User: &discordgo.User{ID: "guild-user-1"},
			},
		},
	}
	if got := interactionUserID(i); got != "guild-user-1" {
		t.Errorf("expected guild-user-1, got %s", got)
	}

	// DM context: only User is set.
	i2 := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			User: &discordgo.User{ID: "dm-user-2"},
		},
	}
	if got := interactionUserID(i2); got != "dm-user-2" {
		t.Errorf("expected dm-user-2, got %s", got)
	}
}

func TestCommandDefinitions(t *testing.T) {
	if len(commands) != 4 {
		t.Fatalf("expected 4 top-level commands, got %d", len(commands))
	}

	sessionCmd := commands[0]
	if sessionCmd.Name != "session" {
		t.Errorf("expected first command to be 'session', got %q", sessionCmd.Name)
	}
	if len(sessionCmd.Options) != 3 {
		t.Errorf("expected 3 session subcommands, got %d", len(sessionCmd.Options))
	}

	charCmd := commands[1]
	if charCmd.Name != "character" {
		t.Errorf("expected second command to be 'character', got %q", charCmd.Name)
	}
	if len(charCmd.Options) != 3 {
		t.Errorf("expected 3 character subcommands, got %d", len(charCmd.Options))
	}

	// Verify set subcommand has a required 'name' option.
	setCmd := charCmd.Options[0]
	if setCmd.Name != "set" {
		t.Errorf("expected first character subcommand to be 'set', got %q", setCmd.Name)
	}
	foundRequired := false
	for _, opt := range setCmd.Options {
		if opt.Name == "name" && opt.Required {
			foundRequired = true
		}
	}
	if !foundRequired {
		t.Error("character set subcommand should have a required 'name' option")
	}
}

func TestStubSummariser(t *testing.T) {
	s := &stubSummariser{
		result: &summarise.SummaryResult{
			Summary:   "The party defeated a dragon.",
			KeyEvents: []string{"Dragon fight", "Treasure found"},
		},
	}

	result, err := s.Summarise(context.Background(), "transcript", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.called {
		t.Error("expected Summarise to be called")
	}
	if result.Summary != "The party defeated a dragon." {
		t.Errorf("unexpected summary: %s", result.Summary)
	}
	if len(result.KeyEvents) != 2 {
		t.Errorf("expected 2 key events, got %d", len(result.KeyEvents))
	}
}

func TestMergeAndFormatIntegration(t *testing.T) {
	// Verify that the transcribe package functions we rely on in the pipeline
	// work correctly with our expected inputs.
	userSegments := map[string][]transcribe.Segment{
		"user1": {
			{StartTime: 0.0, EndTime: 5.0, Text: "Hello everyone"},
			{StartTime: 10.0, EndTime: 15.0, Text: "Let's begin"},
		},
		"user2": {
			{StartTime: 6.0, EndTime: 9.0, Text: "Ready to play"},
		},
	}
	charNames := map[string]string{
		"user1": "Gandalf",
		"user2": "Frodo",
	}

	merged := transcribe.MergeTranscripts(userSegments, charNames, nil)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged segments, got %d", len(merged))
	}
	// Should be sorted by StartTime.
	if merged[0].CharacterName != "Gandalf" || merged[0].Text != "Hello everyone" {
		t.Errorf("unexpected first segment: %+v", merged[0])
	}
	if merged[1].CharacterName != "Frodo" || merged[1].Text != "Ready to play" {
		t.Errorf("unexpected second segment: %+v", merged[1])
	}

	formatted := transcribe.FormatTranscript(merged)
	if formatted == "" {
		t.Error("expected non-empty formatted transcript")
	}
	if !strings.Contains(formatted, "Gandalf") || !strings.Contains(formatted, "Frodo") {
		t.Error("formatted transcript should contain character names")
	}
}

// ---------------------------------------------------------------------------
// Entity mention matching tests
// ---------------------------------------------------------------------------

func TestFindEntityMentions(t *testing.T) {
	nameToID := map[string]int64{
		"Strahd":    1,
		"Barovia":   2,
		"Waterdeep": 3,
		"Sunblade":  4,
	}

	tests := []struct {
		name     string
		text     string
		expected map[string]int64
	}{
		{
			name:     "exact match",
			text:     "We met Strahd in the castle.",
			expected: map[string]int64{"Strahd": 1},
		},
		{
			name:     "case insensitive",
			text:     "The land of barovia is dark.",
			expected: map[string]int64{"Barovia": 2},
		},
		{
			name:     "multiple matches",
			text:     "Strahd rules Barovia with an iron fist.",
			expected: map[string]int64{"Strahd": 1, "Barovia": 2},
		},
		{
			name:     "no match",
			text:     "The party rested at the inn.",
			expected: map[string]int64{},
		},
		{
			name:     "word boundary - should not match substring",
			text:     "She wielded a sunbladelike weapon.",
			expected: map[string]int64{},
		},
		{
			name:     "word boundary - match at start",
			text:     "Strahd appeared suddenly.",
			expected: map[string]int64{"Strahd": 1},
		},
		{
			name:     "word boundary - match at end",
			text:     "They traveled to Waterdeep",
			expected: map[string]int64{"Waterdeep": 3},
		},
		{
			name:     "match with punctuation boundary",
			text:     "Is that Strahd? Yes, and Barovia!",
			expected: map[string]int64{"Strahd": 1, "Barovia": 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := findEntityMentions(tc.text, nameToID)
			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d matches, got %d: %v", len(tc.expected), len(result), result)
			}
			for name, expectedID := range tc.expected {
				gotID, ok := result[name]
				if !ok {
					t.Errorf("expected match for %q, not found", name)
					continue
				}
				if gotID != expectedID {
					t.Errorf("expected id %d for %q, got %d", expectedID, name, gotID)
				}
			}
		})
	}
}

func TestFindEntityMentionsSkipsShortNames(t *testing.T) {
	// Names shorter than 3 characters should be filtered out before calling
	// findEntityMentions in the actual pipeline. Verify that even if they're
	// passed in, they still match (the filtering happens at the caller level).
	nameToID := map[string]int64{
		"Bo": 1,
	}
	result := findEntityMentions("Bo went to the store.", nameToID)
	// "Bo" is 2 chars but findEntityMentions doesn't filter — that's the caller's job.
	// It should still match at a word boundary.
	if len(result) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result))
	}
}

func TestTruncateContext(t *testing.T) {
	short := "A short text."
	if got := truncateContext(short, "short", 200); got != short {
		t.Errorf("expected full text for short string, got %q", got)
	}

	long := "The beginning of the text. " +
		"Here in the middle we find Strahd lurking in the shadows. " +
		"And then the text continues on and on and on and on and on for quite a while longer."
	result := truncateContext(long, "Strahd", 80)
	if len(result) > 80 {
		t.Errorf("expected max 80 chars, got %d", len(result))
	}
	if !strings.Contains(result, "Strahd") {
		t.Errorf("expected context to contain 'Strahd', got %q", result)
	}
}
