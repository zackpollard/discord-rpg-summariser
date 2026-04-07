package summarise

import (
	"context"
	"fmt"
	"strings"
)

// TranscriptAnnotator annotates transcript segments with classification,
// corrections, scene boundaries, and NPC voice attribution.
type TranscriptAnnotator interface {
	AnnotateTranscript(ctx context.Context, segments []AnnotationInput, vocab AnnotationVocabulary, dmName string) (*AnnotationResult, error)
}

// AnnotationInput represents a single transcript segment to annotate.
type AnnotationInput struct {
	ID        int64
	Speaker   string // character name, "DM", or discord display name
	StartTime float64
	Text      string
}

// AnnotationVocabulary provides campaign context for the annotation.
type AnnotationVocabulary struct {
	CharacterNames []string // player character names
	EntityNames    []string // known NPCs, places, items
	GameSystem     string
}

// AnnotatedSegment is the per-segment annotation output from the LLM.
type AnnotatedSegment struct {
	ID             int64   `json:"id"`
	Classification string  `json:"classification"`  // narrative, table_talk, ambiguous
	CorrectedText  *string `json:"corrected_text"`  // null if no correction needed
	Scene          *string `json:"scene"`           // scene label, null if same as previous
	NPCVoice       *string `json:"npc_voice"`       // NPC name if DM is voicing, null otherwise
	MergeWithNext  bool    `json:"merge_with_next"` // true if this segment should be merged with the next
	Tone           *string `json:"tone"`            // emotional tone: dramatic, funny, tense, sad, neutral, etc.
}

// AnnotationResult is the top-level JSON response.
type AnnotationResult struct {
	Segments []AnnotatedSegment `json:"segments"`
}

// BuildAnnotationPrompt constructs the prompt for transcript annotation.
func BuildAnnotationPrompt(segments []AnnotationInput, vocab AnnotationVocabulary, dmName string) string {
	var b strings.Builder

	b.WriteString(`You are an expert TTRPG session annotator. Analyze each transcript segment and provide annotations.

## Your Tasks

### 1. Classification
Classify each segment as one of:
- **narrative**: Content that actually happened in the game story — in-character dialogue, DM narration, combat actions, skill checks, exploration, roleplay
- **table_talk**: Jokes about the game, hypothetical "what if we did X" that didn't happen, meta-discussion about rules, out-of-character banter, real-world references, laughter/reactions that aren't part of the story
- **ambiguous**: Unclear whether it's narrative or table talk

### 2. Transcript Correction
Fix ASR (speech recognition) errors in proper nouns ONLY. Use the vocabulary list below as reference. Do NOT rephrase or rewrite general speech. Set corrected_text to null if no correction is needed.

### 3. Scene Boundaries
When the narrative shifts to a different scene type, set the "scene" field to a short label (2-4 words). Common scene types: exploration, combat, roleplay, shopping, travel, rest, puzzle, planning, chase. Set to null for segments continuing the same scene.

### 4. NPC Voice Attribution
When the DM is voicing an NPC (speaking in character as an NPC), set "npc_voice" to the NPC's name. Only applies to DM segments. Set to null for DM narration or non-DM speakers.

### 5. Segment Merging
Sometimes ASR splits one continuous sentence across multiple segments. When a segment is clearly the first part of a sentence that continues in the next segment (e.g. ends mid-phrase without punctuation), set "merge_with_next" to true. Otherwise false.

### 6. Emotional Tone
Tag each narrative segment with its emotional tone. Use one of: "dramatic", "funny", "tense", "sad", "triumphant", "mysterious", "casual", "angry", "neutral". This helps identify memorable moments. Set to null for table_talk segments.

`)

	// Vocabulary context.
	if len(vocab.CharacterNames) > 0 || len(vocab.EntityNames) > 0 {
		b.WriteString("## Known Vocabulary\n\n")
		if len(vocab.CharacterNames) > 0 {
			b.WriteString("**Player Characters:** ")
			b.WriteString(strings.Join(vocab.CharacterNames, ", "))
			b.WriteString("\n\n")
		}
		if len(vocab.EntityNames) > 0 {
			b.WriteString("**Known NPCs/Places/Items:** ")
			b.WriteString(strings.Join(vocab.EntityNames, ", "))
			b.WriteString("\n\n")
		}
	}

	if vocab.GameSystem != "" {
		fmt.Fprintf(&b, "**Game System:** %s\n\n", vocab.GameSystem)
	}

	if dmName != "" {
		fmt.Fprintf(&b, "**DM:** %s\n\n", dmName)
	}

	// Transcript.
	b.WriteString("## Transcript\n\n")

	for _, seg := range segments {
		ts := formatAnnotationTime(seg.StartTime)
		fmt.Fprintf(&b, "#%d [%s] %s: %s\n", seg.ID, ts, seg.Speaker, seg.Text)
	}

	// Output format.
	b.WriteString(`
## Output Format

Respond with a JSON object containing a "segments" array. Each element must have:
- "id": the segment ID number (from #N above)
- "classification": "narrative", "table_talk", or "ambiguous"
- "corrected_text": corrected text string, or null if no correction needed
- "scene": scene label string (only on the first segment of a new scene), or null
- "npc_voice": NPC name string (only for DM segments voicing an NPC), or null
- "merge_with_next": boolean, true if this segment continues into the next
- "tone": emotional tone string (dramatic/funny/tense/sad/triumphant/mysterious/casual/angry/neutral), or null for table_talk

Output ONLY the JSON object, no other text.
`)

	return b.String()
}

// TitleAndQuotesExtractor generates a session title and extracts memorable quotes.
type TitleAndQuotesExtractor interface {
	ExtractTitleAndQuotes(ctx context.Context, transcript, summary, dmName string) (*TitleAndQuotesResult, error)
}

// TitleAndQuotesResult is the JSON response from the title/quotes LLM call.
type TitleAndQuotesResult struct {
	Title  string           `json:"title"`
	Quotes []ExtractedQuote `json:"quotes"`
}

// ExtractedQuote is a single memorable quote extracted from the transcript.
type ExtractedQuote struct {
	Speaker   string  `json:"speaker"`
	Text      string  `json:"text"`
	StartTime float64 `json:"start_time"`
	Tone      string  `json:"tone"`
}

// BuildTitleAndQuotesPrompt constructs the prompt for session title generation
// and memorable quote extraction.
func BuildTitleAndQuotesPrompt(transcript, summary, dmName string) string {
	var b strings.Builder

	b.WriteString("You are an expert at creating evocative titles for tabletop RPG sessions and identifying memorable moments.\n\n")

	if dmName != "" {
		fmt.Fprintf(&b, "The DM is %s.\n\n", dmName)
	}

	b.WriteString("## Session Summary\n\n")
	b.WriteString(summary)
	b.WriteString("\n\n## Session Transcript\n\n")
	b.WriteString(transcript)

	b.WriteString("\n\n## Instructions\n\n")
	b.WriteString("Based on the summary and transcript above, produce:\n\n")
	b.WriteString("1. **A session title** (3-8 words): dramatic, evocative, and specific to what happened. ")
	b.WriteString("Good examples: \"The Ambush at Thornwall\", \"A Deal with the Devil\", \"Blood on the Altar\", \"The Crown of Forgotten Kings\". ")
	b.WriteString("Avoid generic titles like \"An Adventurous Session\" or \"The Next Chapter\".\n\n")
	b.WriteString("2. **Memorable direct quotes** from the transcript (0-10). Only include quotes that are genuinely:\n")
	b.WriteString("   - Funny, dramatic, or emotionally impactful\n")
	b.WriteString("   - Iconic character moments\n")
	b.WriteString("   - Lines players would remember and laugh or gasp about\n")
	b.WriteString("   - If nothing stands out as truly quote-worthy, return an empty array — do NOT force quotes\n")
	b.WriteString("   - Include the exact speaker name, approximate start_time (in seconds from the transcript timestamps), and a tone tag\n")
	b.WriteString("   - Tone tags: funny, dramatic, tense, sad, triumphant, mysterious, angry, badass, wholesome\n\n")

	b.WriteString("Return ONLY valid JSON with exactly these fields:\n")
	b.WriteString("{\n")
	b.WriteString("  \"title\": \"The Session Title Here\",\n")
	b.WriteString("  \"quotes\": [\n")
	b.WriteString("    {\"speaker\": \"CharacterName\", \"text\": \"The exact quote.\", \"start_time\": 1234.5, \"tone\": \"funny\"}\n")
	b.WriteString("  ]\n")
	b.WriteString("}\n")

	return b.String()
}

func formatAnnotationTime(secs float64) string {
	h := int(secs) / 3600
	m := (int(secs) % 3600) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// --- "Previously On..." Generator ---

// PreviouslyOnGenerator generates a dramatic "Previously on..." narration.
type PreviouslyOnGenerator interface {
	GeneratePreviouslyOn(ctx context.Context, lastSessionSummary, campaignRecap string) (*PreviouslyOnResult, error)
}

// PreviouslyOnResult holds the generated "Previously on..." text.
type PreviouslyOnResult struct {
	Text string `json:"text"`
}

// BuildPreviouslyOnPrompt constructs the prompt for a dramatic "Previously on..." narration.
func BuildPreviouslyOnPrompt(lastSessionSummary, campaignRecap string) string {
	var b strings.Builder

	b.WriteString("You are a dramatic narrator for a tabletop RPG show, similar to the voice-over at the start of a TV episode.\n\n")
	b.WriteString("Your task: Write a dramatic 2-3 paragraph \"Previously on...\" narration designed to be read aloud at the start of the next session.\n\n")

	b.WriteString("## Guidelines\n\n")
	b.WriteString("- Focus on cliffhangers, unresolved tensions, and immediate context from the LAST session\n")
	b.WriteString("- Use dramatic, cinematic language — build tension and anticipation\n")
	b.WriteString("- End with a hook that makes players excited to continue\n")
	b.WriteString("- Use character names, not player names\n")
	b.WriteString("- Keep it to 2-3 short paragraphs (this will be read aloud, so brevity matters)\n")
	b.WriteString("- Write in present tense for immediacy (\"The party stands at the threshold...\")\n\n")

	if campaignRecap != "" {
		b.WriteString("## Campaign Context (for background only)\n\n")
		b.WriteString(campaignRecap)
		b.WriteString("\n\n")
	}

	b.WriteString("## Last Session Summary (focus on this)\n\n")
	b.WriteString(lastSessionSummary)
	b.WriteString("\n\n")

	b.WriteString("Return ONLY valid JSON with exactly this field:\n")
	b.WriteString("{\n")
	b.WriteString("  \"text\": \"The full Previously On narration text.\"\n")
	b.WriteString("}\n")

	return b.String()
}

// --- Character Summary Generator ---

// CharacterSummaryGenerator generates per-character story arc summaries.
type CharacterSummaryGenerator interface {
	GenerateCharacterSummary(ctx context.Context, characterName string, sessionSummaries []string, relationships []string) (*CharacterSummaryResult, error)
}

// CharacterSummaryResult holds the generated character story arc summary.
type CharacterSummaryResult struct {
	StoryArc              string             `json:"story_arc"`
	KeyMoments            []string           `json:"key_moments"`
	RelationshipSummaries []RelationshipNote `json:"relationship_summaries"`
}

// RelationshipNote describes a character's relationship with another character.
type RelationshipNote struct {
	Character string `json:"character"`
	Summary   string `json:"summary"`
}

// BuildCharacterSummaryPrompt constructs the prompt for character story arc generation.
func BuildCharacterSummaryPrompt(characterName string, sessionSummaries []string, relationships []string) string {
	var b strings.Builder

	b.WriteString("You are an expert storyteller analyzing a character's journey through a tabletop RPG campaign.\n\n")
	fmt.Fprintf(&b, "## Character: %s\n\n", characterName)

	b.WriteString("## Session Summaries (chronological)\n\n")
	for i, summary := range sessionSummaries {
		fmt.Fprintf(&b, "--- Session %d ---\n%s\n\n", i+1, summary)
	}

	if len(relationships) > 0 {
		b.WriteString("## Known Relationships\n\n")
		for _, rel := range relationships {
			fmt.Fprintf(&b, "- %s\n", rel)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Instructions\n\n")
	fmt.Fprintf(&b, "Analyze %s's journey through this campaign and produce:\n\n", characterName)
	b.WriteString("1. **story_arc**: A 1-2 paragraph narrative summary of this character's personal story arc, growth, and key decisions\n")
	b.WriteString("2. **key_moments**: 3-8 pivotal moments for this character (brief descriptions)\n")
	b.WriteString("3. **relationship_summaries**: For each significant relationship, a brief summary of how it developed\n\n")

	b.WriteString("Return ONLY valid JSON:\n")
	b.WriteString("{\n")
	b.WriteString("  \"story_arc\": \"Narrative summary...\",\n")
	b.WriteString("  \"key_moments\": [\"Moment one\", \"Moment two\"],\n")
	b.WriteString("  \"relationship_summaries\": [{\"character\": \"NPC Name\", \"summary\": \"How the relationship evolved\"}]\n")
	b.WriteString("}\n")

	return b.String()
}

// --- Combat Analyzer ---

// CombatAnalyzer generates tactical analysis of combat encounters.
type CombatAnalyzer interface {
	AnalyzeCombat(ctx context.Context, encounterSummary string, actions []string, playerCharacters []string) (*CombatAnalysisResult, error)
}

// CombatAnalysisResult holds the generated combat analysis.
type CombatAnalysisResult struct {
	TacticalSummary string `json:"tactical_summary"`
	MVP             string `json:"mvp"`
	ClosestCall     string `json:"closest_call"`
	FunniestMoment  string `json:"funniest_moment"`
}

// BuildCombatAnalysisPrompt constructs the prompt for combat encounter analysis.
func BuildCombatAnalysisPrompt(encounterSummary string, actions []string, playerCharacters []string) string {
	var b strings.Builder

	b.WriteString("You are a tactical combat analyst for tabletop RPG sessions. Analyze this combat encounter with a mix of tactical insight and entertainment.\n\n")

	b.WriteString("## Encounter Summary\n\n")
	b.WriteString(encounterSummary)
	b.WriteString("\n\n")

	if len(playerCharacters) > 0 {
		b.WriteString("## Player Characters\n\n")
		b.WriteString(strings.Join(playerCharacters, ", "))
		b.WriteString("\n\n")
	}

	b.WriteString("## Combat Actions\n\n")
	for _, action := range actions {
		fmt.Fprintf(&b, "- %s\n", action)
	}
	b.WriteString("\n")

	b.WriteString("## Instructions\n\n")
	b.WriteString("Produce a combat analysis with:\n")
	b.WriteString("1. **tactical_summary**: 1-2 paragraph analysis of the combat tactics, strategy, and flow\n")
	b.WriteString("2. **mvp**: Who performed best and why (1-2 sentences)\n")
	b.WriteString("3. **closest_call**: The most dangerous moment where things almost went wrong (1-2 sentences)\n")
	b.WriteString("4. **funniest_moment**: The most amusing or unexpected thing that happened (1-2 sentences, or empty string if nothing funny occurred)\n\n")

	b.WriteString("Return ONLY valid JSON:\n")
	b.WriteString("{\n")
	b.WriteString("  \"tactical_summary\": \"...\",\n")
	b.WriteString("  \"mvp\": \"...\",\n")
	b.WriteString("  \"closest_call\": \"...\",\n")
	b.WriteString("  \"funniest_moment\": \"...\"\n")
	b.WriteString("}\n")

	return b.String()
}

// --- Clip Name Suggester ---

// ClipNameSuggester suggests names for soundboard clips.
type ClipNameSuggester interface {
	SuggestClipNames(ctx context.Context, transcriptExcerpt string) (*ClipNameResult, error)
}

// ClipNameResult holds suggested clip names.
type ClipNameResult struct {
	Suggestions []string `json:"suggestions"`
}

// BuildClipNamePrompt constructs the prompt for clip name suggestions.
func BuildClipNamePrompt(transcriptExcerpt string) string {
	var b strings.Builder

	b.WriteString("You are a creative soundboard clip naming assistant for a tabletop RPG group.\n\n")
	b.WriteString("Given a transcript excerpt from a session, suggest 3-5 short, catchy names for a soundboard clip.\n\n")

	b.WriteString("## Guidelines\n\n")
	b.WriteString("- Names should be short (1-5 words)\n")
	b.WriteString("- Be catchy, memorable, and fun\n")
	b.WriteString("- Reference what's being said or the emotion/tone\n")
	b.WriteString("- Think of names that would make sense on a soundboard button\n")
	b.WriteString("- Include a mix: some literal, some funny, some referencing the moment\n\n")

	b.WriteString("## Transcript Excerpt\n\n")
	b.WriteString(transcriptExcerpt)
	b.WriteString("\n\n")

	b.WriteString("Return ONLY valid JSON:\n")
	b.WriteString("{\n")
	b.WriteString("  \"suggestions\": [\"Name 1\", \"Name 2\", \"Name 3\"]\n")
	b.WriteString("}\n")

	return b.String()
}
