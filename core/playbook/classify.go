package playbook

import (
	"strings"
	"unicode"
)

// ClassifyTaskType applies fixed, inspectable text rules. The order is part of
// the contract: question, then bug, then research, with feature as the default.
func ClassifyTaskType(direction string) TaskType {
	text := strings.ToLower(strings.TrimSpace(direction))
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	has := func(set map[string]bool) bool {
		for _, word := range words {
			if set[word] {
				return true
			}
		}
		return false
	}
	first := ""
	if len(words) > 0 {
		first = words[0]
		if first == "please" && len(words) > 1 {
			first = words[1]
		}
	}
	replyImperative := map[string]bool{
		"reply": true, "respond": true, "answer": true, "say": true,
	}[first]
	if strings.Contains(text, "?") || replyImperative || has(map[string]bool{
		"what": true, "why": true, "when": true, "where": true, "who": true,
		"how": true, "can": true, "could": true, "would": true, "explain": true,
	}) {
		return TaskQuestion
	}
	if has(map[string]bool{
		"bug": true, "fix": true, "broken": true, "crash": true, "crashes": true,
		"error": true, "errors": true, "fail": true, "fails": true, "failed": true,
		"failure": true, "regression": true,
	}) {
		return TaskBug
	}
	if has(map[string]bool{
		"research": true, "investigate": true, "explore": true, "compare": true,
		"analyze": true, "analyse": true, "audit": true, "recon": true, "study": true,
	}) {
		return TaskResearch
	}
	return TaskFeature
}
