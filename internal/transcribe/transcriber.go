package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Segment represents a single transcribed segment of audio.
type Segment struct {
	StartTime float64 // seconds from start of audio
	EndTime   float64
	Text      string
}

// Transcriber wraps the whisper-cli binary for audio transcription.
type Transcriber struct {
	binaryPath string
	modelPath  string
	threads    int
	language   string
	gpu        bool
}

// NewTranscriber creates a new Transcriber that shells out to whisper-cli.
func NewTranscriber(binaryPath, modelPath string, threads int, language string, gpu bool) *Transcriber {
	return &Transcriber{
		binaryPath: binaryPath,
		modelPath:  modelPath,
		threads:    threads,
		language:   language,
		gpu:        gpu,
	}
}

// whisperOutput mirrors the JSON structure produced by whisper-cli --output-json.
type whisperOutput struct {
	Transcription []whisperSegment `json:"transcription"`
}

type whisperSegment struct {
	Timestamps whisperTimestamps `json:"timestamps"`
	Text       string            `json:"text"`
}

type whisperTimestamps struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// TranscribeFile runs whisper-cli on the given WAV file and returns parsed segments.
func (t *Transcriber) TranscribeFile(ctx context.Context, wavPath string) ([]Segment, error) {
	args := []string{
		"-m", t.modelPath,
		"-t", strconv.Itoa(t.threads),
		"-l", t.language,
		"--output-json",
		"--prompt", "Dungeons and Dragons RPG session with fantasy names and places",
		"-f", wavPath,
	}
	if t.gpu {
		args = append(args, "--gpu")
	}

	cmd := exec.CommandContext(ctx, t.binaryPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("whisper-cli failed: %w: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("whisper-cli failed: %w", err)
	}

	var wo whisperOutput
	if err := json.Unmarshal(output, &wo); err != nil {
		return nil, fmt.Errorf("parse whisper-cli JSON output: %w", err)
	}

	segments := make([]Segment, 0, len(wo.Transcription))
	for _, ws := range wo.Transcription {
		startSec, err := parseTimestamp(ws.Timestamps.From)
		if err != nil {
			return nil, fmt.Errorf("parse start timestamp %q: %w", ws.Timestamps.From, err)
		}
		endSec, err := parseTimestamp(ws.Timestamps.To)
		if err != nil {
			return nil, fmt.Errorf("parse end timestamp %q: %w", ws.Timestamps.To, err)
		}
		segments = append(segments, Segment{
			StartTime: startSec,
			EndTime:   endSec,
			Text:      strings.TrimSpace(ws.Text),
		})
	}

	return segments, nil
}

// parseTimestamp converts a whisper-cli timestamp string "HH:MM:SS,mmm" to seconds.
func parseTimestamp(ts string) (float64, error) {
	// Format: "00:00:02,500" -> 2.5 seconds
	ts = strings.TrimSpace(ts)

	// Split on comma to separate seconds from milliseconds.
	parts := strings.SplitN(ts, ",", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected comma-separated timestamp, got %q", ts)
	}

	millis, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("parse milliseconds %q: %w", parts[1], err)
	}

	// Split HH:MM:SS
	hms := strings.SplitN(parts[0], ":", 3)
	if len(hms) != 3 {
		return 0, fmt.Errorf("expected HH:MM:SS, got %q", parts[0])
	}

	hours, err := strconv.Atoi(hms[0])
	if err != nil {
		return 0, fmt.Errorf("parse hours %q: %w", hms[0], err)
	}
	minutes, err := strconv.Atoi(hms[1])
	if err != nil {
		return 0, fmt.Errorf("parse minutes %q: %w", hms[1], err)
	}
	seconds, err := strconv.Atoi(hms[2])
	if err != nil {
		return 0, fmt.Errorf("parse seconds %q: %w", hms[2], err)
	}

	return float64(hours)*3600 + float64(minutes)*60 + float64(seconds) + float64(millis)/1000.0, nil
}
