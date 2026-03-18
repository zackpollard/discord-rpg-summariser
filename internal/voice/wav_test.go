package voice

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestWAVWriterProducesValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")

	w, err := NewWAVWriter(path)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}

	// Write 480 samples (10ms at 48kHz).
	samples := make([]int16, 480)
	for i := range samples {
		samples[i] = int16(i % 1000)
	}
	if err := w.Write(samples); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read back and verify.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(data) != wavHeaderSize+len(samples)*2 {
		t.Fatalf("file size: got %d, want %d", len(data), wavHeaderSize+len(samples)*2)
	}

	// RIFF header.
	if string(data[0:4]) != "RIFF" {
		t.Errorf("chunk ID: got %q, want RIFF", data[0:4])
	}
	riffSize := binary.LittleEndian.Uint32(data[4:8])
	wantRiffSize := uint32(len(data) - 8)
	if riffSize != wantRiffSize {
		t.Errorf("RIFF size: got %d, want %d", riffSize, wantRiffSize)
	}
	if string(data[8:12]) != "WAVE" {
		t.Errorf("format: got %q, want WAVE", data[8:12])
	}
}

func TestWAVHeaderValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "header.wav")

	w, err := NewWAVWriter(path)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}

	samples := make([]int16, 960)
	for i := range samples {
		samples[i] = int16(i)
	}
	if err := w.Write(samples); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// fmt chunk.
	if string(data[12:16]) != "fmt " {
		t.Errorf("sub-chunk1 ID: got %q, want \"fmt \"", data[12:16])
	}
	subChunk1Size := binary.LittleEndian.Uint32(data[16:20])
	if subChunk1Size != 16 {
		t.Errorf("sub-chunk1 size: got %d, want 16", subChunk1Size)
	}

	format := binary.LittleEndian.Uint16(data[20:22])
	if format != 1 {
		t.Errorf("audio format: got %d, want 1 (PCM)", format)
	}

	channels := binary.LittleEndian.Uint16(data[22:24])
	if channels != 1 {
		t.Errorf("channels: got %d, want 1", channels)
	}

	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	if sampleRate != 48000 {
		t.Errorf("sample rate: got %d, want 48000", sampleRate)
	}

	byteRate := binary.LittleEndian.Uint32(data[28:32])
	if byteRate != 96000 {
		t.Errorf("byte rate: got %d, want 96000", byteRate)
	}

	blockAlign := binary.LittleEndian.Uint16(data[32:34])
	if blockAlign != 2 {
		t.Errorf("block align: got %d, want 2", blockAlign)
	}

	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])
	if bitsPerSample != 16 {
		t.Errorf("bits per sample: got %d, want 16", bitsPerSample)
	}

	// data chunk.
	if string(data[36:40]) != "data" {
		t.Errorf("sub-chunk2 ID: got %q, want \"data\"", data[36:40])
	}

	dataSize := binary.LittleEndian.Uint32(data[40:44])
	wantDataSize := uint32(len(samples) * 2)
	if dataSize != wantDataSize {
		t.Errorf("data size: got %d, want %d", dataSize, wantDataSize)
	}
}

func TestWAVDataMatchesInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.wav")

	w, err := NewWAVWriter(path)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}

	samples := []int16{-32768, -1, 0, 1, 32767}
	if err := w.Write(samples); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	pcm := data[wavHeaderSize:]
	if len(pcm) != len(samples)*2 {
		t.Fatalf("pcm length: got %d, want %d", len(pcm), len(samples)*2)
	}

	for i, want := range samples {
		got := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		if got != want {
			t.Errorf("sample[%d]: got %d, want %d", i, got, want)
		}
	}
}
