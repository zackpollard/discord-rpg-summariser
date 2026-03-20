package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parsePathID extracts an int64 ID from the named path parameter. It writes a
// 400 error and returns 0, false if the value is missing or not a valid integer.
func parsePathID(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(param), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+param)
		return 0, false
	}
	return id, true
}

// displayNameResolver returns a function that caches resolved display names
// for user IDs using the server's member provider.
func (s *Server) displayNameResolver() func(string) string {
	cache := make(map[string]string)
	return func(userID string) string {
		if name, ok := cache[userID]; ok {
			return name
		}
		name := userID
		if s.memberP != nil {
			name = s.memberP.ResolveUsername(userID)
		}
		cache[userID] = name
		return name
	}
}

// parsePagination extracts limit and offset query parameters with defaults.
func parsePagination(r *http.Request, defaultLimit int) (limit, offset int) {
	limit = defaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}
