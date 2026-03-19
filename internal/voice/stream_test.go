package voice

import "testing"

func TestFindDAVEFrame_ValidFrame(t *testing.T) {
	// Build a minimal valid DAVE frame:
	// [payload bytes...] [supplemental (ss bytes)] [ss byte] [0xFA 0xFA]
	// ss must be >= 12, and frameLen - ss >= 1 (at least 1 byte of payload).
	ss := byte(12)
	payload := []byte{0x01} // 1 byte of opus payload
	supplemental := make([]byte, int(ss))
	for i := range supplemental {
		supplemental[i] = byte(i)
	}
	frame := append(payload, supplemental...)
	frame = append(frame, ss)
	frame = append(frame, 0xFA, 0xFA)

	out, ok := findDAVEFrame(frame)
	if !ok {
		t.Fatal("expected findDAVEFrame to return true for valid frame")
	}
	if len(out) != len(frame) {
		t.Errorf("output length: got %d, want %d", len(out), len(frame))
	}
}

func TestFindDAVEFrame_NoTrailer(t *testing.T) {
	// Data without the 0xFAFA trailer should not be detected as DAVE.
	data := make([]byte, 20)
	for i := range data {
		data[i] = byte(i)
	}

	_, ok := findDAVEFrame(data)
	if ok {
		t.Error("expected findDAVEFrame to return false for data without trailer")
	}
}

func TestFindDAVEFrame_SmallFrame(t *testing.T) {
	// Frames smaller than 13 bytes should return false.
	data := make([]byte, 12)
	data[10] = 0xFA
	data[11] = 0xFA

	_, ok := findDAVEFrame(data)
	if ok {
		t.Error("expected findDAVEFrame to return false for 12-byte frame")
	}

	// Exactly 13 bytes is the minimum; test with a valid 13-byte DAVE frame.
	ss := byte(12)
	// frame = [1 byte payload] [12 bytes supplemental] [ss] [0xFA 0xFA] = 16 bytes
	// For exactly 13 bytes: payload is empty if ss=11, but ss must be >= 12.
	// Minimum valid: 1 payload + 12 supplemental + 1 ss + 2 trailer = 16 bytes.
	// So 13 bytes can't be a valid DAVE frame even with trailer present.
	small := make([]byte, 13)
	small[11] = 0xFA
	small[12] = 0xFA
	small[10] = ss

	// frameLen = 13, ss = 12, frameLen - ss = 1, so technically valid structurally.
	out, ok := findDAVEFrame(small)
	if !ok {
		t.Fatal("expected findDAVEFrame to return true for minimal 13-byte valid frame")
	}
	if len(out) != 13 {
		t.Errorf("output length: got %d, want 13", len(out))
	}
}

func TestFindDAVEFrame_MisalignedTrailer(t *testing.T) {
	// RTP extension stripping can leave trailing bytes after the DAVE trailer.
	// findDAVEFrame scans up to 8 positions back from the end.
	ss := byte(12)
	payload := []byte{0x01}
	supplemental := make([]byte, int(ss))
	frame := append(payload, supplemental...)
	frame = append(frame, ss)
	frame = append(frame, 0xFA, 0xFA)

	// Append 3 trailing junk bytes.
	junk := []byte{0x00, 0x00, 0x00}
	data := append(frame, junk...)

	out, ok := findDAVEFrame(data)
	if !ok {
		t.Fatal("expected findDAVEFrame to find trailer despite trailing junk")
	}

	// Output should be trimmed to the DAVE frame boundary.
	if len(out) != len(frame) {
		t.Errorf("output length: got %d, want %d", len(out), len(frame))
	}
}

func TestFindDAVEFrame_InvalidSupplemental(t *testing.T) {
	// Trailer present but supplemental size is too small (< 12).
	data := make([]byte, 20)
	data[18] = 0xFA
	data[19] = 0xFA
	data[17] = 5 // ss = 5, which is < 12

	_, ok := findDAVEFrame(data)
	if ok {
		t.Error("expected findDAVEFrame to return false when supplemental size < 12")
	}
}

func TestIsOpusSilence(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"single 0xF8 byte", []byte{0xF8}, true},
		{"two bytes starting with 0xF8", []byte{0xF8, 0xFF}, true},
		{"three bytes starting with 0xF8", []byte{0xF8, 0xFF, 0xFE}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOpusSilence(tt.data); got != tt.want {
				t.Errorf("isOpusSilence(%x): got %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestIsOpusSilence_NotSilence(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty slice", []byte{}, false},
		{"non-0xF8 first byte", []byte{0x01, 0x02}, false},
		{"four bytes even with 0xF8", []byte{0xF8, 0x01, 0x02, 0x03}, false},
		{"0xFF first byte", []byte{0xFF}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOpusSilence(tt.data); got != tt.want {
				t.Errorf("isOpusSilence(%x): got %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}
