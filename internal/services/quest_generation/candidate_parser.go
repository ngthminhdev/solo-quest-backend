package quest_generation

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

	var response QuestCandidateResponse
	if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(response.Quests) == 0 {
		return nil, fmt.Errorf("parsed response contains no quests")
	}

	return &response, nil
}
