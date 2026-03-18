package voice

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	wavSampleRate  = 48000
	wavNumChannels = 1
	wavBitsPerSamp = 16
	wavByteRate    = wavSampleRate * wavNumChannels * (wavBitsPerSamp / 8)
	wavBlockAlign  = wavNumChannels * (wavBitsPerSamp / 8)
	wavHeaderSize  = 44
)

// WAVWriter writes 48kHz, 16-bit PCM mono audio to a WAV file.
type WAVWriter struct {
	file     *os.File
	dataSize uint32
}

// NewWAVWriter creates a new WAV file at path and writes a placeholder header.
func NewWAVWriter(path string) (*WAVWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create wav file: %w", err)
	}

	w := &WAVWriter{file: f}

	if err := w.writeHeader(0); err != nil {
		f.Close()
		return nil, fmt.Errorf("write wav header: %w", err)
	}

	return w, nil
}

// Write encodes PCM samples as little-endian int16 and appends them to the data chunk.
func (w *WAVWriter) Write(samples []int16) error {
	if err := binary.Write(w.file, binary.LittleEndian, samples); err != nil {
		return fmt.Errorf("write pcm samples: %w", err)
	}
	w.dataSize += uint32(len(samples)) * 2
	return nil
}

// Close seeks back to update the WAV header with final sizes, then closes the file.
func (w *WAVWriter) Close() error {
	if err := w.writeHeader(w.dataSize); err != nil {
		w.file.Close()
		return fmt.Errorf("finalize wav header: %w", err)
	}
	return w.file.Close()
}

func (w *WAVWriter) writeHeader(dataSize uint32) error {
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	riffSize := uint32(wavHeaderSize - 8 + dataSize)

	var header [wavHeaderSize]byte

	// RIFF chunk descriptor
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], riffSize)
	copy(header[8:12], "WAVE")

	// fmt sub-chunk
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16) // sub-chunk size
	binary.LittleEndian.PutUint16(header[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(header[22:24], wavNumChannels)
	binary.LittleEndian.PutUint32(header[24:28], wavSampleRate)
	binary.LittleEndian.PutUint32(header[28:32], wavByteRate)
	binary.LittleEndian.PutUint16(header[32:34], wavBlockAlign)
	binary.LittleEndian.PutUint16(header[34:36], wavBitsPerSamp)

	// data sub-chunk
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	_, err := w.file.Write(header[:])
	return err
}
