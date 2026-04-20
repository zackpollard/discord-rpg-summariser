package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// TTSAudioCache stores metadata for a cached TTS audio file.
type TTSAudioCache struct {
	ID          int64
	CampaignID  int64
	Source      string // "recap" or "previously-on"
	VoiceKey    string // "user:{discordID}" or "profile:{profileID}"
	AudioPath   string
	GeneratedAt time.Time
}

// VoiceKeyForUser returns the voice key string for a Discord user ID.
func VoiceKeyForUser(userID string) string {
	return "user:" + userID
}

// VoiceKeyForProfile returns the voice key string for a voice profile ID.
func VoiceKeyForProfile(profileID int64) string {
	return fmt.Sprintf("profile:%d", profileID)
}

func (s *Store) GetTTSCache(ctx context.Context, campaignID int64, source, voiceKey string) (*TTSAudioCache, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, campaign_id, source, voice_key, audio_path, generated_at
		 FROM tts_audio_cache
		 WHERE campaign_id = $1 AND source = $2 AND voice_key = $3`,
		campaignID, source, voiceKey,
	)
	var c TTSAudioCache
	err := row.Scan(&c.ID, &c.CampaignID, &c.Source, &c.VoiceKey, &c.AudioPath, &c.GeneratedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tts cache: %w", err)
	}
	return &c, nil
}

func (s *Store) UpsertTTSCache(ctx context.Context, entry TTSAudioCache) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO tts_audio_cache (campaign_id, source, voice_key, audio_path, generated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (campaign_id, source, voice_key)
		 DO UPDATE SET audio_path = EXCLUDED.audio_path, generated_at = NOW()`,
		entry.CampaignID, entry.Source, entry.VoiceKey, entry.AudioPath,
	)
	if err != nil {
		return fmt.Errorf("upsert tts cache: %w", err)
	}
	return nil
}

func (s *Store) ListTTSCache(ctx context.Context, campaignID int64) ([]TTSAudioCache, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, campaign_id, source, voice_key, audio_path, generated_at
		 FROM tts_audio_cache
		 WHERE campaign_id = $1
		 ORDER BY generated_at DESC`,
		campaignID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tts cache: %w", err)
	}
	defer rows.Close()

	var entries []TTSAudioCache
	for rows.Next() {
		var c TTSAudioCache
		if err := rows.Scan(&c.ID, &c.CampaignID, &c.Source, &c.VoiceKey, &c.AudioPath, &c.GeneratedAt); err != nil {
			return nil, fmt.Errorf("scan tts cache: %w", err)
		}
		entries = append(entries, c)
	}
	return entries, nil
}

// DeleteTTSCacheForCampaignSource removes cache entries and returns the
// audio file paths so callers can clean up the files on disk.
func (s *Store) DeleteTTSCacheForCampaignSource(ctx context.Context, campaignID int64, source string) ([]string, error) {
	rows, err := s.Pool.Query(ctx,
		`DELETE FROM tts_audio_cache WHERE campaign_id = $1 AND source = $2 RETURNING audio_path`,
		campaignID, source,
	)
	if err != nil {
		return nil, fmt.Errorf("delete tts cache: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return paths, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}
