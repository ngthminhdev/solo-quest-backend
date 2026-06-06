package ai

type GenerateTextRequest struct {
	SystemPrompt string  `json:"system_prompt"`
	UserPrompt   string  `json:"user_prompt"`
	Temperature  float32 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
}

type GenerateTextResponse struct {
	Text         string `json:"text"`
	Model        string `json:"model"`
	LatencyMs    int    `json:"latency_ms"`
	FinishReason string `json:"finish_reason"`
}
