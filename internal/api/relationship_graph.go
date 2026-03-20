package api

import (
	"net/http"
	"strconv"
)

type graphNodeResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type graphEdgeResponse struct {
	Source       int64  `json:"source"`
	Target       int64  `json:"target"`
	Relationship string `json:"relationship"`
	Description  string `json:"description"`
}

type relationshipGraphResponse struct {
	Nodes []graphNodeResponse `json:"nodes"`
	Edges []graphEdgeResponse `json:"edges"`
}

func (s *Server) handleRelationshipGraph(w http.ResponseWriter, r *http.Request) {
	campaignID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign id")
		return
	}

	entities, rels, err := s.store.GetCampaignRelationshipGraph(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get relationship graph")
		return
	}

	nodes := make([]graphNodeResponse, len(entities))
	for i := range entities {
		e := &entities[i]
		nodes[i] = graphNodeResponse{
			ID:   e.ID,
			Name: e.Name,
			Type: e.Type,
		}
	}

	edges := make([]graphEdgeResponse, len(rels))
	for i := range rels {
		rel := &rels[i]
		edges[i] = graphEdgeResponse{
			Source:       rel.SourceID,
			Target:       rel.TargetID,
			Relationship: rel.Relationship,
			Description:  rel.Description,
		}
	}

	writeJSON(w, http.StatusOK, relationshipGraphResponse{
		Nodes: nodes,
		Edges: edges,
	})
}
