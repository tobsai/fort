package handler

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/postgres"
)

const cronSchedulerID = "fort-cloud"

type scheduleStore interface {
	controlapi.ScheduleRepository
	Close() error
}

type scheduleStoreOpener func(context.Context, string, string) (scheduleStore, error)

var productionCronEndpoint = newCronEndpoint(os.Getenv, func(ctx context.Context, databaseURL, accountID string) (scheduleStore, error) {
	return postgres.Open(ctx, databaseURL, accountID)
})

// Handler is the bounded Vercel Go Function entrypoint for
// /api/v2/cron/tick. It never starts a loop or a provider.
func Handler(response http.ResponseWriter, request *http.Request) {
	productionCronEndpoint.ServeHTTP(response, request)
}

func newCronEndpoint(getenv func(string) string, open scheduleStoreOpener) http.Handler {
	state := &cronEndpointState{getenv: getenv, open: open}
	config := controlapi.CronHandlerConfig{}
	if getenv != nil {
		config = controlapi.CronHandlerConfig{
			Secret:        getenv("CRON_SECRET"),
			AuthorityMode: getenv("FORT_AUTHORITY_MODE"),
			AccountID:     strings.TrimSpace(getenv("FORT_CRON_ACCOUNT_ID")),
			SchedulerID:   cronSchedulerID,
		}
	}
	return controlapi.CronHandler(config, state.ticker)
}

type cronEndpointState struct {
	mu     sync.Mutex
	getenv func(string) string
	open   scheduleStoreOpener
	cached controlapi.ScheduleTicker
}

func (state *cronEndpointState) ticker(ctx context.Context) (controlapi.ScheduleTicker, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.cached != nil {
		return state.cached, nil
	}
	if state.getenv == nil || state.open == nil {
		return nil, controlapi.ErrAssertionConfiguration
	}
	databaseURL := strings.TrimSpace(state.getenv("DATABASE_URL"))
	accountID := strings.TrimSpace(state.getenv("FORT_CRON_ACCOUNT_ID"))
	parsedAccount, err := uuid.Parse(accountID)
	if databaseURL == "" || err != nil || parsedAccount.String() != accountID {
		return nil, controlapi.ErrAssertionConfiguration
	}
	store, err := state.open(ctx, databaseURL, accountID)
	if err != nil {
		return nil, err
	}
	state.cached = controlapi.ScheduleTickService{
		Repository: store,
		Clock:      time.Now,
		TickIDs:    uuid.NewString,
	}
	return state.cached, nil
}
