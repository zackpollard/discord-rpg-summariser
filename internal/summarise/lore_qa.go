package summarise

import "strings"

// BuildLoreQAPrompt constructs a prompt for answering lore questions with
// context from the campaign's knowledge base.
func BuildLoreQAPrompt(question, context string) string {
	var b strings.Builder

	b.WriteString("You are a knowledgeable lore-keeper for a tabletop RPG campaign.\n\n")
	b.WriteString("Answer the following question using ONLY the provided campaign context. ")
	b.WriteString("If the context doesn't contain enough information, say so honestly.\n\n")

	b.WriteString("Context from the campaign knowledge base:\n")
	b.WriteString(context)
	b.WriteString("\n\n---\n\n")

	b.WriteString("Question: ")
	b.WriteString(question)
	b.WriteString("\n\n")

	b.WriteString("Return ONLY valid JSON: {\"answer\": \"Your answer here.\", \"sources\": [\"Entity or session referenced\"]}\n")

	return b.String()
}
