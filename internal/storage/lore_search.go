package storage

import (
	"context"
	"fmt"
	"strings"
)

type LoreSearchResult struct {
	Type    string `json:"type"` // entity, note, summary, quest
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// stopWords are common words filtered out when building keyword search patterns.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true, "have": true,
	"has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true, "may": true,
	"might": true, "shall": true, "can": true, "to": true, "of": true,
	"in": true, "for": true, "on": true, "with": true, "at": true,
	"by": true, "from": true, "as": true, "into": true, "about": true,
	"that": true, "this": true, "it": true, "its": true, "and": true,
	"or": true, "but": true, "not": true, "no": true, "so": true,
	"if": true, "then": true, "than": true, "when": true, "where": true,
	"what": true, "which": true, "who": true, "whom": true, "how": true,
	"why": true, "we": true, "they": true, "he": true, "she": true,
	"i": true, "you": true, "me": true, "us": true, "them": true,
	"my": true, "our": true, "your": true, "his": true, "her": true,
	"their": true, "there": true, "here": true, "up": true, "out": true,
	"just": true, "also": true, "very": true, "too": true, "all": true,
	"any": true, "each": true, "some": true, "tell": true, "know": true,
	"meet": true, "met": true, "happen": true, "happened": true,
}

// extractKeywords splits a query into meaningful words for ILIKE search.
func extractKeywords(query string) []string {
	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, w := range words {
		// Strip common punctuation
		w = strings.Trim(w, "?,!.;:'\"")
		if len(w) < 2 || stopWords[w] {
			continue
		}
		keywords = append(keywords, w)
	}
	return keywords
}

// SearchLore performs keyword search across entities, entity notes, session
// summaries, and quests for a campaign. The query is split into keywords and
// results matching any keyword are returned.
func (s *Store) SearchLore(ctx context.Context, campaignID int64, query string, limit int) ([]LoreSearchResult, error) {
	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		// Fall back to full query if no keywords extracted
		keywords = []string{query}
	}

	// Build OR conditions for each keyword
	var conditions []string
	var args []any
	args = append(args, campaignID) // $1

	for i, kw := range keywords {
		paramIdx := i + 2 // $2, $3, ...
		conditions = append(conditions, fmt.Sprintf("$%d", paramIdx))
		args = append(args, "%"+kw+"%")
	}

	// Build the ILIKE ANY pattern using array
	likeClause := fmt.Sprintf("ILIKE ANY(ARRAY[%s])", strings.Join(conditions, ","))

	sql := fmt.Sprintf(`
		SELECT type, id, name, content FROM (
			SELECT 'entity' as type, e.id, e.name, e.description as content
			FROM entities e WHERE e.campaign_id = $1 AND (e.name %[1]s OR e.description %[1]s)

			UNION ALL

			SELECT 'note' as type, en.id, e.name, en.content
			FROM entity_notes en
			JOIN entities e ON e.id = en.entity_id
			WHERE e.campaign_id = $1 AND en.content %[1]s

			UNION ALL

			SELECT 'summary' as type, s.id, 'Session #' || s.id, s.summary
			FROM sessions s WHERE s.campaign_id = $1 AND s.summary %[1]s

			UNION ALL

			SELECT 'quest' as type, q.id, q.name, q.description as content
			FROM quests q WHERE q.campaign_id = $1 AND (q.name %[1]s OR q.description %[1]s)
		) results
		LIMIT %d`, likeClause, limit)

	args = append(args) // limit is embedded in query string

	rows, err := s.Pool.Query(ctx, sql, args...)
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
