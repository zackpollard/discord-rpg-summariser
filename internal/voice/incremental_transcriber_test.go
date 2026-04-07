package voice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"discord-rpg-summariser/internal/transcribe"
)

// mockTranscriber implements transcribe.Transcriber for testing.
type mockTranscriber struct {
	chunkCalls int
	fileCalls  int
}

func (m *mockTranscriber) TranscribeFile(ctx context.Context, wavPath string) ([]transcribe.Segment, error) {
	m.fileCalls++
	return []transcribe.Segment{
		{StartTime: 0, EndTime: 10, Text: "test segment"},
	}, nil
}

func (m *mockTranscriber) TranscribeChunk(ctx context.Context, samples []float32, timeOffset time.Duration, prompt string) ([]transcribe.Segment, error) {
	m.chunkCalls++
	offset := timeOffset.Seconds()
	return []transcribe.Segment{
		{StartTime: offset, EndTime: offset + 5, Text: "chunk segment"},
	}, nil
}

func (m *mockTranscriber) SetGameSystem(system string)  {}
func (m *mockTranscriber) SetVocabulary(words []string) {}
func (m *mockTranscriber) Close() error                 { return nil }

func TestNewIncrementalTranscriber(t *testing.T) {
	dir := t.TempDir()
	mock := &mockTranscriber{}
	it := NewIncrementalTranscriber(mock, dir, 1)

	if it == nil {
		t.Fatal("expected non-nil transcriber")
	}
	if it.outputDir != dir {
		t.Errorf("outputDir = %q, want %q", it.outputDir, dir)
	}
	if it.sessionID != 1 {
		t.Errorf("sessionID = %d, want 1", it.sessionID)
	}
}

func TestIncrementalTranscriber_AddUser(t *testing.T) {
	dir := t.TempDir()
	mock := &mockTranscriber{}
	it := NewIncrementalTranscriber(mock, dir, 1)

	it.AddUser("user1", filepath.Join(dir, "user1.wav"))

	it.mu.Lock()
	defer it.mu.Unlock()
	if _, ok := it.userFiles["user1"]; !ok {
		t.Error("user1 should be registered")
	}
}

func TestIncrementalTranscriber_CollectedSegments_Empty(t *testing.T) {
	dir := t.TempDir()
	mock := &mockTranscriber{}
	it := NewIncrementalTranscriber(mock, dir, 1)

	segs, offsets := it.CollectedSegments()
	if len(segs) != 0 {
		t.Errorf("expected 0 segments, got %d", len(segs))
	}
	if len(offsets) != 0 {
		t.Errorf("expected 0 offsets, got %d", len(offsets))
	}
}

func TestIncrementalTranscriber_DiscoverFiles(t *testing.T) {
	dir := t.TempDir()
	mock := &mockTranscriber{}
	it := NewIncrementalTranscriber(mock, dir, 1)

	// Create some WAV files.
	os.WriteFile(filepath.Join(dir, "user1.wav"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(dir, "user2.wav"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(dir, "mixed.wav"), make([]byte, 100), 0644) // should be skipped
	os.WriteFile(filepath.Join(dir, "offsets.json"), []byte("{}"), 0644)   // should be skipped
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0644)      // should be skipped

	it.discoverFiles()

	it.mu.Lock()
	defer it.mu.Unlock()
	if len(it.userFiles) != 2 {
		t.Errorf("expected 2 user files, got %d: %v", len(it.userFiles), it.userFiles)
	}
	if _, ok := it.userFiles["user1"]; !ok {
		t.Error("user1 should be discovered")
	}
	if _, ok := it.userFiles["user2"]; !ok {
		t.Error("user2 should be discovered")
	}
	if _, ok := it.userFiles["mixed"]; ok {
		t.Error("mixed should be skipped")
	}
}

func TestIncrementalTranscriber_SkipsSmallFiles(t *testing.T) {
	dir := t.TempDir()
	mock := &mockTranscriber{}
	it := NewIncrementalTranscriber(mock, dir, 1)

	// Create a WAV file that's too small (< 90s at 48kHz).
	smallSize := 10 * 48000 * 2 // 10 seconds
	os.WriteFile(filepath.Join(dir, "user1.wav"), make([]byte, smallSize+44), 0644)
	it.AddUser("user1", filepath.Join(dir, "user1.wav"))

	ctx := context.Background()
	it.processUser(ctx, "user1", filepath.Join(dir, "user1.wav"))

	// Should not have processed anything.
	segs, _ := it.CollectedSegments()
	if len(segs["user1"]) != 0 {
		t.Errorf("expected 0 segments for small file, got %d", len(segs["user1"]))
	}
}

func TestIncrementalTranscriber_StopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	mock := &mockTranscriber{}
	it := NewIncrementalTranscriber(mock, dir, 1)
	it.Start(context.Background())
	it.Stop()
	// Should not panic on second stop or collect.
	segs, offsets := it.CollectedSegments()
	if segs == nil {
		t.Error("segs should not be nil")
	}
	if offsets == nil {
		t.Error("offsets should not be nil")
	}
}
