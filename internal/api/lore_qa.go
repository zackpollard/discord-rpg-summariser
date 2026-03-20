package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"discord-rpg-summariser/internal/storage"
)

// LoreQAProvider answers lore questions using LLM + context. Implemented by *bot.Bot.
type LoreQAProvider interface {
	AskLore(ctx context.Context, campaignID int64, question, context string) (string, error)
}

type loreAskRequest struct {
	Question string `json:"question"`
}

type loreAskResponse struct {
	Answer  string                     `json:"answer"`
	Sources []storage.LoreSearchResult `json:"sources"`
}

func (s *Server) handleLoreAsk(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	var req loreAskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}

	// Try semantic search first if embedder is available, fall back to keyword search.
	var contextStr string
	var sources []storage.LoreSearchResult

	if s.embedder != nil {
		semanticCtx, semanticSources, err := s.buildSemanticContext(r.Context(), campaignID, req.Question)
		if err != nil {
			log.Printf("lore_qa: semantic search failed, falling back to keyword: %v", err)
		} else {
			contextStr = semanticCtx
			sources = semanticSources
		}
	}

	// Fall back to keyword search if semantic search was not available or failed.
	if contextStr == "" {
		keywordSources, err := s.store.SearchLore(r.Context(), campaignID, req.Question, 10)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to search lore")
			return
		}
		if keywordSources == nil {
			keywordSources = []storage.LoreSearchResult{}
		}
		sources = keywordSources
		contextStr = buildKeywordContext(keywordSources)
	}

	if s.loreQA == nil {
		writeError(w, http.StatusServiceUnavailable, "lore Q&A provider not available")
		return
	}

	answer, err := s.loreQA.AskLore(r.Context(), campaignID, req.Question, contextStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate answer")
		return
	}

	writeJSON(w, http.StatusOK, loreAskResponse{
		Answer:  answer,
		Sources: sources,
	})
}

// buildSemanticContext embeds the question, performs similarity search, and
// returns a formatted context string with source annotations, plus keyword
// search results for the response's Sources field.
func (s *Server) buildSemanticContext(ctx context.Context, campaignID int64, question string) (string, []storage.LoreSearchResult, error) {
	queryVec, err := s.embedder.Embed(ctx, question)
	if err != nil {
		return "", nil, fmt.Errorf("embed question: %w", err)
	}

	results, err := s.store.SearchSimilar(ctx, campaignID, queryVec, 10)
	if err != nil {
		return "", nil, fmt.Errorf("search similar: %w", err)
	}

	if len(results) == 0 {
		return "", nil, nil
	}

	// Build annotated context string.
	var sb strings.Builder
	var sources []storage.LoreSearchResult

	for _, r := range results {
		// Skip very low similarity results.
		if r.Similarity < 0.3 {
			continue
		}

		label := docTypeLabel(r.DocType)
		if r.Title != "" {
			sb.WriteString(fmt.Sprintf("[%s: %s] (relevance: %.0f%%)\n", label, r.Title, r.Similarity*100))
		} else {
			sb.WriteString(fmt.Sprintf("[%s] (relevance: %.0f%%)\n", label, r.Similarity*100))
		}
		sb.WriteString(r.Content)
		sb.WriteString("\n\n")

		// Convert to LoreSearchResult for backwards-compatible API response.
		sources = append(sources, storage.LoreSearchResult{
			Type:    r.DocType,
			ID:      r.DocID,
			Name:    r.Title,
			Content: r.Content,
		})
	}

	return sb.String(), sources, nil
}

// buildKeywordContext formats keyword search results into a context string.
func buildKeywordContext(sources []storage.LoreSearchResult) string {
	var sb strings.Builder
	for _, src := range sources {
		sb.WriteString("[")
		sb.WriteString(src.Type)
		sb.WriteString("] ")
		sb.WriteString(src.Name)
		sb.WriteString(": ")
		sb.WriteString(src.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// docTypeLabel returns a human-readable label for an embedding doc type.
func docTypeLabel(docType string) string {
	switch docType {
	case "summary":
		return "Session Summary"
	case "transcript_chunk":
		return "Transcript"
	case "entity":
		return "Entity"
	case "quest":
		return "Quest"
	default:
		return docType
	}
}

func (s *Server) handleLoreSearch(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter q is required")
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	results, err := s.store.SearchLore(r.Context(), campaignID, query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search lore")
		return
	}

	if results == nil {
		results = []storage.LoreSearchResult{}
	}

	writeJSON(w, http.StatusOK, results)
}
