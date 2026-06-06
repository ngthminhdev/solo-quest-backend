package ai

import "context"

type Client interface {
	GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error)
}
