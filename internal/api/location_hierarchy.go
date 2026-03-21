package api

import (
	"net/http"
	"time"

	"discord-rpg-summariser/internal/storage"
)

type locationNodeResponse struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	ParentID    *int64                 `json:"parent_id"`
	Children    []locationNodeResponse `json:"children"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func (s *Server) handleLocationHierarchy(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	places, err := s.store.GetLocationHierarchy(r.Context(), campaignID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get location hierarchy")
		return
	}

	roots := buildLocationTree(places)
	if roots == nil {
		roots = []locationNodeResponse{}
	}

	writeJSON(w, http.StatusOK, roots)
}

// buildLocationTree converts a flat slice of place entities into a nested tree.
func buildLocationTree(places []storage.Entity) []locationNodeResponse {
	// Build index of id -> children.
	childrenOf := make(map[int64][]int)
	idSet := make(map[int64]bool, len(places))
	for _, p := range places {
		idSet[p.ID] = true
	}

	for i, p := range places {
		if p.ParentEntityID != nil && idSet[*p.ParentEntityID] {
			childrenOf[*p.ParentEntityID] = append(childrenOf[*p.ParentEntityID], i)
		}
	}

	// Recursive builder.
	var build func(idx int) locationNodeResponse
	build = func(idx int) locationNodeResponse {
		p := &places[idx]
		node := locationNodeResponse{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			ParentID:    p.ParentEntityID,
			Children:    []locationNodeResponse{},
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		}
		for _, ci := range childrenOf[p.ID] {
			node.Children = append(node.Children, build(ci))
		}
		return node
	}

	// Roots are places with no parent or whose parent is not in the place set.
	var roots []locationNodeResponse
	for i, p := range places {
		if p.ParentEntityID == nil || !idSet[*p.ParentEntityID] {
			roots = append(roots, build(i))
		}
	}
	return roots
}
