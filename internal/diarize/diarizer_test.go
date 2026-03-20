package diarize

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// CosineSimilarity
// ---------------------------------------------------------------------------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	got := CosineSimilarity(a, a)
	if math.Abs(got-1.0) > 1e-6 {
		t.Errorf("identical vectors: got %f, want ~1.0", got)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	got := CosineSimilarity(a, b)
	if math.Abs(got) > 1e-6 {
		t.Errorf("orthogonal vectors: got %f, want ~0.0", got)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	got := CosineSimilarity(a, b)
	if math.Abs(got-(-1.0)) > 1e-6 {
		t.Errorf("opposite vectors: got %f, want ~-1.0", got)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("zero vector: got %f, want 0.0", got)
	}

	// Both zero.
	got = CosineSimilarity(a, a)
	if got != 0 {
		t.Errorf("both zero vectors: got %f, want 0.0", got)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("different lengths: got %f, want 0.0", got)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	got := CosineSimilarity(nil, nil)
	if got != 0 {
		t.Errorf("nil vectors: got %f, want 0.0", got)
	}

	got = CosineSimilarity([]float32{}, []float32{})
	if got != 0 {
		t.Errorf("empty vectors: got %f, want 0.0", got)
	}
}

// ---------------------------------------------------------------------------
// IdentifyPrimarySpeaker
// ---------------------------------------------------------------------------

func TestIdentifyPrimarySpeaker_Speaker0MoreTime(t *testing.T) {
	segments := []SpeakerSegment{
		{Start: 0, End: 10, Speaker: 0},
		{Start: 10, End: 15, Speaker: 1},
	}
	got := IdentifyPrimarySpeaker(segments)
	if got != 0 {
		t.Errorf("speaker 0 has more time: got %d, want 0", got)
	}
}

func TestIdentifyPrimarySpeaker_Speaker1MoreTime(t *testing.T) {
	segments := []SpeakerSegment{
		{Start: 0, End: 3, Speaker: 0},
		{Start: 3, End: 15, Speaker: 1},
	}
	got := IdentifyPrimarySpeaker(segments)
	if got != 1 {
		t.Errorf("speaker 1 has more time: got %d, want 1", got)
	}
}

func TestIdentifyPrimarySpeaker_EqualTime(t *testing.T) {
	segments := []SpeakerSegment{
		{Start: 0, End: 5, Speaker: 0},
		{Start: 5, End: 10, Speaker: 1},
	}
	got := IdentifyPrimarySpeaker(segments)
	// With equal time, the implementation picks whichever has strictly greater
	// time. Since map iteration is non-deterministic and times are equal,
	// either 0 or 1 is acceptable.
	if got != 0 && got != 1 {
		t.Errorf("equal time: got %d, want 0 or 1", got)
	}
}

func TestIdentifyPrimarySpeaker_SingleSpeaker(t *testing.T) {
	segments := []SpeakerSegment{
		{Start: 0, End: 5, Speaker: 0},
		{Start: 5, End: 10, Speaker: 0},
	}
	got := IdentifyPrimarySpeaker(segments)
	if got != 0 {
		t.Errorf("single speaker: got %d, want 0", got)
	}
}

func TestIdentifyPrimarySpeaker_EmptySegments(t *testing.T) {
	got := IdentifyPrimarySpeaker(nil)
	if got != 0 {
		t.Errorf("empty segments: got %d, want 0 (default)", got)
	}

	got = IdentifyPrimarySpeaker([]SpeakerSegment{})
	if got != 0 {
		t.Errorf("empty slice: got %d, want 0 (default)", got)
	}
}

// ---------------------------------------------------------------------------
// IdentifySpeakerByEmbedding
// ---------------------------------------------------------------------------

func TestIdentifySpeakerByEmbedding_BothEnrolled_CorrectMatch(t *testing.T) {
	// emb0 is close to ownerEmb, emb1 is close to partnerEmb.
	ownerEmb := []float32{1, 0, 0}
	partnerEmb := []float32{0, 1, 0}
	emb0 := []float32{0.9, 0.1, 0}
	emb1 := []float32{0.1, 0.9, 0}

	got := IdentifySpeakerByEmbedding(emb0, emb1, ownerEmb, partnerEmb)
	if got != 0 {
		t.Errorf("speaker 0 matches owner: got %d, want 0", got)
	}
}

func TestIdentifySpeakerByEmbedding_BothEnrolled_Swapped(t *testing.T) {
	// emb1 is close to ownerEmb, emb0 is close to partnerEmb.
	ownerEmb := []float32{1, 0, 0}
	partnerEmb := []float32{0, 1, 0}
	emb0 := []float32{0.1, 0.9, 0}
	emb1 := []float32{0.9, 0.1, 0}

	got := IdentifySpeakerByEmbedding(emb0, emb1, ownerEmb, partnerEmb)
	if got != 1 {
		t.Errorf("speaker 1 matches owner: got %d, want 1", got)
	}
}

func TestIdentifySpeakerByEmbedding_OnlyOwnerEnrolled(t *testing.T) {
	ownerEmb := []float32{1, 0, 0}
	emb0 := []float32{0.9, 0.1, 0}
	emb1 := []float32{0.1, 0.9, 0}

	got := IdentifySpeakerByEmbedding(emb0, emb1, ownerEmb, nil)
	if got != 0 {
		t.Errorf("only owner enrolled, speaker 0 closer: got %d, want 0", got)
	}

	// Swap: speaker 1 is closer to owner.
	got = IdentifySpeakerByEmbedding(emb1, emb0, ownerEmb, nil)
	if got != 1 {
		t.Errorf("only owner enrolled, speaker 1 closer: got %d, want 1", got)
	}
}

func TestIdentifySpeakerByEmbedding_OnlyPartnerEnrolled(t *testing.T) {
	partnerEmb := []float32{0, 1, 0}
	emb0 := []float32{0.1, 0.9, 0} // closer to partner
	emb1 := []float32{0.9, 0.1, 0}

	// Speaker 0 matches partner, so the owner is speaker 1.
	got := IdentifySpeakerByEmbedding(emb0, emb1, nil, partnerEmb)
	if got != 1 {
		t.Errorf("only partner enrolled, speaker 0 is partner: got %d, want 1", got)
	}

	// Swap: speaker 1 matches partner, owner is speaker 0.
	got = IdentifySpeakerByEmbedding(emb1, emb0, nil, partnerEmb)
	if got != 0 {
		t.Errorf("only partner enrolled, speaker 1 is partner: got %d, want 0", got)
	}
}

func TestIdentifySpeakerByEmbedding_NeitherEnrolled(t *testing.T) {
	emb0 := []float32{1, 0, 0}
	emb1 := []float32{0, 1, 0}

	got := IdentifySpeakerByEmbedding(emb0, emb1, nil, nil)
	if got != -1 {
		t.Errorf("no enrollments: got %d, want -1", got)
	}
}

func TestIdentifySpeakerByEmbedding_Ambiguous(t *testing.T) {
	// Both speakers have identical embeddings, and both enrollments are the
	// same embedding. The result is deterministic (sim00+sim11 >= sim10+sim01
	// since both sums are equal), so speaker 0 should be returned.
	emb := []float32{1, 1, 1}

	got := IdentifySpeakerByEmbedding(emb, emb, emb, emb)
	if got != 0 {
		t.Errorf("ambiguous (all equal): got %d, want 0 (>= tie-break)", got)
	}
}

// ---------------------------------------------------------------------------
// ExtractSpeakerAudio
// ---------------------------------------------------------------------------

func TestExtractSpeakerAudio_CorrectSpeaker(t *testing.T) {
	// 2 seconds of samples at 16kHz = 32000 samples.
	samples := make([]float32, 32000)
	for i := range samples {
		samples[i] = float32(i)
	}

	segments := []SpeakerSegment{
		{Start: 0.0, End: 1.0, Speaker: 0}, // samples [0, 16000)
		{Start: 1.0, End: 2.0, Speaker: 1}, // samples [16000, 32000)
	}

	audio0 := ExtractSpeakerAudio(samples, segments, 0)
	if len(audio0) != 16000 {
		t.Fatalf("speaker 0: got %d samples, want 16000", len(audio0))
	}
	if audio0[0] != 0 {
		t.Errorf("speaker 0 first sample: got %f, want 0", audio0[0])
	}

	audio1 := ExtractSpeakerAudio(samples, segments, 1)
	if len(audio1) != 16000 {
		t.Fatalf("speaker 1: got %d samples, want 16000", len(audio1))
	}
	if audio1[0] != 16000 {
		t.Errorf("speaker 1 first sample: got %f, want 16000", audio1[0])
	}
}

func TestExtractSpeakerAudio_SkipsOtherSpeakers(t *testing.T) {
	samples := make([]float32, 32000)
	segments := []SpeakerSegment{
		{Start: 0.0, End: 1.0, Speaker: 0},
		{Start: 1.0, End: 2.0, Speaker: 1},
	}

	audio := ExtractSpeakerAudio(samples, segments, 0)
	if len(audio) != 16000 {
		t.Errorf("expected 16000 samples for speaker 0, got %d", len(audio))
	}
}

func TestExtractSpeakerAudio_BoundaryPastSamples(t *testing.T) {
	// Only 8000 samples (0.5s at 16kHz), but segment goes to 1.0s.
	samples := make([]float32, 8000)
	for i := range samples {
		samples[i] = 1.0
	}

	segments := []SpeakerSegment{
		{Start: 0.0, End: 1.0, Speaker: 0},
	}

	audio := ExtractSpeakerAudio(samples, segments, 0)
	if len(audio) != 8000 {
		t.Errorf("boundary clamp: got %d samples, want 8000", len(audio))
	}
}

func TestExtractSpeakerAudio_SegmentStartPastSamples(t *testing.T) {
	samples := make([]float32, 16000) // 1 second

	segments := []SpeakerSegment{
		{Start: 2.0, End: 3.0, Speaker: 0}, // entirely past buffer
	}

	audio := ExtractSpeakerAudio(samples, segments, 0)
	if len(audio) != 0 {
		t.Errorf("segment past samples: got %d samples, want 0", len(audio))
	}
}

func TestExtractSpeakerAudio_EmptySegments(t *testing.T) {
	samples := make([]float32, 16000)
	audio := ExtractSpeakerAudio(samples, nil, 0)
	if len(audio) != 0 {
		t.Errorf("empty segments: got %d samples, want 0", len(audio))
	}
}

func TestExtractSpeakerAudio_MultipleSameSegments(t *testing.T) {
	// Speaker 0 has two non-contiguous segments.
	samples := make([]float32, 48000) // 3 seconds
	for i := range samples {
		samples[i] = float32(i)
	}

	segments := []SpeakerSegment{
		{Start: 0.0, End: 1.0, Speaker: 0},
		{Start: 1.0, End: 2.0, Speaker: 1},
		{Start: 2.0, End: 3.0, Speaker: 0},
	}

	audio := ExtractSpeakerAudio(samples, segments, 0)
	if len(audio) != 32000 {
		t.Fatalf("two segments for speaker 0: got %d samples, want 32000", len(audio))
	}
	// First portion starts at 0, second at sample 32000.
	if audio[0] != 0 {
		t.Errorf("first portion start: got %f, want 0", audio[0])
	}
	if audio[16000] != 32000 {
		t.Errorf("second portion start: got %f, want 32000", audio[16000])
	}
}

// ---------------------------------------------------------------------------
// UniqueSpeakers
// ---------------------------------------------------------------------------

func TestUniqueSpeakers_TwoSpeakers(t *testing.T) {
	segments := []SpeakerSegment{
		{Speaker: 0},
		{Speaker: 1},
		{Speaker: 0},
	}
	got := UniqueSpeakers(segments)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique speakers, got %d", len(got))
	}
	// Order is insertion order: 0, then 1.
	if got[0] != 0 || got[1] != 1 {
		t.Errorf("expected [0, 1], got %v", got)
	}
}

func TestUniqueSpeakers_SingleSpeaker(t *testing.T) {
	segments := []SpeakerSegment{
		{Speaker: 0},
		{Speaker: 0},
	}
	got := UniqueSpeakers(segments)
	if len(got) != 1 {
		t.Fatalf("expected 1 unique speaker, got %d", len(got))
	}
	if got[0] != 0 {
		t.Errorf("expected [0], got %v", got)
	}
}

func TestUniqueSpeakers_Empty(t *testing.T) {
	got := UniqueSpeakers(nil)
	if len(got) != 0 {
		t.Errorf("nil segments: expected empty, got %v", got)
	}

	got = UniqueSpeakers([]SpeakerSegment{})
	if len(got) != 0 {
		t.Errorf("empty segments: expected empty, got %v", got)
	}
}

func TestUniqueSpeakers_DuplicateIDs(t *testing.T) {
	segments := []SpeakerSegment{
		{Speaker: 2},
		{Speaker: 2},
		{Speaker: 2},
	}
	got := UniqueSpeakers(segments)
	if len(got) != 1 {
		t.Fatalf("expected 1 unique speaker, got %d", len(got))
	}
	if got[0] != 2 {
		t.Errorf("expected [2], got %v", got)
	}
}

// ---------------------------------------------------------------------------
// AttributeSegment
// ---------------------------------------------------------------------------

func TestAttributeSegment_SingleOverlap(t *testing.T) {
	diarSegments := []SpeakerSegment{
		{Start: 0, End: 5, Speaker: 0},
		{Start: 5, End: 10, Speaker: 1},
	}

	// Query is entirely within speaker 0's range.
	got := AttributeSegment(1.0, 4.0, diarSegments)
	if got != 0 {
		t.Errorf("overlap with speaker 0: got %d, want 0", got)
	}

	// Query is entirely within speaker 1's range.
	got = AttributeSegment(6.0, 9.0, diarSegments)
	if got != 1 {
		t.Errorf("overlap with speaker 1: got %d, want 1", got)
	}
}

func TestAttributeSegment_TwoOverlaps_PicksLarger(t *testing.T) {
	diarSegments := []SpeakerSegment{
		{Start: 0, End: 5, Speaker: 0},
		{Start: 5, End: 10, Speaker: 1},
	}

	// Query spans 3..7: overlap with speaker 0 is 2s (3-5), with speaker 1 is 2s (5-7).
	// Equal overlap: speaker 0 is found first and held, speaker 1 not strictly greater.
	got := AttributeSegment(3.0, 7.0, diarSegments)
	if got != 0 {
		t.Errorf("equal overlap (first wins): got %d, want 0", got)
	}

	// Query spans 4..9: overlap with speaker 0 is 1s (4-5), with speaker 1 is 4s (5-9).
	got = AttributeSegment(4.0, 9.0, diarSegments)
	if got != 1 {
		t.Errorf("speaker 1 has more overlap: got %d, want 1", got)
	}
}

func TestAttributeSegment_NoOverlap(t *testing.T) {
	diarSegments := []SpeakerSegment{
		{Start: 0, End: 5, Speaker: 0},
	}

	// Query is entirely after the diarization segment.
	got := AttributeSegment(10.0, 15.0, diarSegments)
	// No positive overlap: returns default speaker 0.
	if got != 0 {
		t.Errorf("no overlap: got %d, want 0 (default)", got)
	}
}

func TestAttributeSegment_EmptyDiarSegments(t *testing.T) {
	got := AttributeSegment(0, 5, nil)
	if got != 0 {
		t.Errorf("nil diar segments: got %d, want 0 (default)", got)
	}
}
