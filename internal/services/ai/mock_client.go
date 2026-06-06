package ai

import (
	"context"
)

type MockClient struct {
	ResponseText string
	Model        string
	FinishReason string
	Err          error
}

func NewMockClient(responseText string, err error) *MockClient {
	return &MockClient{
		ResponseText: responseText,
		Model:        "mock-model",
		FinishReason: "stop",
		Err:          err,
	}
}

func (m *MockClient) GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &GenerateTextResponse{
		Text:         m.ResponseText,
		Model:        m.Model,
		LatencyMs:    5,
		FinishReason: m.FinishReason,
	}, nil
}
