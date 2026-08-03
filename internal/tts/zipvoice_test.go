package tts

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildWAV assembles a RIFF/WAVE file from the given chunks (id + body).
func buildWAV(chunks ...[]byte) []byte {
	var body []byte
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := make([]byte, 12)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(4+len(body)))
	copy(out[8:12], "WAVE")
	return append(out, body...)
}

func chunk(id string, body []byte) []byte {
	out := make([]byte, 8)
	copy(out[0:4], id)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	out = append(out, body...)
	if len(body)%2 == 1 {
		out = append(out, 0) // word alignment padding
	}
	return out
}

func fmtChunk(format, channels, bits uint16, rate uint32) []byte {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body[0:2], format)
	binary.LittleEndian.PutUint16(body[2:4], channels)
	binary.LittleEndian.PutUint32(body[4:8], rate)
	binary.LittleEndian.PutUint32(body[8:12], rate*uint32(channels)*uint32(bits)/8)
	binary.LittleEndian.PutUint16(body[12:14], channels*bits/8)
	binary.LittleEndian.PutUint16(body[14:16], bits)
	return chunk("fmt ", body)
}

func writeTemp(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write temp wav: %v", err)
	}
	return path
}

// TestLoadWAV_SkipsExtraChunks covers the torchaudio/ffmpeg output layout,
// which puts a LIST/INFO chunk between fmt and data.
func TestLoadWAV_SkipsExtraChunks(t *testing.T) {
	pcm := []byte{0x00, 0x00, 0x00, 0x40, 0x00, 0xC0} // 0, +0.5, -0.5
	data := buildWAV(
		fmtChunk(wavFormatPCM, 1, 16, 24000),
		chunk("LIST", []byte("INFOISFTLavf58.76.100\x00")),
		chunk("data", pcm),
	)

	samples, rate, err := loadWAV(writeTemp(t, data))
	if err != nil {
		t.Fatalf("loadWAV: %v", err)
	}
	if rate != 24000 {
		t.Errorf("sample rate: got %d, want 24000", rate)
	}
	if len(samples) != 3 {
		t.Fatalf("samples: got %d, want 3", len(samples))
	}
	if samples[0] != 0 {
		t.Errorf("samples[0]: got %v, want 0 (no header bytes decoded as audio)", samples[0])
	}
	if samples[1] != 0.5 || samples[2] != -0.5 {
		t.Errorf("samples: got %v, want [0 0.5 -0.5]", samples)
	}
}

// TestLoadWAV_TrailingChunk verifies chunks after data are not decoded.
func TestLoadWAV_TrailingChunk(t *testing.T) {
	data := buildWAV(
		fmtChunk(wavFormatPCM, 1, 16, 24000),
		chunk("data", []byte{0x00, 0x40, 0x00, 0x40}),
		chunk("LIST", []byte("INFO")),
	)

	samples, _, err := loadWAV(writeTemp(t, data))
	if err != nil {
		t.Fatalf("loadWAV: %v", err)
	}
	if len(samples) != 2 {
		t.Errorf("samples: got %d, want 2", len(samples))
	}
}

// TestLoadWAV_RejectsFloat32 covers the soundfile fallback, which writes
// 32-bit IEEE float audio that must not be decoded as 16-bit PCM.
func TestLoadWAV_RejectsFloat32(t *testing.T) {
	data := buildWAV(
		fmtChunk(3, 1, 32, 24000),
		chunk("data", make([]byte, 16)),
	)

	if _, _, err := loadWAV(writeTemp(t, data)); err == nil {
		t.Error("expected an error for 32-bit float wav, got nil")
	}
}

func TestLoadWAV_Invalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"too short", []byte("RIFF")},
		{"not riff", make([]byte, 64)},
		{"no data chunk", buildWAV(fmtChunk(wavFormatPCM, 1, 16, 24000))},
		{"no fmt chunk", buildWAV(chunk("data", []byte{0x00, 0x00}))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := loadWAV(writeTemp(t, tc.data)); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// TestLoadWAV_RoundTrip checks the loader against this package's own writer.
func TestLoadWAV_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roundtrip.wav")
	want := []float32{0, 0.25, -0.25, 0.75}
	if err := WriteWAV(path, want, 24000); err != nil {
		t.Fatalf("WriteWAV: %v", err)
	}

	got, rate, err := loadWAV(path)
	if err != nil {
		t.Fatalf("loadWAV: %v", err)
	}
	if rate != 24000 {
		t.Errorf("sample rate: got %d, want 24000", rate)
	}
	if len(got) != len(want) {
		t.Fatalf("samples: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if diff := got[i] - want[i]; diff > 0.001 || diff < -0.001 {
			t.Errorf("samples[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}
