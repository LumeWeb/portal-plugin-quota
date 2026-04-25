package core

import (
	"context"
	"time"

	portalCore "go.lumeweb.com/portal/core"
)

const EventQuotaPlanChanged = "quota.plan.changed"

type QuotaPlanChangedEvent struct {
	Ctx       context.Context `json:"-"`
	UserID    uint            `json:"user_id"`
	OldPlanID *uint64         `json:"old_plan_id,omitempty"`
	NewPlanID *uint64         `json:"new_plan_id,omitempty"`
	ChangedAt time.Time       `json:"changed_at"`
}

func NewQuotaPlanChangedEvent(ctx context.Context, userID uint, oldPlanID, newPlanID *uint64) *QuotaPlanChangedEvent {
	return &QuotaPlanChangedEvent{
		Ctx:       ctx,
		UserID:    userID,
		OldPlanID: oldPlanID,
		NewPlanID: newPlanID,
		ChangedAt: time.Now(),
	}
}

func OnQuotaPlanChanged(ctx portalCore.Context, handler func(context.Context, QuotaPlanChangedEvent) error, priority ...int) {
	portalCore.Listen[QuotaPlanChangedEvent](ctx, EventQuotaPlanChanged, func(e *portalCore.CoreEvent[QuotaPlanChangedEvent]) error {
		return handler(e.Data.Ctx, e.Data)
	}, priority...)
}
