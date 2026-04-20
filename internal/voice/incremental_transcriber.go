package voice

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"discord-rpg-summariser/internal/audio"
	"discord-rpg-summariser/internal/transcribe"
)

// IncrementalTranscriber processes per-user WAV files during a live session,
// transcribing completed chunks as they become available rather than waiting
// until the session ends. This uses the same StreamResample chunking logic
// (silence-based splits) as the post-session pipeline.
type IncrementalTranscriber struct {
	transcriber transcribe.Transcriber
	outputDir   string
	sessionID   int64

	mu             sync.Mutex
	userFiles      map[string]string               // userID -> WAV path
	processedBytes map[string]int64                // userID -> bytes already processed
	segments       map[string][]transcribe.Segment // userID -> accumulated segments
	running        bool
	done           chan struct{}
	stopped        chan struct{} // closed when the background goroutine exits
	checkInterval  time.Duration
}

// NewIncrementalTranscriber creates a new incremental transcriber.
func NewIncrementalTranscriber(t transcribe.Transcriber, outputDir string, sessionID int64) *IncrementalTranscriber {
	return &IncrementalTranscriber{
		transcriber:    t,
		outputDir:      outputDir,
		sessionID:      sessionID,
		userFiles:      make(map[string]string),
		processedBytes: make(map[string]int64),
		segments:       make(map[string][]transcribe.Segment),
		done:           make(chan struct{}),
		stopped:        make(chan struct{}),
		checkInterval:  30 * time.Second, // check for new chunks every 30s
	}
}

// AddUser registers a user's WAV file for incremental transcription.
func (it *IncrementalTranscriber) AddUser(userID, wavPath string) {
	it.mu.Lock()
	defer it.mu.Unlock()
	it.userFiles[userID] = wavPath
}

// Start begins the background processing loop.
func (it *IncrementalTranscriber) Start(ctx context.Context) {
	it.mu.Lock()
	it.running = true
	it.mu.Unlock()

	go func() {
		defer close(it.stopped)
		ticker := time.NewTicker(it.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-it.done:
				// Final pass: transcribe any audio that arrived since
				// the last tick so the pipeline has less remainder work.
				it.processAll(ctx)
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				it.processAll(ctx)
			}
		}
	}()
}

// Stop signals the background loop to exit and waits for it to finish
// any in-flight transcription before returning.
func (it *IncrementalTranscriber) Stop() {
	it.mu.Lock()
	it.running = false
	it.mu.Unlock()
	close(it.done)
	<-it.stopped
}

// CollectedSegments returns all segments transcribed so far for each user,
// and the byte offset processed per user. The pipeline uses this to skip
// already-transcribed portions.
func (it *IncrementalTranscriber) CollectedSegments() (map[string][]transcribe.Segment, map[string]int64) {
	it.mu.Lock()
	defer it.mu.Unlock()

	segs := make(map[string][]transcribe.Segment, len(it.segments))
	for uid, s := range it.segments {
		segs[uid] = append([]transcribe.Segment{}, s...)
	}

	offsets := make(map[string]int64, len(it.processedBytes))
	for uid, off := range it.processedBytes {
		offsets[uid] = off
	}

	return segs, offsets
}

func (it *IncrementalTranscriber) processAll(ctx context.Context) {
	// Auto-discover user WAV files in the output directory.
	it.discoverFiles()

	it.mu.Lock()
	files := make(map[string]string, len(it.userFiles))
	for uid, path := range it.userFiles {
		files[uid] = path
	}
	it.mu.Unlock()

	for userID, wavPath := range files {
		it.processUser(ctx, userID, wavPath)
	}
}

func (it *IncrementalTranscriber) discoverFiles() {
	entries, err := os.ReadDir(it.outputDir)
	if err != nil {
		return
	}
	it.mu.Lock()
	defer it.mu.Unlock()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 5 || name[len(name)-4:] != ".wav" {
			continue
		}
		userID := name[:len(name)-4]
		if userID == "mixed" || userID == "offsets" {
			continue
		}
		if _, ok := it.userFiles[userID]; !ok {
			path := it.outputDir + "/" + name
			it.userFiles[userID] = path
			log.Printf("incremental: discovered user file %s", userID)
		}
	}
}

func (it *IncrementalTranscriber) processUser(ctx context.Context, userID, wavPath string) {
	it.mu.Lock()
	processedBytes := it.processedBytes[userID]
	it.mu.Unlock()

	// Check if the file has grown since last check.
	info, err := os.Stat(wavPath)
	if err != nil {
		return
	}

	currentSize := info.Size()
	// Only process when we have at least 90 seconds of new audio — matching
	// the maxChunkSamples parameter (90s) from StreamResample. This ensures
	// silence-based chunking works correctly and we don't create chunks that
	// are too short. Shorter chunks would lack context for the transcriber.
	minNewBytes := int64(90 * 48000 * 2) // 90s at 48kHz 16-bit mono
	if currentSize-processedBytes < minNewBytes {
		return
	}

	// Run StreamResample on the unprocessed portion by seeking past processed bytes.
	// We use the full file but track how many output chunks correspond to new data.
	newSegments, newProcessedBytes := it.transcribeFromOffset(ctx, wavPath, processedBytes)

	if len(newSegments) > 0 {
		it.mu.Lock()
		it.segments[userID] = append(it.segments[userID], newSegments...)
		it.processedBytes[userID] = newProcessedBytes
		it.mu.Unlock()

		log.Printf("incremental: transcribed %d new segments for %s (processed %.1fs)",
			len(newSegments), userID, float64(newProcessedBytes)/(48000*2))
	}
}

func (it *IncrementalTranscriber) transcribeFromOffset(
	ctx context.Context, wavPath string, startOffset int64,
) ([]transcribe.Segment, int64) {
	// Use StreamResample which handles the full file, but we only keep
	// segments that start after our processed offset.
	startTimeSec := float64(startOffset) / (48000 * 2)
	var newSegments []transcribe.Segment
	var lastEndBytes int64

	err := audio.StreamResample(wavPath, func(samples []float32, offsetSeconds float64) error {
		// Skip chunks we've already processed.
		if offsetSeconds < startTimeSec-1.0 { // small overlap for context
			return nil
		}

		segs, err := it.transcriber.TranscribeChunk(ctx, samples,
			time.Duration(offsetSeconds*float64(time.Second)), "")
		if err != nil {
			return err
		}

		for _, seg := range segs {
			// Only keep segments that start after our previous endpoint.
			if seg.StartTime >= startTimeSec {
				newSegments = append(newSegments, seg)
			}
		}

		// Track how far we've processed in bytes.
		endTimeSec := offsetSeconds + float64(len(samples))/16000*3 // approximate: 16kHz resampled, 3:1 from 48kHz
		lastEndBytes = int64(endTimeSec * 48000 * 2)

		return nil
	})

	if err != nil {
		log.Printf("incremental: stream resample error for %s: %v", wavPath, err)
		// Return what we have — don't update the offset on error.
		if len(newSegments) == 0 {
			return nil, startOffset
		}
	}

	// Don't process the last chunk — it's likely still being written to.
	// Remove the last segment and reduce the processed offset accordingly.
	if len(newSegments) > 1 {
		newSegments = newSegments[:len(newSegments)-1]
		lastSeg := newSegments[len(newSegments)-1]
		lastEndBytes = int64(lastSeg.EndTime * 48000 * 2)
	} else if len(newSegments) == 1 {
		// Only one segment — keep it but don't advance the offset much
		// since we might need to re-process this area.
		return nil, startOffset
	}

	return newSegments, lastEndBytes
}
