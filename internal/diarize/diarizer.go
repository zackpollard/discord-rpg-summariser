// Package diarize provides speaker diarization using sherpa-onnx to split
// shared-microphone audio into per-speaker segments.
package diarize

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const (
	segmentationModelURL  = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/sherpa-onnx-pyannote-segmentation-3-0.tar.bz2"
	embeddingModelURL     = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/3dspeaker_speech_eres2net_base_sv_zh-cn_3dspeaker_16k.onnx"
	segmentationModelName = "sherpa-onnx-pyannote-segmentation-3-0"
	embeddingModelFile    = "3dspeaker_speech_eres2net_base_sv_zh-cn_3dspeaker_16k.onnx"
)

// SpeakerSegment represents a time range attributed to a speaker.
type SpeakerSegment struct {
	Start   float64 // seconds
	End     float64 // seconds
	Speaker int     // 0-based speaker ID
}

// Diarizer performs speaker diarization using sherpa-onnx.
type Diarizer struct {
	sd *sherpa.OfflineSpeakerDiarization
	mu sync.Mutex
}

// NewDiarizer creates a new diarizer, downloading models if needed.
func NewDiarizer(modelDir string, threads int) (*Diarizer, error) {
	segModelPath := filepath.Join(modelDir, segmentationModelName, "model.onnx")
	embModelPath := filepath.Join(modelDir, embeddingModelFile)

	// Download models if needed.
	if _, err := os.Stat(segModelPath); os.IsNotExist(err) {
		if err := downloadAndExtractTarBz2(segmentationModelURL, modelDir); err != nil {
			return nil, fmt.Errorf("download segmentation model: %w", err)
		}
	}
	if _, err := os.Stat(embModelPath); os.IsNotExist(err) {
		if err := downloadFile(embeddingModelURL, embModelPath); err != nil {
			return nil, fmt.Errorf("download embedding model: %w", err)
		}
	}

	config := sherpa.OfflineSpeakerDiarizationConfig{}
	config.Segmentation.Pyannote.Model = segModelPath
	config.Segmentation.NumThreads = threads
	config.Embedding.Model = embModelPath
	config.Embedding.NumThreads = threads
	// We always know there are exactly 2 speakers for shared mic.
	config.Clustering.NumClusters = 2

	sd := sherpa.NewOfflineSpeakerDiarization(&config)
	if sd == nil {
		return nil, fmt.Errorf("failed to create speaker diarization (check model paths)")
	}

	log.Printf("diarize: initialized with segmentation=%s embedding=%s", segModelPath, embModelPath)
	return &Diarizer{sd: sd}, nil
}

// Close releases the sherpa-onnx resources.
func (d *Diarizer) Close() {
	if d.sd != nil {
		sherpa.DeleteOfflineSpeakerDiarization(d.sd)
		d.sd = nil
	}
}

// Diarize processes 16kHz mono float32 audio and returns speaker segments.
func (d *Diarizer) Diarize(samples []float32) ([]SpeakerSegment, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := d.sd.Process(samples)

	segments := make([]SpeakerSegment, len(result))
	for i, seg := range result {
		segments[i] = SpeakerSegment{
			Start:   float64(seg.Start),
			End:     float64(seg.End),
			Speaker: seg.Speaker,
		}
	}

	return segments, nil
}

// SampleRate returns the expected sample rate (16000).
func (d *Diarizer) SampleRate() int {
	return d.sd.SampleRate()
}

// IdentifyDMSpeaker returns the speaker ID (0 or 1) most likely to be the DM,
// based on total speaking time. The DM typically speaks far more than any
// single player in an RPG session.
func IdentifyDMSpeaker(segments []SpeakerSegment) int {
	speakTime := make(map[int]float64)
	for _, seg := range segments {
		speakTime[seg.Speaker] += seg.End - seg.Start
	}

	bestSpeaker := 0
	bestTime := 0.0
	for speaker, t := range speakTime {
		if t > bestTime {
			bestSpeaker = speaker
			bestTime = t
		}
	}
	return bestSpeaker
}

// AttributeSegment finds the diarization speaker with the most overlap for a
// given time range (e.g., a whisper transcription segment).
func AttributeSegment(start, end float64, diarSegments []SpeakerSegment) int {
	bestSpeaker := 0
	bestOverlap := 0.0

	for _, ds := range diarSegments {
		overlapStart := start
		if ds.Start > overlapStart {
			overlapStart = ds.Start
		}
		overlapEnd := end
		if ds.End < overlapEnd {
			overlapEnd = ds.End
		}
		overlap := overlapEnd - overlapStart
		if overlap > bestOverlap {
			bestOverlap = overlap
			bestSpeaker = ds.Speaker
		}
	}
	return bestSpeaker
}

func downloadFile(url, destPath string) error {
	log.Printf("diarize: downloading %s...", url)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	n, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	log.Printf("diarize: downloaded %s (%d MB)", filepath.Base(destPath), n/1024/1024)
	return nil
}

func downloadAndExtractTarBz2(url, destDir string) error {
	log.Printf("diarize: downloading and extracting %s...", url)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	return extractTarBz2(resp.Body, destDir)
}
