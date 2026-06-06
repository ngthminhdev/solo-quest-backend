package quest_generation

type QuestCandidate struct {
	Type             string   `json:"type"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Difficulty       string   `json:"difficulty"`
	EstimatedMinutes int      `json:"estimated_minutes"`
	XPReward         int      `json:"xp_reward"`
	Tags             []string `json:"tags"`
	Reason           string   `json:"reason"`
	Instruction      string   `json:"instruction"`
	ReminderTime     string   `json:"reminder_time"`
}

type QuestCandidateResponse struct {
	Quests []QuestCandidate `json:"quests"`
}
