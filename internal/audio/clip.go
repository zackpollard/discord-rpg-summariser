package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// MixClip extracts a time range [startSec, endSec) from the selected per-user
// WAV files, mixes them with peak normalization, and writes the result.
// joinOffsets maps userID to seconds of silence before their audio starts.
func MixClip(userFiles map[string]string, outputPath string, joinOffsets map[string]float64, startSec, endSec float64) error {
	if len(userFiles) == 0 {
		return fmt.Errorf("no input files")
	}
	if endSec <= startSec {
		return fmt.Errorf("invalid time range: %.1f-%.1f", startSec, endSec)
	}

	clipSamples := int64((endSec - startSec) * float64(mixSampleRate))
	if clipSamples == 0 {
		return fmt.Errorf("clip too short")
	}

	type fileInfo struct {
		path        string
		peak        float64
		dataStart   int64 // byte offset of PCM data in the WAV file
		skipSamples int64 // samples to skip from the start of the user's audio
		readSamples int64 // samples to read from the user's audio
		mixOffset   int64 // sample offset within the output mix buffer
	}

	files := make(map[string]*fileInfo)

	for userID, path := range userFiles {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}

		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("stat %s: %w", path, err)
		}

		if stat.Size() < wavHeaderSkip {
			f.Close()
			continue
		}

		totalSamples := (stat.Size() - wavHeaderSkip) / 2
		if totalSamples == 0 {
			f.Close()
			continue
		}

		// The user's audio starts at their join offset in absolute time.
		userOffset := joinOffsets[userID]

		// Compute what portion of this user's audio falls within [startSec, endSec).
		userStartAbs := userOffset                                              // when user's audio begins (absolute)
		userEndAbs := userOffset + float64(totalSamples)/float64(mixSampleRate) // when it ends

		// Overlap between clip range and user's audio range.
		overlapStart := math.Max(startSec, userStartAbs)
		overlapEnd := math.Min(endSec, userEndAbs)

		if overlapStart >= overlapEnd {
			f.Close()
			continue // no overlap
		}

		// Position within the user's WAV file to start reading.
		skipSec := overlapStart - userOffset
		skipSamp := int64(skipSec * float64(mixSampleRate))
		readSamp := int64((overlapEnd - overlapStart) * float64(mixSampleRate))

		// Position within the output mix buffer where this user's audio starts.
		mixOff := int64((overlapStart - startSec) * float64(mixSampleRate))

		// Scan for peak amplitude in the relevant range.
		seekPos := wavHeaderSkip + skipSamp*2
		if _, err := f.Seek(seekPos, io.SeekStart); err != nil {
			f.Close()
			return fmt.Errorf("seek %s: %w", path, err)
		}

		var peak float64
		buf := make([]byte, chunkSamples*2)
		remaining := readSamp
		for remaining > 0 {
			toRead := int64(chunkSamples)
			if remaining < toRead {
				toRead = remaining
			}
			n, err := f.Read(buf[:toRead*2])
			if n > 0 {
				samples := n / 2
				for i := 0; i < samples; i++ {
					s := int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
					amp := math.Abs(float64(s) / 32768.0)
					if amp > peak {
						peak = amp
					}
				}
				remaining -= int64(samples)
			}
			if err != nil {
				break
			}
		}

		f.Close()

		files[userID] = &fileInfo{
			path:        path,
			peak:        peak,
			dataStart:   seekPos,
			skipSamples: skipSamp,
			readSamples: readSamp,
			mixOffset:   mixOff,
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no audio data in the selected time range")
	}

	// Compute gain factors.
	gains := make(map[string]float64, len(files))
	for uid, fi := range files {
		if fi.peak > 0 {
			gains[uid] = peakTarget / fi.peak
		} else {
			gains[uid] = 1.0
		}
	}

	// Mix into output buffer.
	mixBuf := make([]float64, clipSamples)

	for uid, fi := range files {
		f, err := os.Open(fi.path)
		if err != nil {
			return fmt.Errorf("reopen %s: %w", fi.path, err)
		}

		if _, err := f.Seek(fi.dataStart, io.SeekStart); err != nil {
			f.Close()
			return fmt.Errorf("seek %s: %w", fi.path, err)
		}

		gain := gains[uid]
		buf := make([]byte, chunkSamples*2)
		offset := fi.mixOffset
		remaining := fi.readSamples

		for remaining > 0 {
			toRead := int64(chunkSamples)
			if remaining < toRead {
				toRead = remaining
			}
			n, err := io.ReadFull(f, buf[:toRead*2])
			if n > 0 {
				samples := n / 2
				for i := 0; i < samples; i++ {
					s := int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
					if offset+int64(i) < clipSamples {
						mixBuf[offset+int64(i)] += float64(s) / 32768.0 * gain
					}
				}
				offset += int64(samples)
				remaining -= int64(samples)
			}
			if err != nil {
				break
			}
		}

		f.Close()
	}

	// Write output WAV.
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	dataSize := uint32(clipSamples * 2)
	if err := writeWAVHeader(outFile, dataSize); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	outBuf := make([]byte, 2)
	for i := int64(0); i < clipSamples; i++ {
		clamped := mixBuf[i]
		if clamped > 1.0 {
			clamped = 1.0
		} else if clamped < -1.0 {
			clamped = -1.0
		}
		binary.LittleEndian.PutUint16(outBuf, uint16(int16(clamped*32767.0)))
		if _, err := outFile.Write(outBuf); err != nil {
			return fmt.Errorf("write sample: %w", err)
		}
	}

	return nil
}
