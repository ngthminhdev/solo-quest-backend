package quest_generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrNoCandidates is returned when a response parses successfully but contains
// no quest candidates (empty wrapper {"quests": []} or empty array []). Callers
// can use errors.Is to distinguish "AI produced nothing usable" (clean
// fallback) from a hard parse failure.
var ErrNoCandidates = errors.New("parsed response contains no quests")

// ParseQuestCandidateResponse tolerantly parses an AI quest response. It accepts
// both the wrapper object form {"quests": [ ... ]} and a bare array form [ ... ],
// and strips a leading markdown code fence if present. An empty result (either
// form) returns ErrNoCandidates so the generation service can fall back cleanly.
func ParseQuestCandidateResponse(raw string) (*QuestCandidateResponse, error) {
	trimmed := strings.TrimSpace(raw)

	// Strip markdown code fences if present
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			var contentLines []string
			for _, line := range lines[1:] {
				if strings.HasPrefix(strings.TrimSpace(line), "```") {
					break
				}
				contentLines = append(contentLines, line)
			}
			trimmed = strings.TrimSpace(strings.Join(contentLines, "\n"))
		}
	}

	if trimmed == "" {
		return nil, fmt.Errorf("empty raw input")
	}

	// Raw array form: [ { ... }, ... ]
	if strings.HasPrefix(trimmed, "[") {
		var quests []QuestCandidate
		if err := json.Unmarshal([]byte(trimmed), &quests); err != nil {
			return nil, fmt.Errorf("failed to parse JSON array: %w", err)
		}
		if len(quests) == 0 {
			return nil, ErrNoCandidates
		}
		normalizeCandidateAliases(quests)
		return &QuestCandidateResponse{Quests: quests}, nil
	}

	// Wrapper object form: { "quests": [ ... ] }
	var response QuestCandidateResponse
	if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(response.Quests) == 0 {
		return nil, ErrNoCandidates
	}

	normalizeCandidateAliases(response.Quests)
	return &response, nil
}

func normalizeCandidateAliases(candidates []QuestCandidate) {
	for i := range candidates {
		if strings.TrimSpace(candidates[i].ReminderTime) == "" {
			candidates[i].ReminderTime = strings.TrimSpace(candidates[i].ScheduledTime)
		}
		if strings.TrimSpace(candidates[i].Instruction) == "" {
			candidates[i].Instruction = strings.TrimSpace(candidates[i].CompletionCondition)
		}
		candidates[i].Type = NormalizeType(candidates[i].Type)
		if candidates[i].Tags == nil {
			candidates[i].Tags = []string{NormalizeType(candidates[i].Type)}
		}
	}
}
