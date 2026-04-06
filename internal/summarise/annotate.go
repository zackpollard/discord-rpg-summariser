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
	Speaker   string  // character name, "DM", or discord display name
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
	Classification string  `json:"classification"`   // narrative, table_talk, ambiguous
	CorrectedText  *string `json:"corrected_text"`   // null if no correction needed
	Scene          *string `json:"scene"`             // scene label, null if same as previous
	NPCVoice       *string `json:"npc_voice"`         // NPC name if DM is voicing, null otherwise
	MergeWithNext  bool    `json:"merge_with_next"`   // true if this segment should be merged with the next
	Tone           *string `json:"tone"`              // emotional tone: dramatic, funny, tense, sad, neutral, etc.
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

func formatAnnotationTime(secs float64) string {
	h := int(secs) / 3600
	m := (int(secs) % 3600) / 60
	s := int(secs) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
