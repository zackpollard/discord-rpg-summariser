package api

import (
	"fmt"
	"net/http"
	"strings"
)

type combatAnalysisResponse struct {
	TacticalSummary string `json:"tactical_summary"`
	MVP             string `json:"mvp"`
	ClosestCall     string `json:"closest_call"`
	FunniestMoment  string `json:"funniest_moment"`
}

func (s *Server) handleGetCombatAnalysis(w http.ResponseWriter, r *http.Request) {
	encounterID, ok := parsePathID(w, r, "encounterId")
	if !ok {
		return
	}

	if s.summariser == nil {
		writeError(w, http.StatusServiceUnavailable, "summariser not available")
		return
	}

	// Get actions for this encounter.
	actions, err := s.store.GetCombatActions(r.Context(), encounterID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get combat actions")
		return
	}

	// Build action descriptions.
	var actionDescs []string
	for _, a := range actions {
		desc := fmt.Sprintf("%s used %s (%s)", a.Actor, a.ActionType, a.Detail)
		if a.Target != "" {
			desc += " on " + a.Target
		}
		if a.Damage != nil {
			desc += fmt.Sprintf(" for %d damage", *a.Damage)
		}
		actionDescs = append(actionDescs, desc)
	}

	// Get encounter summary from the query param (frontend sends it).
	encounterSummary := r.URL.Query().Get("summary")
	if encounterSummary == "" {
		encounterSummary = "Combat encounter"
	}

	// Get player characters from query.
	var playerCharacters []string
	if pc := r.URL.Query().Get("characters"); pc != "" {
		for _, name := range strings.Split(pc, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				playerCharacters = append(playerCharacters, name)
			}
		}
	}

	result, err := s.summariser.AnalyzeCombat(r.Context(), encounterSummary, actionDescs, playerCharacters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to analyze combat")
		return
	}

	writeJSON(w, http.StatusOK, combatAnalysisResponse{
		TacticalSummary: result.TacticalSummary,
		MVP:             result.MVP,
		ClosestCall:     result.ClosestCall,
		FunniestMoment:  result.FunniestMoment,
	})
}
