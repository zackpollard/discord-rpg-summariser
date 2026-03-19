package summarise

import (
	"strings"
	"testing"
)

func TestBuildLoreQAPrompt(t *testing.T) {
	question := "Who is the ruler of Barovia?"
	ctx := "Strahd von Zarovich is the vampire lord who rules over the land of Barovia."

	prompt := BuildLoreQAPrompt(question, ctx)

	if !strings.Contains(prompt, question) {
		t.Error("prompt should contain the question")
	}
	if !strings.Contains(prompt, ctx) {
		t.Error("prompt should contain the context")
	}
	if !strings.Contains(prompt, "Question:") {
		t.Error("prompt should have a Question: label")
	}
	if !strings.Contains(prompt, "campaign context") {
		t.Error("prompt should reference campaign context")
	}
	if !strings.Contains(prompt, "ONLY the provided") {
		t.Error("prompt should instruct to use only provided context")
	}
}

func TestBuildLoreQAPrompt_JSONInstruction(t *testing.T) {
	prompt := BuildLoreQAPrompt("some question", "some context")

	if !strings.Contains(prompt, "valid JSON") {
		t.Error("prompt should instruct to return valid JSON")
	}
	if !strings.Contains(prompt, `"answer"`) {
		t.Error("prompt should contain JSON field instruction for answer")
	}
	if !strings.Contains(prompt, `"sources"`) {
		t.Error("prompt should contain JSON field instruction for sources")
	}
}
