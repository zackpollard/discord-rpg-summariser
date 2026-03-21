// Package pdf generates a D&D-style campaign book as a PDF.
package pdf

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"discord-rpg-summariser/internal/storage"

	"github.com/go-pdf/fpdf"
)

// CampaignBook holds all the data needed to generate a campaign PDF.
type CampaignBook struct {
	Campaign Campaign
	Sessions []Session
	Entities []Entity
	Quests   []Quest
	Stats    *CampaignStats
	Recap    string
}

// Campaign is a lightweight copy of the campaign data for PDF generation.
type Campaign struct {
	Name        string
	Description string
	CreatedAt   time.Time
}

// Session is a lightweight copy of session data for PDF generation.
type Session struct {
	ID        int64
	StartedAt time.Time
	EndedAt   *time.Time
	Summary   string
	KeyEvents []string
}

// Entity is a lightweight copy of entity data for PDF generation.
type Entity struct {
	Name         string
	Type         string
	Description  string
	Status       string
	CauseOfDeath string
}

// Quest is a lightweight copy of quest data for PDF generation.
type Quest struct {
	Name        string
	Description string
	Status      string
	Giver       string
}

// CampaignStats is a lightweight copy of stats for PDF generation.
type CampaignStats struct {
	TotalSessions   int
	TotalDurationMin float64
	AvgDurationMin  float64
	TotalWords      int
	TotalQuests     int
	ActiveQuests    int
	CompletedQuests int
	FailedQuests    int
	TotalEncounters int
	TotalDamage     int
	EntityCounts    map[string]int
	NPCStatusCounts map[string]int
}

// colour constants (RGB) for the D&D-inspired theme.
var (
	colBackground = [3]int{253, 245, 230} // #FDF5E6 oldlace
	colText       = [3]int{62, 39, 35}    // #3E2723 dark brown
	colHeader     = [3]int{139, 0, 0}     // #8B0000 dark red
	colAccent     = [3]int{218, 165, 32}  // #DAA520 goldenrod
	colMuted      = [3]int{120, 90, 70}   // warm muted brown
	colQuestBox   = [3]int{245, 235, 220} // slightly darker parchment for boxes
)

// page dimensions (A4 portrait, mm).
const (
	pageW  = 210.0
	pageH  = 297.0
	marginL = 20.0
	marginR = 20.0
	marginT = 25.0
	marginB = 25.0
	contentW = pageW - marginL - marginR
)

// tocEntry stores table-of-contents items collected during generation.
type tocEntry struct {
	title string
	page  int
}

// generator wraps fpdf and state used during generation.
type generator struct {
	pdf          *fpdf.Fpdf
	book         *CampaignBook
	toc          []tocEntry
	currentPage  int
	tocPlacePage int // page where TOC starts, so we can rewrite it
}

// Generate creates a PDF campaign book and returns the raw bytes.
func Generate(book *CampaignBook) ([]byte, error) {
	g := &generator{
		book: book,
	}
	g.initPDF()
	g.buildBook()

	var buf bytes.Buffer
	if err := g.pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	if g.pdf.Err() {
		return nil, fmt.Errorf("pdf error: %w", g.pdf.Error())
	}
	return buf.Bytes(), nil
}

func (g *generator) initPDF() {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, marginB)
	pdf.SetMargins(marginL, marginT, marginR)

	// Footer with page number and campaign name.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
		pdf.CellFormat(contentW/2, 10, g.book.Campaign.Name, "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/2, 10, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	g.pdf = pdf
}

func (g *generator) buildBook() {
	g.buildTitlePage()

	// Placeholder page for TOC — we will come back and fill it in.
	g.tocPlacePage = g.pdf.PageNo() + 1
	g.pdf.AddPage()
	g.drawPageBackground()

	// Build content sections, collecting TOC entries.
	if g.book.Recap != "" {
		g.buildRecapSection()
	}
	if len(g.book.Sessions) > 0 {
		g.buildSessionsSection()
	}
	if len(g.book.Entities) > 0 {
		g.buildEntitiesSection()
	}
	if len(g.book.Quests) > 0 {
		g.buildQuestsSection()
	}
	if g.book.Stats != nil {
		g.buildStatsSection()
	}

	// Now fill in the TOC page.
	g.fillTOC()
}

// ---- Title Page ----

func (g *generator) buildTitlePage() {
	g.pdf.AddPage()
	g.drawPageBackground()

	// Decorative top border.
	g.pdf.SetDrawColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.SetLineWidth(1.5)
	g.pdf.Line(marginL, 40, pageW-marginR, 40)
	g.pdf.Line(marginL, 42, pageW-marginR, 42)

	// Campaign name.
	g.pdf.SetY(65)
	g.pdf.SetFont("Helvetica", "B", 36)
	g.pdf.SetTextColor(colHeader[0], colHeader[1], colHeader[2])
	g.pdf.MultiCell(contentW, 14, g.book.Campaign.Name, "", "C", false)

	// Subtitle.
	g.pdf.Ln(6)
	g.pdf.SetFont("Helvetica", "I", 16)
	g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.CellFormat(contentW, 10, "Campaign Journal", "", 1, "C", false, 0, "")

	// Description.
	if g.book.Campaign.Description != "" {
		g.pdf.Ln(10)
		g.pdf.SetFont("Helvetica", "", 11)
		g.pdf.SetTextColor(colText[0], colText[1], colText[2])
		g.pdf.MultiCell(contentW, 6, g.book.Campaign.Description, "", "C", false)
	}

	// Decorative diamond.
	g.pdf.Ln(15)
	g.pdf.SetFont("Helvetica", "", 18)
	g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.CellFormat(contentW, 10, "---  *  ---", "", 1, "C", false, 0, "")

	// Date.
	g.pdf.Ln(5)
	g.pdf.SetFont("Helvetica", "I", 10)
	g.pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
	dateStr := fmt.Sprintf("Generated on %s", time.Now().Format("2 January 2006"))
	g.pdf.CellFormat(contentW, 8, dateStr, "", 1, "C", false, 0, "")

	if !g.book.Campaign.CreatedAt.IsZero() {
		g.pdf.SetFont("Helvetica", "I", 10)
		g.pdf.CellFormat(contentW, 8, fmt.Sprintf("Campaign started %s", g.book.Campaign.CreatedAt.Format("2 January 2006")), "", 1, "C", false, 0, "")
	}

	// Decorative bottom border.
	g.pdf.SetDrawColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.SetLineWidth(1.5)
	g.pdf.Line(marginL, 230, pageW-marginR, 230)
	g.pdf.Line(marginL, 232, pageW-marginR, 232)
}

// ---- Table of Contents ----

func (g *generator) fillTOC() {
	// Go to the TOC page and draw it.
	g.pdf.SetPage(g.tocPlacePage)
	g.pdf.SetY(marginT)

	g.pdf.SetFont("Helvetica", "B", 22)
	g.pdf.SetTextColor(colHeader[0], colHeader[1], colHeader[2])
	g.pdf.CellFormat(contentW, 12, "Table of Contents", "", 1, "C", false, 0, "")
	g.pdf.Ln(4)
	g.drawHorizontalRule()
	g.pdf.Ln(6)

	g.pdf.SetFont("Helvetica", "", 12)
	g.pdf.SetTextColor(colText[0], colText[1], colText[2])

	for _, entry := range g.toc {
		y := g.pdf.GetY()
		if y > pageH-marginB-10 {
			break // don't overflow the TOC page
		}
		title := entry.title
		pageStr := fmt.Sprintf("%d", entry.page)

		// Draw title on left.
		g.pdf.CellFormat(contentW-15, 8, title, "", 0, "L", false, 0, "")
		// Draw page on right.
		g.pdf.CellFormat(15, 8, pageStr, "", 1, "R", false, 0, "")
	}
}

// ---- Recap Section ----

func (g *generator) buildRecapSection() {
	g.pdf.AddPage()
	g.drawPageBackground()
	g.toc = append(g.toc, tocEntry{title: "The Story So Far", page: g.pdf.PageNo()})

	g.drawSectionTitle("The Story So Far")
	g.pdf.Ln(4)

	// Drop cap for first paragraph.
	paragraphs := splitParagraphs(g.book.Recap)
	for i, para := range paragraphs {
		if i == 0 {
			g.drawDropCapParagraph(para)
		} else {
			g.drawBodyText(para)
		}
		g.pdf.Ln(3)
	}
}

// ---- Sessions Section ----

func (g *generator) buildSessionsSection() {
	g.pdf.AddPage()
	g.drawPageBackground()
	g.toc = append(g.toc, tocEntry{title: "Session Chronicles", page: g.pdf.PageNo()})

	g.drawSectionTitle("Session Chronicles")
	g.pdf.Ln(4)

	for i, sess := range g.book.Sessions {
		g.checkPageBreak(40) // ensure enough space for a session header

		// Session header.
		title := fmt.Sprintf("Session %d", i+1)
		dateStr := sess.StartedAt.Format("2 January 2006")
		duration := ""
		if sess.EndedAt != nil {
			dur := sess.EndedAt.Sub(sess.StartedAt)
			hours := int(dur.Hours())
			mins := int(dur.Minutes()) % 60
			if hours > 0 {
				duration = fmt.Sprintf("%dh %dm", hours, mins)
			} else {
				duration = fmt.Sprintf("%dm", mins)
			}
		}

		g.pdf.SetFont("Helvetica", "B", 13)
		g.pdf.SetTextColor(colHeader[0], colHeader[1], colHeader[2])
		g.pdf.CellFormat(contentW, 8, title, "", 1, "L", false, 0, "")

		g.pdf.SetFont("Helvetica", "I", 9)
		g.pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
		meta := dateStr
		if duration != "" {
			meta += "  |  Duration: " + duration
		}
		g.pdf.CellFormat(contentW, 5, meta, "", 1, "L", false, 0, "")
		g.pdf.Ln(2)

		// Summary.
		if sess.Summary != "" {
			g.drawBodyText(sess.Summary)
			g.pdf.Ln(2)
		}

		// Key events.
		if len(sess.KeyEvents) > 0 {
			g.pdf.SetFont("Helvetica", "BI", 10)
			g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
			g.pdf.CellFormat(contentW, 6, "Key Events:", "", 1, "L", false, 0, "")
			g.pdf.Ln(1)

			g.pdf.SetFont("Helvetica", "", 9)
			g.pdf.SetTextColor(colText[0], colText[1], colText[2])
			for _, evt := range sess.KeyEvents {
				g.checkPageBreak(8)
				bullet := "  \u2022  " + evt
				g.pdf.MultiCell(contentW-5, 5, bullet, "", "L", false)
			}
		}

		g.pdf.Ln(4)
		g.drawThinRule()
		g.pdf.Ln(4)
	}
}

// ---- Entities Section ----

func (g *generator) buildEntitiesSection() {
	g.pdf.AddPage()
	g.drawPageBackground()
	g.toc = append(g.toc, tocEntry{title: "Compendium of Lore", page: g.pdf.PageNo()})

	g.drawSectionTitle("Compendium of Lore")
	g.pdf.Ln(4)

	// Group entities by type.
	typeOrder := []string{"pc", "npc", "place", "organisation", "item", "event"}
	typeLabels := map[string]string{
		"pc":           "Player Characters",
		"npc":          "Non-Player Characters",
		"place":        "Places",
		"organisation": "Organisations",
		"item":         "Items",
		"event":        "Events",
	}
	grouped := make(map[string][]Entity)
	for _, e := range g.book.Entities {
		grouped[e.Type] = append(grouped[e.Type], e)
	}

	// Also collect any types not in our predefined order.
	seen := make(map[string]bool)
	for _, t := range typeOrder {
		seen[t] = true
	}
	for t := range grouped {
		if !seen[t] {
			typeOrder = append(typeOrder, t)
		}
	}

	for _, typ := range typeOrder {
		entities := grouped[typ]
		if len(entities) == 0 {
			continue
		}

		g.checkPageBreak(20)

		// Type sub-header.
		label := typeLabels[typ]
		if label == "" {
			label = strings.Title(typ) //nolint:staticcheck
		}
		g.pdf.SetFont("Helvetica", "B", 14)
		g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
		g.pdf.CellFormat(contentW, 9, label, "", 1, "L", false, 0, "")
		g.pdf.Ln(2)

		// Render entities in a two-column layout.
		g.renderEntitiesTwoColumn(entities)
		g.pdf.Ln(6)
	}
}

func (g *generator) renderEntitiesTwoColumn(entities []Entity) {
	colW := (contentW - 6) / 2 // 6mm gap between columns
	leftX := marginL
	rightX := marginL + colW + 6

	col := 0 // 0 = left, 1 = right
	savedY := g.pdf.GetY()
	maxY := savedY

	for _, e := range entities {
		// Estimate height needed.
		estLines := 2 // name + at least one line
		if e.Description != "" {
			estLines += int(math.Ceil(float64(len(e.Description)) / 60.0))
		}
		estH := float64(estLines)*5 + 8

		g.checkPageBreak(estH)
		if g.pdf.GetY() < savedY {
			// Page break happened, reset columns.
			savedY = g.pdf.GetY()
			maxY = savedY
			col = 0
		}

		var x float64
		if col == 0 {
			x = leftX
			savedY = g.pdf.GetY()
		} else {
			x = rightX
			g.pdf.SetY(savedY)
		}

		// Entity name.
		g.pdf.SetX(x)
		g.pdf.SetFont("Helvetica", "B", 10)
		g.pdf.SetTextColor(colText[0], colText[1], colText[2])
		g.pdf.CellFormat(colW, 5, e.Name, "", 1, "L", false, 0, "")

		// Status badge for NPCs.
		if e.Type == "npc" && e.Status != "" && e.Status != "unknown" {
			g.pdf.SetX(x)
			g.pdf.SetFont("Helvetica", "I", 8)
			if e.Status == "dead" {
				g.pdf.SetTextColor(139, 0, 0)
				statusText := "Dead"
				if e.CauseOfDeath != "" {
					statusText += " - " + e.CauseOfDeath
				}
				g.pdf.CellFormat(colW, 4, statusText, "", 1, "L", false, 0, "")
			} else {
				g.pdf.SetTextColor(34, 139, 34)
				g.pdf.CellFormat(colW, 4, strings.Title(e.Status), "", 1, "L", false, 0, "")//nolint:staticcheck
			}
		}

		// Description.
		if e.Description != "" {
			g.pdf.SetX(x)
			g.pdf.SetFont("Helvetica", "", 8)
			g.pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
			// Truncate very long descriptions.
			desc := e.Description
			if len(desc) > 200 {
				desc = desc[:197] + "..."
			}
			g.pdf.MultiCell(colW, 4, desc, "", "L", false)
		}

		endY := g.pdf.GetY()
		if endY > maxY {
			maxY = endY
		}

		// Alternate columns.
		if col == 0 {
			col = 1
		} else {
			col = 0
			g.pdf.SetY(maxY + 3)
			savedY = g.pdf.GetY()
			maxY = savedY
		}
	}

	// If we ended on the left column, move past the right column's extent.
	if col == 1 {
		g.pdf.SetY(maxY + 3)
	}
}

// ---- Quests Section ----

func (g *generator) buildQuestsSection() {
	g.pdf.AddPage()
	g.drawPageBackground()
	g.toc = append(g.toc, tocEntry{title: "Quest Log", page: g.pdf.PageNo()})

	g.drawSectionTitle("Quest Log")
	g.pdf.Ln(4)

	for _, q := range g.book.Quests {
		g.checkPageBreak(30)
		g.drawQuestBox(q)
		g.pdf.Ln(4)
	}
}

func (g *generator) drawQuestBox(q Quest) {
	x := marginL
	y := g.pdf.GetY()
	boxW := contentW

	// Estimate height.
	descLines := 1
	if q.Description != "" {
		descLines = int(math.Ceil(float64(len(q.Description)) / 80.0))
		if descLines < 1 {
			descLines = 1
		}
	}
	boxH := float64(8+5+descLines*5) + 8 // header + giver + desc + padding

	// Draw box background.
	g.pdf.SetFillColor(colQuestBox[0], colQuestBox[1], colQuestBox[2])
	g.pdf.SetDrawColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.SetLineWidth(0.5)
	g.pdf.RoundedRect(x, y, boxW, boxH, 2, "1234", "FD")

	// Status icon.
	g.pdf.SetY(y + 3)
	g.pdf.SetX(x + 3)
	statusIcon := questStatusIcon(q.Status)
	g.pdf.SetFont("Helvetica", "B", 10)
	g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.CellFormat(8, 6, statusIcon, "", 0, "C", false, 0, "")

	// Quest name.
	g.pdf.SetFont("Helvetica", "B", 11)
	g.pdf.SetTextColor(colText[0], colText[1], colText[2])
	g.pdf.CellFormat(boxW-30, 6, q.Name, "", 0, "L", false, 0, "")

	// Status text.
	g.pdf.SetFont("Helvetica", "I", 9)
	g.pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
	g.pdf.CellFormat(18, 6, strings.Title(q.Status), "", 1, "R", false, 0, "")//nolint:staticcheck

	// Giver.
	if q.Giver != "" {
		g.pdf.SetX(x + 14)
		g.pdf.SetFont("Helvetica", "I", 8)
		g.pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
		g.pdf.CellFormat(boxW-17, 4, "Given by: "+q.Giver, "", 1, "L", false, 0, "")
	}

	// Description.
	if q.Description != "" {
		g.pdf.SetX(x + 14)
		g.pdf.SetFont("Helvetica", "", 9)
		g.pdf.SetTextColor(colText[0], colText[1], colText[2])
		desc := q.Description
		if len(desc) > 300 {
			desc = desc[:297] + "..."
		}
		g.pdf.MultiCell(boxW-17, 5, desc, "", "L", false)
	}

	g.pdf.SetY(y + boxH)
}

func questStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "active":
		return ">"
	case "completed":
		return "+"
	case "failed":
		return "X"
	default:
		return "?"
	}
}

// ---- Stats Section ----

func (g *generator) buildStatsSection() {
	g.pdf.AddPage()
	g.drawPageBackground()
	g.toc = append(g.toc, tocEntry{title: "Campaign Statistics", page: g.pdf.PageNo()})

	g.drawSectionTitle("Campaign Statistics")
	g.pdf.Ln(6)

	stats := g.book.Stats

	// Stats in a grid layout.
	g.drawStatRow("Total Sessions", fmt.Sprintf("%d", stats.TotalSessions))
	g.drawStatRow("Total Play Time", formatDuration(stats.TotalDurationMin))
	g.drawStatRow("Average Session", formatDuration(stats.AvgDurationMin))
	g.drawStatRow("Total Words Spoken", formatNumber(stats.TotalWords))
	g.pdf.Ln(4)
	g.drawThinRule()
	g.pdf.Ln(4)

	// Quest stats.
	g.pdf.SetFont("Helvetica", "B", 12)
	g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.CellFormat(contentW, 7, "Quests", "", 1, "L", false, 0, "")
	g.pdf.Ln(2)

	g.drawStatRow("Total Quests", fmt.Sprintf("%d", stats.TotalQuests))
	g.drawStatRow("Active", fmt.Sprintf("%d", stats.ActiveQuests))
	g.drawStatRow("Completed", fmt.Sprintf("%d", stats.CompletedQuests))
	g.drawStatRow("Failed", fmt.Sprintf("%d", stats.FailedQuests))
	g.pdf.Ln(4)
	g.drawThinRule()
	g.pdf.Ln(4)

	// Combat stats.
	if stats.TotalEncounters > 0 {
		g.pdf.SetFont("Helvetica", "B", 12)
		g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
		g.pdf.CellFormat(contentW, 7, "Combat", "", 1, "L", false, 0, "")
		g.pdf.Ln(2)

		g.drawStatRow("Total Encounters", fmt.Sprintf("%d", stats.TotalEncounters))
		g.drawStatRow("Total Damage Dealt", formatNumber(stats.TotalDamage))
		g.pdf.Ln(4)
		g.drawThinRule()
		g.pdf.Ln(4)
	}

	// Entity counts.
	if len(stats.EntityCounts) > 0 {
		g.pdf.SetFont("Helvetica", "B", 12)
		g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
		g.pdf.CellFormat(contentW, 7, "World Building", "", 1, "L", false, 0, "")
		g.pdf.Ln(2)

		total := 0
		for _, count := range stats.EntityCounts {
			total += count
		}
		g.drawStatRow("Total Entities", fmt.Sprintf("%d", total))

		for typ, count := range stats.EntityCounts {
			label := strings.Title(typ + "s") //nolint:staticcheck
			g.drawStatRow(label, fmt.Sprintf("%d", count))
		}
		g.pdf.Ln(4)
	}

	// NPC status.
	if len(stats.NPCStatusCounts) > 0 {
		g.drawThinRule()
		g.pdf.Ln(4)
		g.pdf.SetFont("Helvetica", "B", 12)
		g.pdf.SetTextColor(colAccent[0], colAccent[1], colAccent[2])
		g.pdf.CellFormat(contentW, 7, "NPC Status", "", 1, "L", false, 0, "")
		g.pdf.Ln(2)

		for status, count := range stats.NPCStatusCounts {
			g.drawStatRow(strings.Title(status), fmt.Sprintf("%d", count)) //nolint:staticcheck
		}
	}
}

func (g *generator) drawStatRow(label, value string) {
	g.pdf.SetFont("Helvetica", "", 10)
	g.pdf.SetTextColor(colText[0], colText[1], colText[2])
	g.pdf.CellFormat(contentW*0.6, 7, label, "", 0, "L", false, 0, "")

	g.pdf.SetFont("Helvetica", "B", 10)
	g.pdf.SetTextColor(colHeader[0], colHeader[1], colHeader[2])
	g.pdf.CellFormat(contentW*0.4, 7, value, "", 1, "R", false, 0, "")
}

// ---- Drawing helpers ----

func (g *generator) drawPageBackground() {
	g.pdf.SetFillColor(colBackground[0], colBackground[1], colBackground[2])
	g.pdf.Rect(0, 0, pageW, pageH, "F")
}

func (g *generator) drawSectionTitle(title string) {
	g.pdf.SetFont("Helvetica", "B", 22)
	g.pdf.SetTextColor(colHeader[0], colHeader[1], colHeader[2])
	g.pdf.CellFormat(contentW, 12, title, "", 1, "C", false, 0, "")
	g.pdf.Ln(2)
	g.drawHorizontalRule()
}

func (g *generator) drawHorizontalRule() {
	y := g.pdf.GetY()
	g.pdf.SetDrawColor(colAccent[0], colAccent[1], colAccent[2])
	g.pdf.SetLineWidth(0.8)
	g.pdf.Line(marginL+10, y, pageW-marginR-10, y)
	g.pdf.Ln(2)
}

func (g *generator) drawThinRule() {
	y := g.pdf.GetY()
	g.pdf.SetDrawColor(colMuted[0], colMuted[1], colMuted[2])
	g.pdf.SetLineWidth(0.3)
	g.pdf.Line(marginL+20, y, pageW-marginR-20, y)
	g.pdf.Ln(1)
}

func (g *generator) drawBodyText(text string) {
	g.pdf.SetFont("Helvetica", "", 10)
	g.pdf.SetTextColor(colText[0], colText[1], colText[2])
	g.pdf.MultiCell(contentW, 5.5, text, "", "L", false)
}

func (g *generator) drawDropCapParagraph(text string) {
	if len(text) == 0 {
		return
	}

	// Get the first rune.
	firstRune, size := utf8.DecodeRuneInString(text)
	rest := text[size:]

	// Draw large first letter.
	g.pdf.SetFont("Helvetica", "B", 28)
	g.pdf.SetTextColor(colHeader[0], colHeader[1], colHeader[2])
	dropW := g.pdf.GetStringWidth(string(firstRune)) + 2
	dropH := 12.0
	dropX := marginL
	dropY := g.pdf.GetY()

	g.pdf.SetXY(dropX, dropY)
	g.pdf.CellFormat(dropW, dropH, string(firstRune), "", 0, "L", false, 0, "")

	// Draw the rest of the first line beside the drop cap.
	g.pdf.SetFont("Helvetica", "", 10)
	g.pdf.SetTextColor(colText[0], colText[1], colText[2])
	g.pdf.SetXY(dropX+dropW, dropY+3)
	remainW := contentW - dropW

	// Write the first portion next to the drop cap.
	g.pdf.MultiCell(remainW, 5.5, rest, "", "L", false)
}

func (g *generator) checkPageBreak(neededH float64) {
	if g.pdf.GetY()+neededH > pageH-marginB {
		g.pdf.AddPage()
		g.drawPageBackground()
	}
}

// ---- Utility functions ----

func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	var result []string
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 && strings.TrimSpace(text) != "" {
		result = append(result, strings.TrimSpace(text))
	}
	return result
}

func formatDuration(minutes float64) string {
	if minutes < 60 {
		return fmt.Sprintf("%.0fm", minutes)
	}
	hours := int(minutes / 60)
	mins := int(minutes) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
}

// FromStorage converts storage types into the lightweight PDF types.
func FromStorage(
	campaign *storage.Campaign,
	sessions []storage.Session,
	entities []storage.Entity,
	quests []storage.Quest,
	stats *storage.CampaignStats,
) *CampaignBook {
	book := &CampaignBook{
		Campaign: Campaign{
			Name:        campaign.Name,
			Description: campaign.Description,
			CreatedAt:   campaign.CreatedAt,
		},
		Recap: campaign.Recap,
	}

	for _, s := range sessions {
		ps := Session{
			ID:        s.ID,
			StartedAt: s.StartedAt,
			EndedAt:   s.EndedAt,
			KeyEvents: s.KeyEvents,
		}
		if s.Summary != nil {
			ps.Summary = *s.Summary
		}
		book.Sessions = append(book.Sessions, ps)
	}

	for _, e := range entities {
		book.Entities = append(book.Entities, Entity{
			Name:         e.Name,
			Type:         e.Type,
			Description:  e.Description,
			Status:       e.Status,
			CauseOfDeath: e.CauseOfDeath,
		})
	}

	for _, q := range quests {
		book.Quests = append(book.Quests, Quest{
			Name:        q.Name,
			Description: q.Description,
			Status:      q.Status,
			Giver:       q.Giver,
		})
	}

	if stats != nil {
		book.Stats = &CampaignStats{
			TotalSessions:   stats.TotalSessions,
			TotalDurationMin: stats.TotalDurationMin,
			AvgDurationMin:  stats.AvgDurationMin,
			TotalWords:      stats.TotalWords,
			TotalQuests:     stats.TotalQuests,
			ActiveQuests:    stats.ActiveQuests,
			CompletedQuests: stats.CompletedQuests,
			FailedQuests:    stats.FailedQuests,
			TotalEncounters: stats.TotalEncounters,
			TotalDamage:     stats.TotalDamage,
			EntityCounts:    stats.EntityCounts,
			NPCStatusCounts: stats.NPCStatusCounts,
		}
	}

	return book
}
