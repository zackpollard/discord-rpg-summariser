// Package diarize provides speaker diarization using sherpa-onnx to split
// shared-microphone audio into per-speaker segments.
package diarize

import (
	"fmt"
	"io"
	"log"
	"math"
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

// Diarizer performs speaker diarization and speaker embedding extraction
// using sherpa-onnx.
type Diarizer struct {
	sd        *sherpa.OfflineSpeakerDiarization
	extractor *sherpa.SpeakerEmbeddingExtractor
	mu        sync.Mutex
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

	// Create a standalone embedding extractor for voice enrollment.
	extractorConfig := sherpa.SpeakerEmbeddingExtractorConfig{
		Model:      embModelPath,
		NumThreads: threads,
		Provider:   "cpu",
	}
	extractor := sherpa.NewSpeakerEmbeddingExtractor(&extractorConfig)
	if extractor == nil {
		sherpa.DeleteOfflineSpeakerDiarization(sd)
		return nil, fmt.Errorf("failed to create speaker embedding extractor")
	}

	log.Printf("diarize: initialized with segmentation=%s embedding=%s (dim=%d)", segModelPath, embModelPath, extractor.Dim())
	return &Diarizer{sd: sd, extractor: extractor}, nil
}

// Close releases the sherpa-onnx resources.
func (d *Diarizer) Close() {
	if d.extractor != nil {
		sherpa.DeleteSpeakerEmbeddingExtractor(d.extractor)
		d.extractor = nil
	}
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

// IdentifyPrimarySpeaker returns the speaker ID (0 or 1) with the most total
// speaking time. For shared microphones, the mic owner is assumed to be the
// primary speaker since their audio is typically captured more prominently.
func IdentifyPrimarySpeaker(segments []SpeakerSegment) int {
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

// ExtractEmbedding computes a speaker embedding from 16kHz mono float32 audio.
func (d *Diarizer) ExtractEmbedding(samples []float32) ([]float32, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("cannot extract embedding from empty audio")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	stream := d.extractor.CreateStream()
	defer sherpa.DeleteOnlineStream(stream)

	stream.AcceptWaveform(16000, samples)
	stream.InputFinished()

	if !d.extractor.IsReady(stream) {
		return nil, fmt.Errorf("not enough audio for embedding extraction")
	}

	return d.extractor.Compute(stream), nil
}

// ExtractSpeakerAudio concatenates all audio belonging to a specific diarized
// speaker from the full 16kHz sample buffer.
func ExtractSpeakerAudio(samples []float32, segments []SpeakerSegment, speakerID int) []float32 {
	var audio []float32
	for _, seg := range segments {
		if seg.Speaker != speakerID {
			continue
		}
		start := int(seg.Start * 16000)
		end := int(seg.End * 16000)
		if start >= len(samples) {
			continue
		}
		if end > len(samples) {
			end = len(samples)
		}
		audio = append(audio, samples[start:end]...)
	}
	return audio
}

// IdentifySpeakerByEmbedding determines which diarized speaker (0 or 1) is the
// mic owner by comparing diarized speaker embeddings against enrolled voice
// embeddings. Returns the speaker ID for the mic owner, or -1 if identification
// is not possible.
func IdentifySpeakerByEmbedding(emb0, emb1, micOwnerEmb, partnerEmb []float32) int {
	if micOwnerEmb != nil && partnerEmb != nil {
		// Both enrolled: find the assignment that maximises total similarity.
		sim00 := CosineSimilarity(emb0, micOwnerEmb) // speaker0=owner
		sim11 := CosineSimilarity(emb1, partnerEmb)  // speaker1=partner
		sim01 := CosineSimilarity(emb0, partnerEmb)  // speaker0=partner
		sim10 := CosineSimilarity(emb1, micOwnerEmb) // speaker1=owner

		if sim00+sim11 >= sim10+sim01 {
			return 0
		}
		return 1
	}

	if micOwnerEmb != nil {
		// Only mic owner enrolled.
		if CosineSimilarity(emb0, micOwnerEmb) >= CosineSimilarity(emb1, micOwnerEmb) {
			return 0
		}
		return 1
	}

	if partnerEmb != nil {
		// Only partner enrolled — find partner, return the other speaker.
		if CosineSimilarity(emb0, partnerEmb) >= CosineSimilarity(emb1, partnerEmb) {
			return 1 // speaker 0 is partner → speaker 1 is owner
		}
		return 0
	}

	return -1 // no enrollments
}

// CosineSimilarity computes the cosine similarity between two embedding vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// UniqueSpeakers returns the distinct speaker IDs in the segments.
func UniqueSpeakers(segments []SpeakerSegment) []int {
	seen := make(map[int]struct{})
	var speakers []int
	for _, seg := range segments {
		if _, ok := seen[seg.Speaker]; !ok {
			seen[seg.Speaker] = struct{}{}
			speakers = append(speakers, seg.Speaker)
		}
	}
	return speakers
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
