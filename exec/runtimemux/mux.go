// Package runtimemux keeps legacy execution and the isolated Primary Channel
// subscription lane mutually exclusive.
package runtimemux

import (
	"context"
	"fmt"
	"strings"

	"github.com/tobsai/fort/core/runtime"
)

type Runtime struct {
	legacy       runtime.Runtime
	subscription runtime.Runtime
}

func New(legacy, subscription runtime.Runtime) *Runtime {
	return &Runtime{legacy: legacy, subscription: subscription}
}

func (r *Runtime) Name() string { return "local-runtime-mux" }

func (r *Runtime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	if err := spec.ValidateAuthority(); err != nil {
		return nil, fmt.Errorf("runtime mux: %s", runtime.ErrorChatPolicyUnavailable)
	}
	switch spec.Authority {
	case "":
		if spec.Agent == "codex-subscription" || strings.HasPrefix(spec.Profile, "codex-subscription:") ||
			r == nil || r.legacy == nil {
			return nil, fmt.Errorf("runtime mux: legacy runtime unavailable")
		}
		return r.legacy.Dispatch(ctx, spec)
	case runtime.AuthorityChatSubscriptionIsolatedV1:
		if spec.Agent != "codex-subscription" || r == nil || r.subscription == nil {
			return nil, fmt.Errorf("runtime mux: %s", runtime.ErrorChatPolicyUnavailable)
		}
		return r.subscription.Dispatch(ctx, spec)
	default:
		return nil, fmt.Errorf("runtime mux: %s", runtime.ErrorChatPolicyUnavailable)
	}
}
