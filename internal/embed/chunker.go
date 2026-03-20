package embed

import (
	"fmt"
	"strings"
)

// Chunk represents a text chunk ready for embedding.
type Chunk struct {
	DocType   string // "transcript_chunk", "summary", "entity", "quest"
	DocID     int64  // unique within doc type (e.g. segment index for chunks)
	SessionID int64  // 0 if not session-scoped
	Title     string
	Content   string
}

// TranscriptSegment is a minimal representation of a transcript segment
// for chunking purposes, avoiding a dependency on the storage package.
type TranscriptSegment struct {
	UserID    string
	StartTime float64
	EndTime   float64
	Text      string
}

// maxChunkChars is the target maximum character count per transcript chunk.
const maxChunkChars = 2000

// ChunkTranscriptSegments groups consecutive transcript segments into chunks
// of approximately maxChunkChars, adding speaker labels from charNames.
// Each chunk gets a sequential DocID starting from 1.
func ChunkTranscriptSegments(segments []TranscriptSegment, sessionID int64, charNames map[string]string) []Chunk {
	if len(segments) == 0 {
		return nil
	}

	var chunks []Chunk
	var buf strings.Builder
	chunkIdx := int64(1)
	startTime := segments[0].StartTime

	for _, seg := range segments {
		name := charNames[seg.UserID]
		if name == "" {
			name = seg.UserID
		}

		line := fmt.Sprintf("[%s] %s: %s\n", formatSeconds(seg.StartTime), name, seg.Text)

		// If adding this line would exceed the limit and we already have content,
		// flush the current chunk.
		if buf.Len()+len(line) > maxChunkChars && buf.Len() > 0 {
			chunks = append(chunks, Chunk{
				DocType:   "transcript_chunk",
				DocID:     sessionID*10000 + chunkIdx,
				SessionID: sessionID,
				Title:     fmt.Sprintf("Transcript chunk %d (from %s)", chunkIdx, formatSeconds(startTime)),
				Content:   buf.String(),
			})
			buf.Reset()
			chunkIdx++
			startTime = seg.StartTime
		}

		buf.WriteString(line)
	}

	// Flush remaining content.
	if buf.Len() > 0 {
		chunks = append(chunks, Chunk{
			DocType:   "transcript_chunk",
			DocID:     sessionID*10000 + chunkIdx,
			SessionID: sessionID,
			Title:     fmt.Sprintf("Transcript chunk %d (from %s)", chunkIdx, formatSeconds(startTime)),
			Content:   buf.String(),
		})
	}

	return chunks
}

// BuildEntityText constructs a text representation of an entity suitable for embedding.
func BuildEntityText(name, entityType, description string, notes []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)", name, entityType)
	if description != "" {
		fmt.Fprintf(&b, ": %s", description)
	}
	for _, note := range notes {
		fmt.Fprintf(&b, "\n- %s", note)
	}
	return b.String()
}

// BuildQuestText constructs a text representation of a quest suitable for embedding.
func BuildQuestText(name, description, status, giver string, updates []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Quest: %s [%s]", name, status)
	if giver != "" {
		fmt.Fprintf(&b, " (given by %s)", giver)
	}
	if description != "" {
		fmt.Fprintf(&b, "\n%s", description)
	}
	for _, u := range updates {
		fmt.Fprintf(&b, "\n- %s", u)
	}
	return b.String()
}

// formatSeconds converts a float64 seconds value to "HH:MM:SS".
func formatSeconds(secs float64) string {
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
