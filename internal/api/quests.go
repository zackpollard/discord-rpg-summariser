package api

import (
	"net/http"
	"time"

	"discord-rpg-summariser/internal/storage"

	"github.com/jackc/pgx/v5"
)

type questResponse struct {
	ID          int64     `json:"id"`
	CampaignID  int64     `json:"campaign_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Giver       string    `json:"giver"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type questDetailResponse struct {
	questResponse
	Updates []questUpdateResponse `json:"updates"`
}

type questUpdateResponse struct {
	ID        int64     `json:"id"`
	QuestID   int64     `json:"quest_id"`
	SessionID int64     `json:"session_id"`
	Content   string    `json:"content"`
	NewStatus *string   `json:"new_status"`
	CreatedAt time.Time `json:"created_at"`
}

func toQuestResponse(q *storage.Quest) questResponse {
	return questResponse{
		ID:          q.ID,
		CampaignID:  q.CampaignID,
		Name:        q.Name,
		Description: q.Description,
		Status:      q.Status,
		Giver:       q.Giver,
		CreatedAt:   q.CreatedAt,
		UpdatedAt:   q.UpdatedAt,
	}
}

func (s *Server) handleListQuests(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	status := r.URL.Query().Get("status")

	quests, err := s.store.ListQuests(r.Context(), campaignID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list quests")
		return
	}

	resp := make([]questResponse, len(quests))
	for i := range quests {
		resp[i] = toQuestResponse(&quests[i])
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetQuest(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	quest, err := s.store.GetQuest(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "quest not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get quest")
		return
	}

	updates, err := s.store.GetQuestUpdates(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get quest updates")
		return
	}

	updateResp := make([]questUpdateResponse, len(updates))
	for i := range updates {
		u := &updates[i]
		updateResp[i] = questUpdateResponse{
			ID:        u.ID,
			QuestID:   u.QuestID,
			SessionID: u.SessionID,
			Content:   u.Content,
			NewStatus: u.NewStatus,
			CreatedAt: u.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, questDetailResponse{
		questResponse: toQuestResponse(quest),
		Updates:       updateResp,
	})
}
