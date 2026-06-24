package coach

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
)

type modelPricing struct {
	input, output, cacheWrite, cacheRead float64
}

var pricing = map[string]modelPricing{
	"claude-opus-4-8":   {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-opus-4-7":   {input: 5, output: 25, cacheWrite: 6.25, cacheRead: 0.5},
	"claude-sonnet-4-6": {input: 3, output: 15, cacheWrite: 3.75, cacheRead: 0.3},
	"claude-haiku-4-5":  {input: 1, output: 5, cacheWrite: 1.25, cacheRead: 0.1},
}

func costUSD(model string, u anthropic.Usage) float64 {
	p, ok := pricing[model]
	if !ok {
		return 0
	}
	const perMTok = 1e6
	return (float64(u.InputTokens)*p.input +
		float64(u.OutputTokens)*p.output +
		float64(u.CacheCreationInputTokens)*p.cacheWrite +
		float64(u.CacheReadInputTokens)*p.cacheRead) / perMTok
}

type telemetry struct {
	tokens metric.Int64Counter
	cost   metric.Float64Counter
	tools  metric.Int64Counter
}

func newTelemetry() (*telemetry, error) {
	m := otel.Meter("naomi.run/coach")
	tokens, err := m.Int64Counter("coach.model.tokens",
		metric.WithDescription("Tokens consumed by the coach model, by type."),
		metric.WithUnit("{token}"))
	if err != nil {
		return nil, err
	}
	cost, err := m.Float64Counter("coach.model.cost",
		metric.WithDescription("Estimated coach model spend."),
		metric.WithUnit("USD"))
	if err != nil {
		return nil, err
	}
	tools, err := m.Int64Counter("coach.tool.calls",
		metric.WithDescription("Coach tool invocations, by tool and status."))
	if err != nil {
		return nil, err
	}
	return &telemetry{tokens: tokens, cost: cost, tools: tools}, nil
}

func (t *telemetry) recordModel(ctx context.Context, model string, u anthropic.Usage) {
	if t == nil {
		return
	}
	m := attribute.String("gen_ai.request.model", model)
	add := func(n int64, kind string) {
		if n > 0 {
			t.tokens.Add(ctx, n, metric.WithAttributes(m, attribute.String("type", kind)))
		}
	}
	add(u.InputTokens, "input")
	add(u.OutputTokens, "output")
	add(u.CacheReadInputTokens, "cache_read")
	add(u.CacheCreationInputTokens, "cache_write")
	t.cost.Add(ctx, costUSD(model, u), metric.WithAttributes(m))
}

func (t *telemetry) afterTool(ctx agent.ToolContext, tl tool.Tool, _, _ map[string]any, err error) (map[string]any, error) {
	if t != nil {
		status := "ok"
		if err != nil {
			status = "error"
		}
		t.tools.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tool", tl.Name()),
			attribute.String("status", status)))
	}
	return nil, nil
}
