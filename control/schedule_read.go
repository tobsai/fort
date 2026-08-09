package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
)

const (
	MaxScheduleDefinitions        = 1000
	ScheduleDetailOccurrenceLimit = 10
	MaxOccurrencePageLimit        = 50
)

var (
	ErrInvalidScheduleFilter       = errors.New("invalid schedule filter")
	ErrInvalidOccurrencePage       = errors.New("invalid schedule occurrence page")
	ErrScheduleInventoryUnaccepted = errors.New("schedule_inventory_unaccepted")
	ErrScheduleInventoryDrift      = errors.New("schedule_inventory_drift")
)

type ScheduleFilter string

const (
	ScheduleFilterAll    ScheduleFilter = "all"
	ScheduleFilterActive ScheduleFilter = "active"
	ScheduleFilterPaused ScheduleFilter = "paused"
)

type SchedulerOwnership string

const (
	SchedulerOwnershipActive   SchedulerOwnership = "active"
	SchedulerOwnershipInactive SchedulerOwnership = "inactive"
	SchedulerOwnershipUnknown  SchedulerOwnership = "unknown"
)

type OccurrencePage struct {
	Limit    int
	Before   time.Time
	BeforeID string
}

type RelatedChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ScheduleItem struct {
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	Enabled            bool                  `json:"enabled"`
	Kind               scheduler.Kind        `json:"kind"`
	Expression         string                `json:"expression"`
	Recurrence         string                `json:"recurrence"`
	Timezone           string                `json:"timezone"`
	NextFireAt         *time.Time            `json:"next_fire_at"`
	LastFireAt         *time.Time            `json:"last_fire_at"`
	TargetKind         string                `json:"target_kind"`
	TargetID           string                `json:"target_id"`
	RelatedChannel     *RelatedChannel       `json:"related_channel,omitempty"`
	LatestOccurrence   *scheduler.Occurrence `json:"latest_occurrence,omitempty"`
	SchedulerOwnership SchedulerOwnership    `json:"scheduler_ownership"`
	ObservedAt         time.Time             `json:"observed_at"`
	updatedAt          time.Time
}

type ScheduleList struct {
	SnapshotID string         `json:"snapshot_id"`
	ObservedAt time.Time      `json:"observed_at"`
	Items      []ScheduleItem `json:"items"`
}

type ScheduleDetail struct {
	Item     ScheduleItem           `json:"item"`
	Upcoming []scheduler.Occurrence `json:"upcoming"`
	Recent   []scheduler.Occurrence `json:"recent"`
}

type ScheduleInventoryState string

const (
	ScheduleInventoryAccepted   ScheduleInventoryState = "accepted"
	ScheduleInventoryUnaccepted ScheduleInventoryState = "unaccepted"
	ScheduleInventoryDrift      ScheduleInventoryState = "drift"
)

type ScheduleInventoryItem struct {
	ID         string         `json:"id"`
	Kind       scheduler.Kind `json:"kind"`
	Expression string         `json:"expression"`
	Timezone   string         `json:"timezone"`
	FlowID     string         `json:"flow_id"`
	FlowDigest string         `json:"flow_digest"`
}

type ScheduleInventory struct {
	CurrentDigest  string                  `json:"current_digest"`
	AcceptedDigest string                  `json:"accepted_digest,omitempty"`
	State          ScheduleInventoryState  `json:"state"`
	Items          []ScheduleInventoryItem `json:"items"`
}

type ScheduleReadRepository interface {
	ReadScheduleCatalog(context.Context, *bool, int) ([]store.ScheduleReadRow, error)
	ReadScheduleDetail(context.Context, string, time.Time, int) (store.ScheduleReadDetail, error)
	ReadScheduleOccurrences(context.Context, string, int, time.Time, string) ([]scheduler.Occurrence, error)
}

type ScheduleReadService struct {
	repository  ScheduleReadRepository
	ownership   SchedulerOwnership
	flowDigests map[string]string
	now         func() time.Time
}

func NewScheduleReadService(repository ScheduleReadRepository, ownership SchedulerOwnership, flowDigests map[string]string) *ScheduleReadService {
	switch ownership {
	case SchedulerOwnershipActive, SchedulerOwnershipInactive, SchedulerOwnershipUnknown:
	default:
		ownership = SchedulerOwnershipUnknown
	}
	digests := make(map[string]string, len(flowDigests))
	for id, digest := range flowDigests {
		digests[id] = digest
	}
	return &ScheduleReadService{repository: repository, ownership: ownership, flowDigests: digests, now: time.Now}
}

func (s *ScheduleReadService) List(ctx context.Context, filter ScheduleFilter) (ScheduleList, error) {
	enabled, err := scheduleEnabledFilter(filter)
	if err != nil {
		return ScheduleList{}, err
	}
	rows, err := s.repository.ReadScheduleCatalog(ctx, enabled, MaxScheduleDefinitions)
	if err != nil {
		return ScheduleList{}, err
	}
	if len(rows) > MaxScheduleDefinitions {
		return ScheduleList{}, store.ErrScheduleCatalogLimit
	}
	observedAt := s.now().UTC()
	items := make([]ScheduleItem, 0, len(rows))
	for _, row := range rows {
		if enabled != nil && row.Definition.Enabled != *enabled {
			continue
		}
		items = append(items, s.scheduleItem(row, observedAt))
	}
	sortScheduleItems(items)
	snapshotID, err := scheduleSnapshotID(items)
	if err != nil {
		return ScheduleList{}, err
	}
	return ScheduleList{SnapshotID: snapshotID, ObservedAt: observedAt, Items: items}, nil
}

func (s *ScheduleReadService) Get(ctx context.Context, id string) (ScheduleDetail, error) {
	observedAt := s.now().UTC()
	detail, err := s.repository.ReadScheduleDetail(ctx, id, observedAt, ScheduleDetailOccurrenceLimit)
	if err != nil {
		return ScheduleDetail{}, err
	}
	upcoming := append([]scheduler.Occurrence(nil), detail.Upcoming...)
	recent := append([]scheduler.Occurrence(nil), detail.Recent...)
	if upcoming == nil {
		upcoming = []scheduler.Occurrence{}
	}
	if recent == nil {
		recent = []scheduler.Occurrence{}
	}
	return ScheduleDetail{Item: s.scheduleItem(detail.Row, observedAt), Upcoming: upcoming, Recent: recent}, nil
}

func (s *ScheduleReadService) Occurrences(ctx context.Context, id string, page OccurrencePage) ([]scheduler.Occurrence, error) {
	if page.Limit < 1 || page.Limit > MaxOccurrencePageLimit || page.Before.IsZero() != (page.BeforeID == "") {
		return nil, ErrInvalidOccurrencePage
	}
	items, err := s.repository.ReadScheduleOccurrences(ctx, id, page.Limit, page.Before, page.BeforeID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []scheduler.Occurrence{}
	}
	return items, nil
}

// Inventory computes the review boundary over enabled durable definitions.
// Missing or malformed loaded-flow digests fail closed as drift; they can
// never be made acceptable by copying an invalid current value.
func (s *ScheduleReadService) Inventory(ctx context.Context, acceptedDigest string) (ScheduleInventory, error) {
	enabled := true
	rows, err := s.repository.ReadScheduleCatalog(ctx, &enabled, MaxScheduleDefinitions)
	if err != nil {
		return ScheduleInventory{}, err
	}
	if len(rows) > MaxScheduleDefinitions {
		return ScheduleInventory{}, store.ErrScheduleCatalogLimit
	}
	items := make([]ScheduleInventoryItem, 0, len(rows))
	missingFlowDigest := false
	for _, row := range rows {
		definition := row.Definition
		if !definition.Enabled {
			continue
		}
		flowDigest := s.flowDigests[definition.FlowID]
		if !validVersionedDigest(flowDigest, "flow-definition:v1:") {
			flowDigest = ""
			missingFlowDigest = true
		}
		items = append(items, ScheduleInventoryItem{
			ID: definition.ID, Kind: definition.Kind, Expression: definition.Expression,
			Timezone: definition.Timezone, FlowID: definition.FlowID, FlowDigest: flowDigest,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	inventory := ScheduleInventory{AcceptedDigest: acceptedDigest, Items: items}
	payload, err := json.Marshal(items)
	if err != nil {
		return ScheduleInventory{}, err
	}
	inventory.CurrentDigest = versionedDigest("schedule-inventory:v1", payload)
	if missingFlowDigest {
		inventory.State = ScheduleInventoryDrift
		return inventory, ErrScheduleInventoryDrift
	}
	switch {
	case acceptedDigest == "":
		inventory.State = ScheduleInventoryUnaccepted
		return inventory, ErrScheduleInventoryUnaccepted
	case acceptedDigest != inventory.CurrentDigest:
		inventory.State = ScheduleInventoryDrift
		return inventory, ErrScheduleInventoryDrift
	default:
		inventory.State = ScheduleInventoryAccepted
		return inventory, nil
	}
}

// FlowDefinitionDigest hashes the canonical JSON representation of one loaded,
// validated flow. Slice order is retained because it is execution-significant.
func FlowDefinitionDigest(definition graph.Flow) (string, error) {
	if err := definition.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	return versionedDigest("flow-definition:v1", payload), nil
}

func FlowDefinitionDigests(definitions []graph.Flow) (map[string]string, error) {
	digests := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		digest, err := FlowDefinitionDigest(definition)
		if err != nil {
			return nil, err
		}
		if _, exists := digests[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate loaded flow %q", definition.ID)
		}
		digests[definition.ID] = digest
	}
	return digests, nil
}

func scheduleEnabledFilter(filter ScheduleFilter) (*bool, error) {
	switch filter {
	case "", ScheduleFilterAll:
		return nil, nil
	case ScheduleFilterActive:
		enabled := true
		return &enabled, nil
	case ScheduleFilterPaused:
		enabled := false
		return &enabled, nil
	default:
		return nil, ErrInvalidScheduleFilter
	}
}

func (s *ScheduleReadService) scheduleItem(row store.ScheduleReadRow, observedAt time.Time) ScheduleItem {
	definition := row.Definition
	item := ScheduleItem{
		ID: definition.ID, Title: definition.Title, Enabled: definition.Enabled, Kind: definition.Kind,
		Expression: definition.Expression, Recurrence: recurrenceSummary(definition), Timezone: definition.Timezone,
		NextFireAt: timePointer(definition.NextFireAt), LastFireAt: timePointer(definition.LastFireAt),
		TargetKind: "flow", TargetID: definition.FlowID, SchedulerOwnership: s.ownership, ObservedAt: observedAt,
		updatedAt: definition.UpdatedAt.UTC(),
	}
	if row.RelatedChannel != nil {
		item.RelatedChannel = &RelatedChannel{ID: row.RelatedChannel.ID, Name: row.RelatedChannel.Name}
	}
	if row.LatestOccurrence != nil {
		latest := *row.LatestOccurrence
		latest.ScheduledFor = latest.ScheduledFor.UTC()
		latest.CreatedAt = latest.CreatedAt.UTC()
		latest.UpdatedAt = latest.UpdatedAt.UTC()
		item.LatestOccurrence = &latest
	}
	return item
}

func sortScheduleItems(items []ScheduleItem) {
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		leftBucket, rightBucket := scheduleBucket(left), scheduleBucket(right)
		if leftBucket != rightBucket {
			return leftBucket < rightBucket
		}
		if leftBucket == 0 {
			if !left.NextFireAt.Equal(*right.NextFireAt) {
				return left.NextFireAt.Before(*right.NextFireAt)
			}
			return left.ID < right.ID
		}
		if !left.updatedAt.Equal(right.updatedAt) {
			return left.updatedAt.After(right.updatedAt)
		}
		return left.ID < right.ID
	})
}

func scheduleBucket(item ScheduleItem) int {
	if item.Enabled && item.NextFireAt != nil {
		return 0
	}
	if item.Enabled {
		return 1
	}
	return 2
}

func scheduleSnapshotID(items []ScheduleItem) (string, error) {
	type normalizedItem struct {
		ID                 string                `json:"id"`
		Title              string                `json:"title"`
		Enabled            bool                  `json:"enabled"`
		Kind               scheduler.Kind        `json:"kind"`
		Expression         string                `json:"expression"`
		Recurrence         string                `json:"recurrence"`
		Timezone           string                `json:"timezone"`
		NextFireAt         *time.Time            `json:"next_fire_at"`
		LastFireAt         *time.Time            `json:"last_fire_at"`
		TargetKind         string                `json:"target_kind"`
		TargetID           string                `json:"target_id"`
		RelatedChannel     *RelatedChannel       `json:"related_channel,omitempty"`
		LatestOccurrence   *scheduler.Occurrence `json:"latest_occurrence,omitempty"`
		SchedulerOwnership SchedulerOwnership    `json:"scheduler_ownership"`
	}
	normalized := make([]normalizedItem, 0, len(items))
	for _, item := range items {
		normalized = append(normalized, normalizedItem{
			ID: item.ID, Title: item.Title, Enabled: item.Enabled, Kind: item.Kind,
			Expression: item.Expression, Recurrence: item.Recurrence, Timezone: item.Timezone,
			NextFireAt: item.NextFireAt, LastFireAt: item.LastFireAt,
			TargetKind: item.TargetKind, TargetID: item.TargetID, RelatedChannel: item.RelatedChannel,
			LatestOccurrence: item.LatestOccurrence, SchedulerOwnership: item.SchedulerOwnership,
		})
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return versionedDigest("schedule-snapshot:v1", payload), nil
}

func versionedDigest(version string, payload []byte) string {
	input := make([]byte, 0, len(version)+len(payload)+2)
	input = append(input, version...)
	input = append(input, '\n')
	input = append(input, payload...)
	input = append(input, '\n')
	digest := sha256.Sum256(input)
	return version + ":" + hex.EncodeToString(digest[:])
}

func validVersionedDigest(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+sha256.Size*2 {
		return false
	}
	encoded := strings.TrimPrefix(value, prefix)
	if encoded != strings.ToLower(encoded) {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}
