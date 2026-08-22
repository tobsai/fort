package postgres

import (
	"context"

	"github.com/tobsai/fort/cloud/controlapi"
)

var _ controlapi.WorkerContextRepository = (*SharedPool)(nil)

func (pool *SharedPool) ReadWorkerContextPage(ctx context.Context, command controlapi.WorkerContextPageCommand) (controlapi.WorkerContextPage, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerContextPage{}, err
	}
	return store.ReadWorkerContextPage(ctx, command)
}
