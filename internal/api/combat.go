package api

import (
	"net/http"
	"strconv"
	"time"
)

type combatActionResponse struct {
	ID         int64    `json:"id"`
	Actor      string   `json:"actor"`
	ActionType string   `json:"action_type"`
	Target     string   `json:"target"`
	Detail     string   `json:"detail"`
	Damage     *int     `json:"damage"`
	Round      *int     `json:"round"`
	Timestamp  *float64 `json:"timestamp"`
}

type combatEncounterResponse struct {
	ID        int64                  `json:"id"`
	SessionID int64                  `json:"session_id"`
	Name      string                 `json:"name"`
	StartTime float64                `json:"start_time"`
	EndTime   float64                `json:"end_time"`
	Summary   string                 `json:"summary"`
	CreatedAt time.Time              `json:"created_at"`
	Actions   []combatActionResponse `json:"actions"`
}

func (s *Server) handleGetSessionCombat(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	encounters, err := s.store.GetCombatEncounters(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get combat encounters")
		return
	}

	resp := make([]combatEncounterResponse, len(encounters))
	for i := range encounters {
		enc := &encounters[i]

		actions, err := s.store.GetCombatActions(r.Context(), enc.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get combat actions")
			return
		}

		actionResp := make([]combatActionResponse, len(actions))
		for j := range actions {
			a := &actions[j]
			actionResp[j] = combatActionResponse{
				ID:         a.ID,
				Actor:      a.Actor,
				ActionType: a.ActionType,
				Target:     a.Target,
				Detail:     a.Detail,
				Damage:     a.Damage,
				Round:      a.Round,
				Timestamp:  a.Timestamp,
			}
		}

		resp[i] = combatEncounterResponse{
			ID:        enc.ID,
			SessionID: enc.SessionID,
			Name:      enc.Name,
			StartTime: enc.StartTime,
			EndTime:   enc.EndTime,
			Summary:   enc.Summary,
			CreatedAt: enc.CreatedAt,
			Actions:   actionResp,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
