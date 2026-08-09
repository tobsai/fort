package capability

import "time"

// OfferState is the closed public state of a profile, logical capability, or
// execution binding.
type OfferState string

const (
	OfferReady         OfferState = "ready"
	OfferSetupRequired OfferState = "setup_required"
	OfferUnavailable   OfferState = "unavailable"
	OfferUnknown       OfferState = "unknown"
)

// Reason is a safe public reason code. It never includes probe output.
type Reason string

const (
	ReasonAbsent                  Reason = "absent"
	ReasonAuthRequired            Reason = "auth_required"
	ReasonCancellationUnconfirmed Reason = "cancellation_unconfirmed"
	ReasonCapabilityDrift         Reason = "capability_drift"
	ReasonCommandContractChanged  Reason = "command_contract_changed"
	ReasonDispatchStateUnknown    Reason = "dispatch_state_unknown"
	ReasonHandoffLimitExceeded    Reason = "handoff_limit_exceeded"
	ReasonHandoffStateUnknown     Reason = "handoff_state_unknown"
	ReasonIncompatibleVersion     Reason = "incompatible_version"
	ReasonModelUnavailable        Reason = "model_unavailable"
	ReasonNoExecutionPlane        Reason = "no_execution_plane"
	ReasonOldNode                 Reason = "old_node"
	ReasonOutputLimitExceeded     Reason = "output_limit_exceeded"
	ReasonPlannerFailed           Reason = "planner_failed"
	ReasonPlannerInvalidOutput    Reason = "planner_invalid_output"
	ReasonPlannerTimedOut         Reason = "planner_timed_out"
	ReasonPluginUnready           Reason = "plugin_unready"
	ReasonProbeFailed             Reason = "probe_failed"
	ReasonProbeTimedOut           Reason = "probe_timed_out"
	ReasonProfileUnmapped         Reason = "profile_unmapped"
	ReasonProjectUnavailable      Reason = "project_unavailable"
	ReasonRuntimeFailed           Reason = "runtime_failed"
	ReasonSetupNotAutomated       Reason = "setup_not_automated"
	ReasonSolverLimitExceeded     Reason = "solver_limit_exceeded"
	ReasonStale                   Reason = "stale"
	ReasonStaticDAGUnsupported    Reason = "static_dag_unsupported"
	ReasonUnavailable             Reason = "unreachable"
	ReasonUnsupportedPlatform     Reason = "unsupported_platform"
)

var reasonOrder = []Reason{
	ReasonUnsupportedPlatform,
	ReasonNoExecutionPlane,
	ReasonOldNode,
	ReasonDispatchStateUnknown,
	ReasonHandoffStateUnknown,
	ReasonUnavailable,
	ReasonStale,
	ReasonAbsent,
	ReasonIncompatibleVersion,
	ReasonCommandContractChanged,
	ReasonAuthRequired,
	ReasonCancellationUnconfirmed,
	ReasonProfileUnmapped,
	ReasonModelUnavailable,
	ReasonProjectUnavailable,
	ReasonPluginUnready,
	ReasonPlannerTimedOut,
	ReasonPlannerFailed,
	ReasonPlannerInvalidOutput,
	ReasonRuntimeFailed,
	ReasonProbeTimedOut,
	ReasonProbeFailed,
	ReasonSetupNotAutomated,
	ReasonStaticDAGUnsupported,
	ReasonSolverLimitExceeded,
	ReasonOutputLimitExceeded,
	ReasonHandoffLimitExceeded,
	ReasonCapabilityDrift,
}

// FirstReason chooses the first closed reason in the normative total order.
func FirstReason(reasons ...Reason) Reason {
	present := make(map[Reason]bool, len(reasons))
	for _, reason := range reasons {
		present[reason] = true
	}
	for _, reason := range reasonOrder {
		if present[reason] {
			return reason
		}
	}
	return ""
}

type PredicateResolution string

const (
	ResolutionProbe   PredicateResolution = "probe"
	ResolutionDerived PredicateResolution = "derived"
)

type PredicateState string

const (
	PredicateSatisfied   PredicateState = "satisfied"
	PredicateUnsatisfied PredicateState = "unsatisfied"
	PredicateBlocked     PredicateState = "blocked"
)

// Predicate is one complete, public, non-secret readiness predicate.
type Predicate struct {
	ID              string              `json:"id"`
	Resolution      PredicateResolution `json:"resolution"`
	State           PredicateState      `json:"state"`
	Reason          Reason              `json:"reason"`
	DependsOn       []string            `json:"depends_on"`
	RemedyEffectIDs []string            `json:"remedy_effect_ids"`
}

type ProfileOffer struct {
	ID              string      `json:"id"`
	Agent           string      `json:"agent"`
	Adapter         string      `json:"adapter"`
	ResolvedModel   string      `json:"resolved_model,omitempty"`
	State           OfferState  `json:"state"`
	BindingRevision string      `json:"binding_revision"`
	Reason          Reason      `json:"reason"`
	Predicates      []Predicate `json:"predicates"`
}

type LogicalOffer struct {
	ID               string      `json:"id"`
	Adapter          string      `json:"adapter"`
	State            OfferState  `json:"state"`
	BindingRevision  string      `json:"binding_revision"`
	AvailableThrough []string    `json:"available_through"`
	Reason           Reason      `json:"reason"`
	Predicates       []Predicate `json:"predicates"`
}

type ExecutionBindingOffer struct {
	ID              string      `json:"id"`
	Profile         string      `json:"profile"`
	Capabilities    []string    `json:"capabilities"`
	State           OfferState  `json:"state"`
	BindingRevision string      `json:"binding_revision"`
	Reason          Reason      `json:"reason"`
	Predicates      []Predicate `json:"predicates"`
}

type MachineState string

const (
	MachineReady   MachineState = "ready"
	MachinePartial MachineState = "partial"
	MachineUnknown MachineState = "unknown"
)

type MachineInventory struct {
	Name                  string                  `json:"name"`
	Local                 bool                    `json:"local"`
	RegistryRank          int                     `json:"registry_rank"`
	Reachable             bool                    `json:"reachable"`
	ProtocolVersion       int                     `json:"protocol_version"`
	CatalogVersion        int                     `json:"catalog_version"`
	ProfileMappingVersion int                     `json:"profile_mapping_version"`
	State                 MachineState            `json:"state"`
	Reason                Reason                  `json:"reason"`
	ObservedAt            time.Time               `json:"observed_at"`
	Profiles              []ProfileOffer          `json:"profiles"`
	Offers                []LogicalOffer          `json:"offers"`
	Bindings              []ExecutionBindingOffer `json:"bindings"`
	TextOnlyOptions       []TextOnlyOptionOffer   `json:"text_only_options"`
}

type Snapshot struct {
	CatalogVersion        int                `json:"catalog_version"`
	ProfileMappingVersion int                `json:"profile_mapping_version"`
	Revision              string             `json:"revision"`
	ObservedAt            time.Time          `json:"observed_at"`
	LocalMachine          string             `json:"local_machine"`
	Machines              []MachineInventory `json:"machines"`
}

// NodeInventory is the node-owned mesh payload. Public machine naming, local
// ownership, registry rank, reachability, and receipt freshness are assigned
// only by the coordinator.
type NodeInventory struct {
	ProtocolVersion       int                     `json:"protocol_version"`
	CatalogVersion        int                     `json:"catalog_version"`
	ProfileMappingVersion int                     `json:"profile_mapping_version"`
	NodeID                string                  `json:"node_id"`
	ObservedAt            time.Time               `json:"observed_at"`
	State                 MachineState            `json:"state"`
	Reason                Reason                  `json:"reason"`
	Profiles              []ProfileOffer          `json:"profiles"`
	Offers                []LogicalOffer          `json:"offers"`
	Bindings              []ExecutionBindingOffer `json:"bindings"`
	TextOnlyOptions       []TextOnlyOptionOffer   `json:"text_only_options"`
}

type RefreshMode string

const (
	RefreshPlanning    RefreshMode = "planning"
	RefreshUserRecheck RefreshMode = "user_recheck"
)

type RecheckRequest struct {
	ProtocolVersion int         `json:"protocol_version"`
	RequestID       string      `json:"request_id"`
	Mode            RefreshMode `json:"mode"`
	MaxAgeSeconds   int         `json:"max_age_seconds"`
	Adapters        []string    `json:"adapters"`
}
