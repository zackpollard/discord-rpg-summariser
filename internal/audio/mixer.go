package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

const (
	mixSampleRate  = 48000
	mixBitsPerSamp = 16
	mixNumChannels = 1
	mixByteRate    = mixSampleRate * mixNumChannels * (mixBitsPerSamp / 8)
	mixBlockAlign  = mixNumChannels * (mixBitsPerSamp / 8)

	// chunkSamples is one second of audio at 48kHz.
	chunkSamples = 48000

	// peakTarget is the normalisation target — leave headroom to avoid clipping.
	peakTarget = 0.9
)

// MixAndNormalize takes per-user WAV file paths, mixes them into a single
// normalized WAV file at the given output path. Uses peak normalization
// per track so all speakers are at the same perceived volume.
func MixAndNormalize(userFiles map[string]string, outputPath string) error {
	if len(userFiles) == 0 {
		return fmt.Errorf("no input files")
	}

	// ---- First pass: find peak amplitude and total samples for each file ----
	type fileInfo struct {
		path      string
		peak      float64
		numSamps  int64
		dataStart int64
	}
	files := make(map[string]*fileInfo, len(userFiles))
	var maxSamples int64

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
			continue // skip empty / invalid files
		}

		dataSize := stat.Size() - wavHeaderSkip
		numSamps := dataSize / 2 // 16-bit samples

		if numSamps == 0 {
			f.Close()
			continue
		}

		// Scan for peak amplitude in chunks.
		if _, err := f.Seek(wavHeaderSkip, io.SeekStart); err != nil {
			f.Close()
			return fmt.Errorf("seek %s: %w", path, err)
		}

		var peak float64
		buf := make([]byte, chunkSamples*2)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				samples := n / 2
				for i := 0; i < samples; i++ {
					s := int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
					amp := math.Abs(float64(s) / 32768.0)
					if amp > peak {
						peak = amp
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				f.Close()
				return fmt.Errorf("read %s: %w", path, err)
			}
		}

		f.Close()

		files[userID] = &fileInfo{
			path:      path,
			peak:      peak,
			numSamps:  numSamps,
			dataStart: wavHeaderSkip,
		}

		if numSamps > maxSamples {
			maxSamples = numSamps
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no valid input files")
	}

	// Compute gain factors: normalise each track's peak to peakTarget.
	gains := make(map[string]float64, len(files))
	for userID, fi := range files {
		if fi.peak > 0 {
			gains[userID] = peakTarget / fi.peak
		} else {
			gains[userID] = 1.0
		}
	}

	// ---- Second pass: mix all tracks with gain applied ----
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	// Write a placeholder WAV header — we'll patch it after writing.
	if err := writeWAVHeader(outFile, 0); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Open all input files and seek past their headers.
	readers := make(map[string]*os.File, len(files))
	for userID, fi := range files {
		f, err := os.Open(fi.path)
		if err != nil {
			closeReaders(readers)
			return fmt.Errorf("open %s: %w", fi.path, err)
		}
		if _, err := f.Seek(fi.dataStart, io.SeekStart); err != nil {
			f.Close()
			closeReaders(readers)
			return fmt.Errorf("seek %s: %w", fi.path, err)
		}
		readers[userID] = f
	}
	defer closeReaders(readers)

	// Process in chunks.
	readBufs := make(map[string][]byte, len(readers))
	for uid := range readers {
		readBufs[uid] = make([]byte, chunkSamples*2)
	}
	outBuf := make([]byte, chunkSamples*2)

	var totalWritten int64
	for totalWritten < maxSamples {
		remaining := maxSamples - totalWritten
		thisBatch := int64(chunkSamples)
		if remaining < thisBatch {
			thisBatch = remaining
		}

		// Mix samples for this chunk.
		mixBuf := make([]float64, thisBatch)

		for userID, r := range readers {
			toRead := int(thisBatch) * 2
			n, err := io.ReadFull(r, readBufs[userID][:toRead])
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return fmt.Errorf("read chunk: %w", err)
			}
			samples := n / 2
			gain := gains[userID]
			for i := 0; i < samples; i++ {
				s := int16(binary.LittleEndian.Uint16(readBufs[userID][i*2 : i*2+2]))
				mixBuf[i] += float64(s) / 32768.0 * gain
			}
		}

		// Convert mixed float64 to int16 with clipping.
		for i := int64(0); i < thisBatch; i++ {
			clamped := mixBuf[i]
			if clamped > 1.0 {
				clamped = 1.0
			} else if clamped < -1.0 {
				clamped = -1.0
			}
			s := int16(clamped * 32767.0)
			binary.LittleEndian.PutUint16(outBuf[i*2:i*2+2], uint16(s))
		}

		if _, err := outFile.Write(outBuf[:thisBatch*2]); err != nil {
			return fmt.Errorf("write chunk: %w", err)
		}
		totalWritten += thisBatch
	}

	// Patch the WAV header with the correct data size.
	dataSize := uint32(totalWritten * 2)
	if _, err := outFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek to header: %w", err)
	}
	if err := writeWAVHeader(outFile, dataSize); err != nil {
		return fmt.Errorf("patch header: %w", err)
	}

	return nil
}

// writeWAVHeader writes a 44-byte WAV header for 48kHz 16-bit mono PCM.
func writeWAVHeader(w io.Writer, dataSize uint32) error {
	fileSize := 36 + dataSize

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], fileSize)
	copy(hdr[8:12], "WAVE")
	copy(hdr[12:16], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)                    // fmt chunk size
	binary.LittleEndian.PutUint16(hdr[20:22], 1)                     // PCM
	binary.LittleEndian.PutUint16(hdr[22:24], mixNumChannels)        // channels
	binary.LittleEndian.PutUint32(hdr[24:28], mixSampleRate)         // sample rate
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(mixByteRate))   // byte rate
	binary.LittleEndian.PutUint16(hdr[32:34], uint16(mixBlockAlign)) // block align
	binary.LittleEndian.PutUint16(hdr[34:36], mixBitsPerSamp)        // bits per sample
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize) // data size

	_, err := w.Write(hdr[:])
	return err
}

func closeReaders(readers map[string]*os.File) {
	for _, f := range readers {
		f.Close()
	}
}
