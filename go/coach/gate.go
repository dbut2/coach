package coach

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/tool"
)

func (c *Coach) planApprovalGate() llmagent.BeforeToolCallback {
	return func(tc agent.ToolContext, tl tool.Tool, args map[string]any) (map[string]any, error) {
		if tl.Name() != "update_plan_day" {
			return nil, nil
		}
		uid, err := uuid.Parse(tc.UserID())
		if err != nil {
			return nil, fmt.Errorf("coach: invalid user id %q: %w", tc.UserID(), err)
		}
		var srcMsg uuid.UUID
		if mid, err := messageID(tc); err == nil {
			srcMsg = mid
		}
		return c.gateUpdatePlanDay(tc, uid, args, srcMsg)
	}
}

func (c *Coach) gateUpdatePlanDay(ctx context.Context, uid uuid.UUID, args map[string]any, srcMsg uuid.UUID) (map[string]any, error) {
	plan, err := c.store.ActivePlan(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("coach: gate load plan: %w", err)
	}
	if plan == nil {
		return nil, nil
	}

	in, err := decodePlanDay(args)
	if err != nil {
		return nil, fmt.Errorf("coach: gate decode change: %w", err)
	}
	if _, err := in.toPlanDay(); err != nil {
		return nil, err
	}
	diff, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	rationale := fmt.Sprintf("%s → %s", in.Date, in.Workout)
	id, err := c.store.CreateProposal(ctx, uid, plan.ID, rationale, diff, srcMsg)
	if err != nil {
		return nil, fmt.Errorf("coach: gate record proposal: %w", err)
	}
	return map[string]any{
		"status":      "proposed",
		"proposal_id": id.String(),
		"note":        "Change recorded as a proposal pending the runner's approval in the app. Tell them you've proposed it, not that it's locked in.",
	}, nil
}
