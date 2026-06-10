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
	Role             string   `json:"role,omitempty"`
	RoadmapStepID    string   `json:"roadmap_step_id,omitempty"`
}

type DailyTheme struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Tone    string `json:"tone"`
	Focus   string `json:"focus"`
}

type QuestCandidateResponse struct {
	DailyTheme *DailyTheme      `json:"daily_theme,omitempty"`
	Quests     []QuestCandidate `json:"quests"`
}
