package summarise

import "fmt"

// truncateTranscript caps a transcript to maxChars by keeping the beginning
// and end of the text, so the model sees the full session arc. The middle
// is replaced with a marker showing how much was omitted.
func truncateTranscript(transcript string, maxChars int) string {
	if len(transcript) <= maxChars {
		return transcript
	}

	// Split 60/40 — more from the start (context setup) and less from the end
	// (session conclusion). Both halves are trimmed to line boundaries.
	headSize := maxChars * 60 / 100
	tailSize := maxChars - headSize

	head := transcript[:headSize]
	// Trim to last complete line.
	if idx := lastNewline(head); idx > 0 {
		head = head[:idx+1]
	}

	tail := transcript[len(transcript)-tailSize:]
	// Trim to first complete line.
	if idx := firstNewline(tail); idx >= 0 && idx < len(tail)-1 {
		tail = tail[idx+1:]
	}

	omitted := len(transcript) - len(head) - len(tail)
	return fmt.Sprintf("%s\n\n... [%d characters omitted — see summary for full context] ...\n\n%s", head, omitted, tail)
}

func lastNewline(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}

func firstNewline(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}
