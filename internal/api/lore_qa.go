package api

import (
	"context"
	"encoding/json"
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
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
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

	sources, err := s.store.SearchLore(r.Context(), campaignID, req.Question, 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search lore")
		return
	}
	if sources == nil {
		sources = []storage.LoreSearchResult{}
	}

	// Build context string from search results.
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

	if s.loreQA == nil {
		writeError(w, http.StatusServiceUnavailable, "lore Q&A provider not available")
		return
	}

	answer, err := s.loreQA.AskLore(r.Context(), campaignID, req.Question, sb.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate answer")
		return
	}

	writeJSON(w, http.StatusOK, loreAskResponse{
		Answer:  answer,
		Sources: sources,
	})
}

func (s *Server) handleLoreSearch(w http.ResponseWriter, r *http.Request) {
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
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
