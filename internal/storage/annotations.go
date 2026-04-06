package storage

import (
	"context"
	"fmt"
	"time"
)

type TranscriptAnnotation struct {
	ID             int64
	SegmentID      int64
	SessionID      int64
	Classification string  // narrative, table_talk, ambiguous
	CorrectedText  *string // nil if no correction needed
	Scene          *string // scene label, nil if same as previous
	NPCVoice       *string // NPC name if DM is voicing, nil otherwise
	MergeWithNext  bool    // true if this segment continues into the next
	Tone           *string // emotional tone: dramatic, funny, tense, etc.
	CreatedAt      time.Time
}

func (s *Store) InsertAnnotations(ctx context.Context, annotations []TranscriptAnnotation) error {
	if len(annotations) == 0 {
		return nil
	}

	for _, a := range annotations {
		_, err := s.Pool.Exec(ctx,
			`INSERT INTO transcript_annotations (segment_id, session_id, classification, corrected_text, scene, npc_voice, merge_with_next, tone)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (segment_id) DO UPDATE SET
			   classification = EXCLUDED.classification,
			   corrected_text = EXCLUDED.corrected_text,
			   scene = EXCLUDED.scene,
			   npc_voice = EXCLUDED.npc_voice,
			   merge_with_next = EXCLUDED.merge_with_next,
			   tone = EXCLUDED.tone`,
			a.SegmentID, a.SessionID, a.Classification, a.CorrectedText, a.Scene, a.NPCVoice, a.MergeWithNext, a.Tone,
		)
		if err != nil {
			return fmt.Errorf("insert annotation: %w", err)
		}
	}

	return nil
}

func (s *Store) GetAnnotations(ctx context.Context, sessionID int64) ([]TranscriptAnnotation, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, segment_id, session_id, classification, corrected_text, scene, npc_voice, merge_with_next, tone, created_at
		 FROM transcript_annotations WHERE session_id = $1 ORDER BY segment_id`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get annotations: %w", err)
	}
	defer rows.Close()

	var annotations []TranscriptAnnotation
	for rows.Next() {
		var a TranscriptAnnotation
		if err := rows.Scan(&a.ID, &a.SegmentID, &a.SessionID, &a.Classification, &a.CorrectedText, &a.Scene, &a.NPCVoice, &a.MergeWithNext, &a.Tone, &a.CreatedAt); err != nil {
			return nil, err
		}
		annotations = append(annotations, a)
	}
	return annotations, rows.Err()
}

func (s *Store) DeleteAnnotations(ctx context.Context, sessionID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM transcript_annotations WHERE session_id = $1`, sessionID)
	return err
}
