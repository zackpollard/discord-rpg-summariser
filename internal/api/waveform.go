package api

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"discord-rpg-summariser/internal/audio"

	"github.com/jackc/pgx/v5"
)

// handleGetSessionWaveform returns a JSON array of peak amplitude values
// for the session's mixed audio. This is much smaller than the full WAV
// file, making it suitable for rendering waveforms in the browser.
func (s *Server) handleGetSessionWaveform(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get session")
		return
	}

	if sess.AudioDir == "" {
		writeError(w, http.StatusNotFound, "no audio directory for session")
		return
	}

	mixedPath := filepath.Join(sess.AudioDir, "mixed.wav")
	if _, err := os.Stat(mixedPath); os.IsNotExist(err) {
		// Generate on demand for older sessions that don't have a cached mix.
		if err := audio.MixFromDir(sess.AudioDir, mixedPath); err != nil {
			log.Printf("waveform: generate mix: %v", err)
			writeError(w, http.StatusNotFound, "mixed audio not available")
			return
		}
	}

	startSec, endSec, parseErr := parseWaveformRange(r)
	if parseErr != nil {
		writeError(w, http.StatusBadRequest, parseErr.Error())
		return
	}

	numPeaks := desiredWaveformPeaks(r, mixedPath, startSec, endSec)

	peaks, fullDuration, err := computePeaksRange(mixedPath, numPeaks, startSec, endSec)
	if err != nil {
		log.Printf("compute waveform peaks: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to compute waveform")
		return
	}

	w.Header().Set("Cache-Control", "max-age=3600")
	writeJSON(w, http.StatusOK, waveformResponse{
		Peaks:         peaks,
		StartSec:      startSec,
		EndSec:        endSec,
		FullDuration:  fullDuration,
	})
}

type waveformResponse struct {
	Peaks        []float64 `json:"peaks"`
	StartSec     float64   `json:"start_sec"`
	EndSec       float64   `json:"end_sec"`
	FullDuration float64   `json:"full_duration_sec"`
}

// parseWaveformRange reads optional ?start_sec=X&end_sec=Y query params.
// Returns (0, 0, nil) if no range was requested (full file).
func parseWaveformRange(r *http.Request) (float64, float64, error) {
	q := r.URL.Query()
	s, e := q.Get("start_sec"), q.Get("end_sec")
	if s == "" && e == "" {
		return 0, 0, nil
	}
	startSec, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start_sec")
	}
	endSec, err := strconv.ParseFloat(e, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end_sec")
	}
	if endSec <= startSec {
		return 0, 0, fmt.Errorf("end_sec must be > start_sec")
	}
	return startSec, endSec, nil
}

// desiredWaveformPeaks decides how many peak buckets to return. Client can
// force via ?peaks=N. Otherwise default to ~40 peaks/sec for the requested
// range (fine enough to spot word boundaries at zoom) capped at 100000.
func desiredWaveformPeaks(r *http.Request, path string, startSec, endSec float64) int {
	if q := r.URL.Query().Get("peaks"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 200000 {
			return n
		}
	}
	dur := endSec - startSec
	if dur <= 0 {
		if stat, err := os.Stat(path); err == nil && stat.Size() > 44 {
			dur = float64(stat.Size()-44) / (48000 * 2)
		}
	}
	// ~40 peaks/sec at zoom — fine enough to see word boundaries.
	n := int(dur * 40)
	if n < 2000 {
		n = 2000
	}
	if n > 100000 {
		n = 100000
	}
	return n
}

// computePeaks reads a 48kHz 16-bit mono WAV file and returns numPeaks
// amplitude values for the full file.
func computePeaks(wavPath string, numPeaks int) ([]float64, error) {
	peaks, _, err := computePeaksRange(wavPath, numPeaks, 0, 0)
	return peaks, err
}

// computePeaksRange reads a 48kHz 16-bit mono WAV file and returns numPeaks
// amplitude values (0.0-1.0) for the requested time range (in seconds).
// When endSec <= startSec, the full file is used. Also returns the full
// file duration in seconds so clients can position the range on a timeline.
func computePeaksRange(wavPath string, numPeaks int, startSec, endSec float64) ([]float64, float64, error) {
	f, err := os.Open(wavPath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}

	const headerSize = 44
	if stat.Size() < headerSize {
		return nil, 0, nil
	}

	const bytesPerSample = 2
	const sampleRate = 48000
	dataSize := stat.Size() - headerSize
	totalSamples := dataSize / bytesPerSample
	fullDuration := float64(totalSamples) / sampleRate

	// Compute byte range within the data chunk.
	startByte := int64(headerSize)
	rangeBytes := dataSize
	if endSec > startSec && endSec > 0 {
		startByte = headerSize + int64(startSec*sampleRate)*bytesPerSample
		endByte := headerSize + int64(endSec*sampleRate)*bytesPerSample
		if startByte < headerSize {
			startByte = headerSize
		}
		if endByte > stat.Size() {
			endByte = stat.Size()
		}
		if endByte <= startByte {
			return []float64{}, fullDuration, nil
		}
		rangeBytes = endByte - startByte
	}
	rangeSamples := rangeBytes / bytesPerSample
	if rangeSamples == 0 {
		return []float64{}, fullDuration, nil
	}

	samplesPerPeak := int(rangeSamples) / numPeaks
	if samplesPerPeak < 1 {
		samplesPerPeak = 1
		numPeaks = int(rangeSamples)
	}

	if _, err := f.Seek(startByte, 0); err != nil {
		return nil, fullDuration, err
	}

	peaks := make([]float64, numPeaks)
	buf := make([]byte, samplesPerPeak*bytesPerSample)

	for i := 0; i < numPeaks; i++ {
		n, err := f.Read(buf)
		if n == 0 {
			break
		}
		samples := n / bytesPerSample
		var maxAmp float64
		for j := 0; j < samples; j++ {
			s := int16(binary.LittleEndian.Uint16(buf[j*2 : j*2+2]))
			amp := math.Abs(float64(s) / 32768.0)
			if amp > maxAmp {
				maxAmp = amp
			}
		}
		peaks[i] = maxAmp
		if err != nil {
			break
		}
	}

	return peaks, fullDuration, nil
}
