package api

import (
	"encoding/binary"
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

	// Scale peaks to ~10 per second of audio for good resolution at any zoom.
	// The client can also request a specific count via ?peaks=N.
	numPeaks := 5000 // default: good for up to ~8 min at full view
	if stat, err := os.Stat(mixedPath); err == nil && stat.Size() > 44 {
		durationSec := float64(stat.Size()-44) / (48000 * 2) // 48kHz 16-bit mono
		scaled := int(durationSec * 10)                       // ~10 peaks per second
		if scaled > numPeaks {
			numPeaks = scaled
		}
		if numPeaks > 50000 {
			numPeaks = 50000 // cap to keep response size reasonable
		}
	}
	if q := r.URL.Query().Get("peaks"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 100000 {
			numPeaks = n
		}
	}

	peaks, err := computePeaks(mixedPath, numPeaks)
	if err != nil {
		log.Printf("compute waveform peaks: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to compute waveform")
		return
	}

	w.Header().Set("Cache-Control", "max-age=3600")
	writeJSON(w, http.StatusOK, peaks)
}

// computePeaks reads a 48kHz 16-bit mono WAV file and returns numPeaks
// amplitude values (0.0-1.0), each representing the max absolute amplitude
// in that segment of the file.
func computePeaks(wavPath string, numPeaks int) ([]float64, error) {
	f, err := os.Open(wavPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	const headerSize = 44
	if stat.Size() < headerSize {
		return nil, nil
	}

	dataSize := stat.Size() - headerSize
	totalSamples := dataSize / 2
	samplesPerPeak := int(totalSamples) / numPeaks
	if samplesPerPeak < 1 {
		samplesPerPeak = 1
		numPeaks = int(totalSamples)
	}

	if _, err := f.Seek(headerSize, 0); err != nil {
		return nil, err
	}

	peaks := make([]float64, numPeaks)
	buf := make([]byte, samplesPerPeak*2)

	for i := 0; i < numPeaks; i++ {
		n, err := f.Read(buf)
		if n == 0 {
			break
		}

		samples := n / 2
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

	return peaks, nil
}
