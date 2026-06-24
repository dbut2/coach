package coach

import (
	"context"
	"encoding/json"
	"iter"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const maxTokens = 8192

type claudeLLM struct {
	client anthropic.Client
	model  anthropic.Model
	tel    *telemetry
}

func newClaudeModel(apiKey, modelName string, tel *telemetry) model.LLM {
	return &claudeLLM{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  anthropic.Model(modelName),
		tel:    tel,
	}
}

func (m *claudeLLM) Name() string { return string(m.model) }

func (m *claudeLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		params, err := m.buildParams(req)
		if err != nil {
			yield(nil, err)
			return
		}

		if stream {
			s := m.client.Messages.NewStreaming(ctx, params)
			acc := anthropic.Message{}
			for s.Next() {
				if err := acc.Accumulate(s.Current()); err != nil {
					yield(nil, err)
					return
				}
			}
			if err := s.Err(); err != nil {
				yield(nil, err)
				return
			}
			m.tel.recordModel(ctx, string(acc.Model), acc.Usage)
			yield(toLLMResponse(&acc), nil)
			return
		}

		msg, err := m.client.Messages.New(ctx, params)
		if err != nil {
			yield(nil, err)
			return
		}
		m.tel.recordModel(ctx, string(msg.Model), msg.Usage)
		yield(toLLMResponse(msg), nil)
	}
}

func (m *claudeLLM) buildParams(req *model.LLMRequest) (anthropic.MessageNewParams, error) {
	params := anthropic.MessageNewParams{
		Model:     m.model,
		MaxTokens: maxTokens,
	}

	if req.Config != nil {
		if si := req.Config.SystemInstruction; si != nil {
			var b strings.Builder
			for _, p := range si.Parts {
				b.WriteString(p.Text)
			}
			if b.Len() > 0 {
				params.System = []anthropic.TextBlockParam{{
					Text:         b.String(),
					CacheControl: anthropic.NewCacheControlEphemeralParam(),
				}}
			}
		}

		for _, t := range req.Config.Tools {
			for _, fd := range t.FunctionDeclarations {
				tool, err := toAnthropicTool(fd)
				if err != nil {
					return params, err
				}
				params.Tools = append(params.Tools, tool)
			}
		}
	}

	msgs, err := toAnthropicMessages(req.Contents)
	if err != nil {
		return params, err
	}
	cacheLastBlock(msgs)
	params.Messages = msgs

	return params, nil
}

func cacheLastBlock(msgs []anthropic.MessageParam) {
	if len(msgs) == 0 {
		return
	}
	blocks := msgs[len(msgs)-1].Content
	if len(blocks) == 0 {
		return
	}
	cc := anthropic.NewCacheControlEphemeralParam()
	switch b := &blocks[len(blocks)-1]; {
	case b.OfText != nil:
		b.OfText.CacheControl = cc
	case b.OfToolUse != nil:
		b.OfToolUse.CacheControl = cc
	case b.OfToolResult != nil:
		b.OfToolResult.CacheControl = cc
	}
}

func toAnthropicMessages(contents []*genai.Content) ([]anthropic.MessageParam, error) {
	var msgs []anthropic.MessageParam
	for _, c := range contents {
		var blocks []anthropic.ContentBlockParamUnion
		for _, p := range c.Parts {
			switch {
			case p.FunctionCall != nil:
				blocks = append(blocks, anthropic.NewToolUseBlock(p.FunctionCall.ID, p.FunctionCall.Args, p.FunctionCall.Name))
			case p.FunctionResponse != nil:
				payload, err := json.Marshal(p.FunctionResponse.Response)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, anthropic.NewToolResultBlock(p.FunctionResponse.ID, string(payload), false))
			case p.Text != "":
				blocks = append(blocks, anthropic.NewTextBlock(p.Text))
			}
		}
		if len(blocks) == 0 {
			continue
		}
		if c.Role == "model" {
			msgs = append(msgs, anthropic.NewAssistantMessage(blocks...))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(blocks...))
		}
	}
	return msgs, nil
}

func toAnthropicTool(fd *genai.FunctionDeclaration) (anthropic.ToolUnionParam, error) {
	schema, err := toInputSchema(fd)
	if err != nil {
		return anthropic.ToolUnionParam{}, err
	}
	t := anthropic.ToolParam{
		Name:        fd.Name,
		Description: anthropic.String(fd.Description),
		InputSchema: schema,
	}
	return anthropic.ToolUnionParam{OfTool: &t}, nil
}

func toInputSchema(fd *genai.FunctionDeclaration) (anthropic.ToolInputSchemaParam, error) {
	schema := anthropic.ToolInputSchemaParam{Properties: map[string]any{}}

	var raw any
	switch {
	case fd.ParametersJsonSchema != nil:
		raw = fd.ParametersJsonSchema
	case fd.Parameters != nil:
		raw = fd.Parameters
	default:
		return schema, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return schema, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return schema, err
	}

	if props, ok := obj["properties"]; ok {
		schema.Properties = props
	}
	if req, ok := obj["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}
	return schema, nil
}

func toLLMResponse(msg *anthropic.Message) *model.LLMResponse {
	content := &genai.Content{Role: "model"}
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			content.Parts = append(content.Parts, &genai.Part{Text: b.Text})
		case anthropic.ToolUseBlock:
			var args map[string]any
			_ = json.Unmarshal([]byte(b.Input), &args)
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{ID: b.ID, Name: b.Name, Args: args},
			})
		}
	}

	return &model.LLMResponse{
		Content:      content,
		ModelVersion: string(msg.Model),
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(msg.Usage.InputTokens),
			CandidatesTokenCount: int32(msg.Usage.OutputTokens),
			TotalTokenCount:      int32(msg.Usage.InputTokens + msg.Usage.OutputTokens),
		},
	}
}
