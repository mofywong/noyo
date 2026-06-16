package assistant

import (
	"context"
	"errors"
	"fmt"
	"io"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type RuntimeConfig struct {
	Provider ProviderConfig
	Tools    []ToolDefinition
}

type Runtime struct {
	chatModel einomodel.ToolCallingChatModel
	tools     *ToolRegistry
}

func NewRuntime(ctx context.Context, cfg RuntimeConfig) (*Runtime, error) {
	chatModel, err := newOpenAICompatibleChatModel(ctx, cfg.Provider)
	if err != nil {
		return nil, err
	}

	registry := NewToolRegistry()
	for _, def := range cfg.Tools {
		if err := registry.Register(def); err != nil {
			return nil, err
		}
	}

	return &Runtime{
		chatModel: chatModel,
		tools:     registry,
	}, nil
}

func (r *Runtime) ChatModel() einomodel.ToolCallingChatModel {
	if r == nil {
		return nil
	}
	return r.chatModel
}

func (r *Runtime) ToolRegistry() *ToolRegistry {
	if r == nil {
		return nil
	}
	return r.tools
}

func (r *Runtime) Generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	model, err := r.modelWithTools(ctx)
	if err != nil {
		return nil, err
	}
	return model.Generate(ctx, messages)
}

func (r *Runtime) Stream(ctx context.Context, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	model, err := r.modelWithTools(ctx)
	if err != nil {
		return nil, err
	}
	return model.Stream(ctx, messages)
}

func (r *Runtime) modelWithTools(ctx context.Context) (einomodel.ToolCallingChatModel, error) {
	model := r.chatModel
	if r.tools != nil {
		toolInfos, err := r.tools.ToolInfos(ctx)
		if err != nil {
			return nil, err
		}
		if len(toolInfos) > 0 {
			model, err = r.chatModel.WithTools(toolInfos)
			if err != nil {
				return nil, err
			}
		}
	}
	return model, nil
}

func (r *Runtime) RunToolLoop(ctx context.Context, messages []*schema.Message, maxTurns int) (*schema.Message, error) {
	if maxTurns <= 0 {
		maxTurns = 1
	}

	conversation := append([]*schema.Message(nil), messages...)
	for turn := 0; turn < maxTurns; turn++ {
		msg, err := r.Generate(ctx, conversation)
		if err != nil {
			return nil, err
		}
		if len(msg.ToolCalls) == 0 {
			return msg, nil
		}
		if r.tools == nil {
			return nil, fmt.Errorf("assistant model requested tools but no tool registry is configured")
		}

		toolNode, err := r.tools.NewToolsNode(ctx)
		if err != nil {
			return nil, err
		}
		toolMessages, err := toolNode.Invoke(ctx, msg)
		if err != nil {
			return nil, err
		}
		for _, toolMessage := range toolMessages {
			NotifyToolResult(ctx, toolMessage)
		}

		conversation = append(conversation, msg)
		conversation = append(conversation, toolMessages...)
	}

	return nil, fmt.Errorf("assistant tool loop exceeded %d turns", maxTurns)
}

func (r *Runtime) RunStreamToolLoop(ctx context.Context, messages []*schema.Message, maxTurns int, emit func(*schema.Message) error) error {
	if maxTurns <= 0 {
		maxTurns = 1
	}

	conversation := append([]*schema.Message(nil), messages...)
	for turn := 0; turn < maxTurns; turn++ {
		stream, err := r.Stream(ctx, conversation)
		if err != nil {
			return err
		}

		chunks := make([]*schema.Message, 0)
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				stream.Close()
				return err
			}
			if chunk == nil {
				continue
			}
			chunks = append(chunks, chunk)
			if emit != nil && (chunk.Content != "" || chunk.ReasoningContent != "") {
				if err := emit(chunk); err != nil {
					stream.Close()
					return err
				}
			}
		}
		stream.Close()

		msg, err := schema.ConcatMessages(chunks)
		if err != nil {
			return err
		}
		if len(msg.ToolCalls) == 0 {
			return nil
		}
		if r.tools == nil {
			return fmt.Errorf("assistant model requested tools but no tool registry is configured")
		}

		toolNode, err := r.tools.NewToolsNode(ctx)
		if err != nil {
			return err
		}
		toolMessages, err := toolNode.Invoke(ctx, msg)
		if err != nil {
			return err
		}
		for _, toolMessage := range toolMessages {
			NotifyToolResult(ctx, toolMessage)
		}

		conversation = append(conversation, msg)
		conversation = append(conversation, toolMessages...)
	}

	return fmt.Errorf("assistant stream tool loop exceeded %d turns", maxTurns)
}
