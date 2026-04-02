package anthropic

import (
	"context"

	"claude-go-code/pkg/types"
)

type Client interface {
	CreateMessage(ctx context.Context, req *types.MessageRequest) (*types.MessageResponse, error)
	StreamMessages(ctx context.Context, req *types.MessageRequest) ([]Event, error)
}

type stubClient struct{}

func NewStubClient() Client {
	return stubClient{}
}

func (stubClient) CreateMessage(_ context.Context, req *types.MessageRequest) (*types.MessageResponse, error) {
	last := ""
	if len(req.Messages) > 0 {
		last = req.Messages[len(req.Messages)-1].Content
	}
	return &types.MessageResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "anthropic stub response: " + last,
		},
		Usage: types.Usage{
			InputTokens:  len(req.Messages),
			OutputTokens: 1,
			TotalTokens:  len(req.Messages) + 1,
		},
		StopReason: "end_turn",
	}, nil
}

func (stubClient) StreamMessages(_ context.Context, req *types.MessageRequest) ([]Event, error) {
	resp, err := stubClient{}.CreateMessage(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return []Event{
		{Type: EventTypeMessageDelta, DeltaText: resp.Message.Content},
		{Type: EventTypeMessageStop, Usage: &resp.Usage},
	}, nil
}
