package bot

import (
	"context"
	"fmt"
	"log"

	"discord-rpg-summariser/internal/embed"
	"discord-rpg-summariser/internal/storage"
	"discord-rpg-summariser/internal/transcribe"
)

// generateEmbeddings creates vector embeddings for a completed session's
// content and stores them for RAG-based retrieval. This is a best-effort
// operation — failures are logged but do not fail the pipeline.
func (b *Bot) generateEmbeddings(ctx context.Context, session *storage.Session, sessionID int64, merged []transcribe.UserSegment, summary string, dmName string) {
	if b.embedder == nil {
		return
	}

	// Resolve character names for chunking.
	charNames := make(map[string]string)
	for _, seg := range merged {
		if _, ok := charNames[seg.UserID]; ok {
			continue
		}
		name, _ := b.store.GetCharacterName(ctx, seg.UserID, session.CampaignID)
		if name != "" {
			charNames[seg.UserID] = name
		}
	}

	// Collect all texts and their metadata for batch embedding.
	var docs []embeddingItem

	// 1. Session summary.
	if summary != "" {
		docs = append(docs, embeddingItem{
			doc: storage.EmbeddingDoc{
				CampaignID: session.CampaignID,
				DocType:    "summary",
				DocID:      sessionID,
				SessionID:  &sessionID,
				Title:      fmt.Sprintf("Session #%d Summary", sessionID),
				Content:    summary,
			},
			text: fmt.Sprintf("Session #%d Summary:\n%s", sessionID, summary),
		})
	}

	// 2. Transcript chunks.
	var segs []embed.TranscriptSegment
	for _, s := range merged {
		segs = append(segs, embed.TranscriptSegment{
			UserID:    s.UserID,
			StartTime: s.StartTime,
			EndTime:   s.EndTime,
			Text:      s.Text,
		})
	}
	chunks := embed.ChunkTranscriptSegments(segs, sessionID, charNames)
	for _, c := range chunks {
		sid := sessionID
		docs = append(docs, embeddingItem{
			doc: storage.EmbeddingDoc{
				CampaignID: session.CampaignID,
				DocType:    c.DocType,
				DocID:      c.DocID,
				SessionID:  &sid,
				Title:      c.Title,
				Content:    c.Content,
			},
			text: c.Content,
		})
	}

	// 3. Entities from this session.
	entities, _ := b.store.ListEntities(ctx, session.CampaignID, "", "", 1000, 0)
	for _, e := range entities {
		notes, _ := b.store.GetEntityNotes(ctx, e.ID)
		var noteTexts []string
		for _, n := range notes {
			noteTexts = append(noteTexts, n.Content)
		}
		text := embed.BuildEntityText(e.Name, e.Type, e.Description, noteTexts)
		docs = append(docs, embeddingItem{
			doc: storage.EmbeddingDoc{
				CampaignID: session.CampaignID,
				DocType:    "entity",
				DocID:      e.ID,
				Title:      fmt.Sprintf("%s (%s)", e.Name, e.Type),
				Content:    text,
			},
			text: text,
		})
	}

	// 4. Quests.
	quests, _ := b.store.ListQuests(ctx, session.CampaignID, "")
	for _, q := range quests {
		updates, _ := b.store.GetQuestUpdates(ctx, q.ID)
		var updateTexts []string
		for _, u := range updates {
			updateTexts = append(updateTexts, u.Content)
		}
		text := embed.BuildQuestText(q.Name, q.Description, q.Status, q.Giver, updateTexts)
		docs = append(docs, embeddingItem{
			doc: storage.EmbeddingDoc{
				CampaignID: session.CampaignID,
				DocType:    "quest",
				DocID:      q.ID,
				Title:      fmt.Sprintf("Quest: %s", q.Name),
				Content:    text,
			},
			text: text,
		})
	}

	if len(docs) == 0 {
		return
	}

	// Batch embed all texts.
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.text
	}

	vectors, err := b.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		log.Printf("pipeline: embeddings batch failed: %v", err)
		return
	}

	if len(vectors) != len(docs) {
		log.Printf("pipeline: embeddings count mismatch: got %d vectors for %d docs", len(vectors), len(docs))
		return
	}

	// Store each embedding.
	stored := 0
	for i, doc := range docs {
		doc.doc.Embedding = vectors[i]
		if err := b.store.UpsertEmbedding(ctx, doc.doc); err != nil {
			log.Printf("pipeline: upsert embedding %s/%d: %v", doc.doc.DocType, doc.doc.DocID, err)
			continue
		}
		stored++
	}

	log.Printf("pipeline: stored %d embeddings for session %d (%d summary, %d chunks, %d entities, %d quests)",
		stored, sessionID,
		boolToInt(summary != ""),
		len(chunks),
		len(entities),
		len(quests))
}

// embeddingItem pairs an EmbeddingDoc (for storage) with the text to embed.
type embeddingItem struct {
	doc  storage.EmbeddingDoc
	text string
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
