package control

import (
	"context"
	"fmt"

	"github.com/tobsai/fort/core/scheduler"
)

type ScheduleCreator interface {
	Create(context.Context, scheduler.Definition) error
}

type ScheduleService struct {
	next    ScheduleCreator
	flowIDs map[string]bool
}

func NewScheduleService(next ScheduleCreator, flowIDs []string) *ScheduleService {
	known := make(map[string]bool, len(flowIDs))
	for _, id := range flowIDs {
		known[id] = true
	}
	return &ScheduleService{next: next, flowIDs: known}
}

func (s *ScheduleService) Create(ctx context.Context, definition scheduler.Definition) error {
	if !s.flowIDs[definition.FlowID] {
		return fmt.Errorf("unknown flow %q", definition.FlowID)
	}
	return s.next.Create(ctx, definition)
}
