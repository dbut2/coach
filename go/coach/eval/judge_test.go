//go:build eval

package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const judgeSystem = `You are a meticulous, skeptical evaluator grading a single reply from Naomi, an AI running coach, inside a chat conversation.

Grade the reply ONLY against the rubric you are given — not your own taste, and not requirements the rubric does not state. The reply passes only if it satisfies every rubric item. Be strict but fair. When the rubric concerns grounding, hold the reply to the FIXTURE ground truth: inventing data the fixture contradicts is a failure. Always answer through the record_verdict tool.`

type verdict struct {
	Pass      bool     `json:"pass"`
	Score     int      `json:"score"`
	Rationale string   `json:"rationale"`
	Failures  []string `json:"failures"`
}

func grade(ctx context.Context, t *testing.T, cfg config, rubric, transcript string) verdict {
	t.Helper()

	client := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	model := anthropic.Model(cfg.JudgeModel)

	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"pass":      map[string]any{"type": "boolean", "description": "true only if the reply satisfies every rubric item"},
			"score":     map[string]any{"type": "integer", "description": "holistic rubric adherence, 1 (poor) to 5 (excellent)"},
			"rationale": map[string]any{"type": "string", "description": "one or two sentences justifying the verdict"},
			"failures":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "each rubric item the reply violated; empty when it passes"},
		},
		Required: []string{"pass", "score", "rationale"},
	}

	prompt := fmt.Sprintf("RUBRIC — the reply must satisfy every item:\n%s\n\nCONVERSATION:\n%s", rubric, transcript)

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Text: judgeSystem}},
		Tools: []anthropic.ToolUnionParam{{OfTool: &anthropic.ToolParam{
			Name:        "record_verdict",
			Description: anthropic.String("Record your verdict on the coach's reply."),
			InputSchema: schema,
		}}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("record_verdict"),
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	})
	if err != nil {
		t.Fatalf("eval: judge call: %v", err)
	}

	for _, block := range msg.Content {
		if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
			var v verdict
			if err := json.Unmarshal(tu.Input, &v); err != nil {
				t.Fatalf("eval: judge verdict decode: %v", err)
			}
			return v
		}
	}
	t.Fatalf("eval: judge returned no verdict; content=%+v", msg.Content)
	return verdict{}
}

func assertJudgePass(t *testing.T, cfg config, rubric, transcript string) {
	t.Helper()
	v := grade(context.Background(), t, cfg, rubric, transcript)
	if !v.Pass {
		t.Errorf("eval: judge failed reply (score %d/5): %s\nfailures: %v\n--- transcript ---\n%s",
			v.Score, v.Rationale, v.Failures, transcript)
		return
	}
	t.Logf("eval: judge passed (score %d/5): %s", v.Score, v.Rationale)
}
