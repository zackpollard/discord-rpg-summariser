package storage

import (
	"context"
	"fmt"
)

type LoreSearchResult struct {
	Type    string `json:"type"` // entity, note, summary, quest
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// SearchLore performs keyword search across entities, entity notes, session
// summaries, and quests for a campaign.
func (s *Store) SearchLore(ctx context.Context, campaignID int64, query string, limit int) ([]LoreSearchResult, error) {
	pattern := "%" + query + "%"
	rows, err := s.Pool.Query(ctx, `
		SELECT type, id, name, content FROM (
			SELECT 'entity' as type, e.id, e.name, e.description as content
			FROM entities e WHERE e.campaign_id = $1 AND (e.name ILIKE $2 OR e.description ILIKE $2)

			UNION ALL

			SELECT 'note' as type, en.id, e.name, en.content
			FROM entity_notes en
			JOIN entities e ON e.id = en.entity_id
			WHERE e.campaign_id = $1 AND en.content ILIKE $2

			UNION ALL

			SELECT 'summary' as type, s.id, 'Session #' || s.id, s.summary
			FROM sessions s WHERE s.campaign_id = $1 AND s.summary ILIKE $2

			UNION ALL

			SELECT 'quest' as type, q.id, q.name, q.description as content
			FROM quests q WHERE q.campaign_id = $1 AND (q.name ILIKE $2 OR q.description ILIKE $2)
		) results
		LIMIT $3`, campaignID, pattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search lore: %w", err)
	}
	defer rows.Close()

	var results []LoreSearchResult
	for rows.Next() {
		var r LoreSearchResult
		if err := rows.Scan(&r.Type, &r.ID, &r.Name, &r.Content); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
