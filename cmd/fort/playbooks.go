package main

import (
	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

// wirePlaybooks installs the durable, deterministic catalog in both daemon
// modes. Full serve also exposes execution through the same FlowExecutor so a
// gated playbook can be resumed from its immutable flow identity after restart.
func wirePlaybooks(deps ui.Deps, st *store.Store, runner *control.FlowExecutor) ui.Deps {
	catalog := control.NewPlaybookCatalog(st)
	deps.Playbooks = catalog
	if runner != nil {
		withPlaybooks := runner.WithPlaybooks(catalog)
		deps.Runner = withPlaybooks
		deps.PlaybookRunner = withPlaybooks
	}
	return deps
}
