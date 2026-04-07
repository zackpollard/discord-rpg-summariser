package tts

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WriteWAV writes float32 samples as a 16-bit mono PCM WAV file.
// Samples are expected in the range [-1, 1].
func WriteWAV(path string, samples []float32, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create wav: %w", err)
	}
	defer f.Close()

	dataSize := uint32(len(samples) * 2) // 16-bit = 2 bytes per sample
	if err := writeWAVHeader(f, dataSize, sampleRate); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	buf := make([]byte, 2)
	for _, s := range samples {
		// Clamp to [-1, 1].
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		binary.LittleEndian.PutUint16(buf, uint16(int16(s*32767.0)))
		if _, err := f.Write(buf); err != nil {
			return fmt.Errorf("write sample: %w", err)
		}
	}

	return nil
}

// writeWAVHeader writes a 44-byte WAV header for mono 16-bit PCM at the given sample rate.
func writeWAVHeader(w io.Writer, dataSize uint32, sampleRate int) error {
	const (
		numChannels = 1
		bitsPerSamp = 16
	)
	byteRate := uint32(sampleRate * numChannels * (bitsPerSamp / 8))
	blockAlign := uint16(numChannels * (bitsPerSamp / 8))
	fileSize := 36 + dataSize

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], fileSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)                 // fmt chunk size
	binary.LittleEndian.PutUint16(hdr[20:22], 1)                  // PCM
	binary.LittleEndian.PutUint16(hdr[22:24], numChannels)        // channels
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sampleRate)) // sample rate
	binary.LittleEndian.PutUint32(hdr[28:32], byteRate)           // byte rate
	binary.LittleEndian.PutUint16(hdr[32:34], blockAlign)         // block align
	binary.LittleEndian.PutUint16(hdr[34:36], bitsPerSamp)        // bits per sample
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize) // data size

	_, err := w.Write(hdr[:])
	return err
}
