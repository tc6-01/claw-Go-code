package openai

import (
	"context"

	"claude-go-code/pkg/types"
)

type Client interface {
	CreateResponse(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error)
	StreamResponses(ctx context.Context, req *types.MessageRequest) ([]Event, error)
}

type stubClient struct{}

func NewStubClient() Client {
	return stubClient{}
}

func (stubClient) CreateResponse(_ context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	last := ""
	if len(req.Messages) > 0 {
		last = req.Messages[len(req.Messages)-1].Content
	}
	return &types.MessageResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "openai stub response: " + last,
		},
		Usage: types.Usage{
			InputTokens:  len(req.Messages),
			OutputTokens: 1,
			TotalTokens:  len(req.Messages) + 1,
		},
		StopReason: "stop",
	}, nil
}

func (stubClient) StreamResponses(_ context.Context, req *types.MessageRequest) ([]Event, error) {
	resp, err := stubClient{}.CreateResponse(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return []Event{
		{Type: EventTypeResponseDelta, DeltaText: resp.Message.Content},
		{Type: EventTypeResponseUsage, Usage: &resp.Usage},
		{Type: EventTypeResponseComplete},
	}, nil
}
