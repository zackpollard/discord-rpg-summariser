package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"discord-rpg-summariser/internal/pdf"

	"github.com/jackc/pgx/v5"
)

func (s *Server) handleGetCampaignPDF(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}

	ctx := r.Context()

	// Load campaign.
	campaign, err := s.store.GetCampaign(ctx, campaignID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		log.Printf("GetCampaign error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get campaign")
		return
	}

	// Load sessions (all complete sessions, chronological order).
	sessions, err := s.store.GetLatestCompleteSessions(ctx, campaignID, 1000)
	if err != nil {
		log.Printf("GetLatestCompleteSessions error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load sessions")
		return
	}

	// Load entities (all, no filter, generous limit).
	entities, err := s.store.ListEntities(ctx, campaignID, "", "", 10000, 0)
	if err != nil {
		log.Printf("ListEntities error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load entities")
		return
	}

	// Load quests (all).
	quests, err := s.store.ListQuests(ctx, campaignID, "")
	if err != nil {
		log.Printf("ListQuests error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load quests")
		return
	}

	// Load stats.
	stats, err := s.store.GetCampaignStats(ctx, campaignID, s.guildID)
	if err != nil {
		log.Printf("GetCampaignStats error: %v", err)
		// Stats are optional; continue without them.
		stats = nil
	}

	// Build the campaign book from storage types.
	book := pdf.FromStorage(campaign, sessions, entities, quests, stats)

	// Generate the PDF.
	pdfData, err := pdf.Generate(book)
	if err != nil {
		log.Printf("PDF generation error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate PDF")
		return
	}

	// Build a safe filename.
	filename := sanitizeFilename(campaign.Name) + ".pdf"

	// Override the Content-Type header (the CORS middleware sets application/json
	// for /api/ routes by default).
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfData)))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfData)
}

// sanitizeFilename makes a string safe for use as a filename.
func sanitizeFilename(name string) string {
	// Replace spaces with hyphens, remove unsafe characters.
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" {
		return "campaign"
	}
	return result
}
