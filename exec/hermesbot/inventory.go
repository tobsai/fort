package hermesbot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tobsai/fort/core/conversation"
	coreruntime "github.com/tobsai/fort/core/runtime"
)

const (
	discoveryTimeout = 5 * time.Second
	maximumResponse  = 1 << 20
	maximumProfiles  = 256
)

var (
	canonicalProfilePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	lowerSHA256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reservedProfileIDs      = map[string]struct{}{
		"hermes": {}, "test": {}, "tmp": {}, "root": {}, "sudo": {},
	}
)

type InventoryOptions struct {
	ExecutionSource conversation.ExecutionSource
	AcceptedSource  LocalSourceContract
	Transport       LocalProfileRosterTransport
	Clock           func() time.Time
	RequestID       func() string
}

type Inventory struct {
	executionSource conversation.ExecutionSource
	acceptedSource  LocalSourceContract
	transport       LocalProfileRosterTransport
	clock           func() time.Time
	requestID       func() string
}

func NewInventory(options InventoryOptions) (*Inventory, error) {
	if err := options.ExecutionSource.Validate(); err != nil {
		return nil, fmt.Errorf("Hermes inventory source: %w", err)
	}
	if options.ExecutionSource.Framework != "hermes" {
		return nil, errors.New("Hermes inventory requires the Hermes framework")
	}
	if options.ExecutionSource.ResourceSharing != conservativeResourceSharing() {
		return nil, errors.New("Hermes inventory resource-sharing disclosure is invalid")
	}
	if err := options.AcceptedSource.validate(); err != nil {
		return nil, err
	}
	if options.Transport == nil || options.Clock == nil || options.RequestID == nil {
		return nil, errors.New("Hermes inventory dependencies are required")
	}
	return &Inventory{
		executionSource: options.ExecutionSource,
		acceptedSource:  options.AcceptedSource,
		transport:       options.Transport,
		clock:           options.Clock,
		requestID:       options.RequestID,
	}, nil
}

func conservativeResourceSharing() conversation.ResourceSharingDisclosure {
	return conversation.ResourceSharingDisclosure{
		ProviderCredentials: conversation.ResourceUnknown,
		Filesystem:          conversation.ResourceMachineShared,
		BrowserSessions:     conversation.ResourceMachineShared,
		FrameworkSessions:   conversation.ResourceProfileScoped,
		SourceMemory:        conversation.ResourceUnknown,
		ToolConfiguration:   conversation.ResourceProfileScoped,
	}
}

func (contract LocalSourceContract) validate() error {
	if !normalizedNonblank(contract.ConnectionIdentity) ||
		!lowerSHA256Pattern.MatchString(contract.LauncherDigest) ||
		!lowerSHA256Pattern.MatchString(contract.CodeRootDigest) ||
		contract.HermesVersion != HermesVersion || contract.HermesRevision != HermesRevision {
		return errors.New("Hermes inventory source contract is invalid")
	}
	return nil
}

func (inventory *Inventory) Inventory(ctx context.Context, request coreruntime.AgentSourceInventoryRequest) (coreruntime.AgentSourceInventorySnapshot, error) {
	if request.ExecutionSourceID != inventory.executionSource.ID {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory source does not match request")
	}
	requestID := inventory.requestID()
	if !normalizedNonblank(requestID) {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory request ID is invalid")
	}
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  "profiles.list",
		Params:  rpcRequestParams{IncludeSessions: false},
	})
	if err != nil {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory request could not be encoded")
	}

	deadlineContext, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	if deadlineContext.Err() != nil {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory canceled")
	}
	exchange, err := inventory.transport.RoundTrip(deadlineContext, body)
	if err != nil {
		if exchange.Body != nil {
			_ = exchange.Body.Close()
		}
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory transport failed")
	}
	if deadlineContext.Err() != nil {
		if exchange.Body != nil {
			_ = exchange.Body.Close()
		}
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory canceled")
	}
	if exchange.Body == nil {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory response is missing")
	}
	responseStream := &singleCloseReadCloser{ReadCloser: exchange.Body}
	defer responseStream.Close()
	if !inventory.acceptedSource.accepts(exchange) {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory source attestation failed")
	}

	responseBody, err := readResponse(deadlineContext, responseStream)
	if err != nil || len(responseBody) > maximumResponse {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory response is invalid")
	}
	response, err := decodeResponse(responseBody, requestID)
	if err != nil {
		return coreruntime.AgentSourceInventorySnapshot{}, err
	}
	if len(response.Result.Profiles) > maximumProfiles {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory response is invalid")
	}

	now := inventory.clock().UTC()
	source := inventory.executionSource
	source.LastSeenAt = now
	agents := make([]coreruntime.SourceAgentInventory, 0, len(response.Result.Profiles))
	seen := make(map[string]struct{}, len(response.Result.Profiles))
	for _, profile := range response.Result.Profiles {
		profileID := *profile.Name
		if !canonicalProfilePattern.MatchString(profileID) {
			return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory contains an invalid profile")
		}
		if _, reserved := reservedProfileIDs[profileID]; reserved {
			return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory contains an invalid profile")
		}
		if _, duplicate := seen[profileID]; duplicate {
			return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory contains a duplicate profile")
		}
		seen[profileID] = struct{}{}
		agents = append(agents, coreruntime.SourceAgentInventory{
			SourceAgent: conversation.SourceAgent{
				ID:                  sourceAgentID(source.ID, profileID),
				ExecutionSourceID:   source.ID,
				OpaqueSourceAgentID: profileID,
				DisplayName:         displayName(profile.decodedDisplayName, profileID),
				LastSeenAt:          now,
			},
			Capabilities: make([]string, 0),
			Readiness: coreruntime.SourceAgentReadiness{
				Ready:            false,
				ContractID:       InventoryContractID,
				ContractRevision: InventoryContractRevision,
				Evidence:         []string{"profile_discovered", "execution_adapter_not_approved"},
			},
		})
	}
	sort.Slice(agents, func(left, right int) bool {
		return agents[left].SourceAgent.OpaqueSourceAgentID < agents[right].SourceAgent.OpaqueSourceAgentID
	})
	snapshot := coreruntime.AgentSourceInventorySnapshot{
		ExecutionSource: source,
		Agents:          agents,
		ObservedAt:      now,
	}
	if err := snapshot.Validate(); err != nil {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory snapshot is invalid")
	}
	if deadlineContext.Err() != nil {
		return coreruntime.AgentSourceInventorySnapshot{}, errors.New("Hermes inventory canceled")
	}
	return snapshot, nil
}

type responseReadResult struct {
	body []byte
	err  error
}

type singleCloseReadCloser struct {
	io.ReadCloser
	once sync.Once
	err  error
}

func (body *singleCloseReadCloser) Close() error {
	body.once.Do(func() { body.err = body.ReadCloser.Close() })
	return body.err
}

func readResponse(ctx context.Context, body io.ReadCloser) ([]byte, error) {
	done := make(chan responseReadResult, 1)
	go func() {
		responseBody, err := io.ReadAll(io.LimitReader(body, maximumResponse+1))
		done <- responseReadResult{body: responseBody, err: err}
	}()
	select {
	case <-ctx.Done():
		_ = body.Close()
		<-done
		return nil, ctx.Err()
	case result := <-done:
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return result.body, result.err
	}
}

func (contract LocalSourceContract) accepts(exchange VerifiedRPCExchange) bool {
	return exchange.ConnectionIdentity == contract.ConnectionIdentity &&
		exchange.LauncherDigest == contract.LauncherDigest &&
		exchange.CodeRootDigest == contract.CodeRootDigest &&
		exchange.HermesVersion == contract.HermesVersion &&
		exchange.HermesRevision == contract.HermesRevision
}

type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      string           `json:"id"`
	Method  string           `json:"method"`
	Params  rpcRequestParams `json:"params"`
}

type rpcRequestParams struct {
	IncludeSessions bool `json:"include_sessions"`
}

type rpcResponse struct {
	JSONRPC string              `json:"jsonrpc"`
	ID      string              `json:"id"`
	Result  *profilesListResult `json:"result,omitempty"`
	Error   json.RawMessage     `json:"error,omitempty"`
}

type profilesListResult struct {
	Profiles        []profileRecord `json:"profiles"`
	BotModeProtocol *bool           `json:"bot_mode_protocol"`
}

type profileRecord struct {
	Name               *string         `json:"name"`
	Path               *string         `json:"path"`
	IsDefault          *bool           `json:"is_default"`
	Model              json.RawMessage `json:"model"`
	Provider           json.RawMessage `json:"provider"`
	Description        *string         `json:"description"`
	DisplayName        json.RawMessage `json:"display_name"`
	SkillCount         *int            `json:"skill_count"`
	UIMeta             json.RawMessage `json:"ui_meta,omitempty"`
	UIMetaRevisions    json.RawMessage `json:"ui_meta_revisions,omitempty"`
	HasAvatar          *bool           `json:"has_avatar"`
	decodedDisplayName string
}

func decodeResponse(body []byte, requestID string) (rpcResponse, error) {
	var response rpcResponse
	if !utf8.Valid(body) {
		return rpcResponse{}, errors.New("Hermes inventory response is invalid")
	}
	if err := validateUniqueJSONKeys(body); err != nil {
		return rpcResponse{}, errors.New("Hermes inventory response is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return rpcResponse{}, errors.New("Hermes inventory response is invalid")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return rpcResponse{}, errors.New("Hermes inventory response is invalid")
	}
	if response.JSONRPC != "2.0" || response.ID != requestID || len(response.Error) != 0 || response.Result == nil ||
		response.Result.Profiles == nil || response.Result.BotModeProtocol == nil || !*response.Result.BotModeProtocol {
		return rpcResponse{}, errors.New("Hermes inventory response is invalid")
	}
	for index := range response.Result.Profiles {
		profile := &response.Result.Profiles[index]
		if profile.Name == nil || profile.Path == nil || profile.IsDefault == nil ||
			!nullableStringPresent(profile.Model) || !nullableStringPresent(profile.Provider) ||
			profile.Description == nil || len(profile.DisplayName) == 0 || profile.SkillCount == nil || *profile.SkillCount < 0 ||
			profile.HasAvatar == nil || !optionalJSONObject(profile.UIMeta) || !optionalJSONObject(profile.UIMetaRevisions) {
			return rpcResponse{}, errors.New("Hermes inventory response is invalid")
		}
		if err := json.Unmarshal(profile.DisplayName, &profile.decodedDisplayName); err != nil {
			return rpcResponse{}, errors.New("Hermes inventory response is invalid")
		}
		if !validJSONStringScalars(profile.DisplayName) {
			profile.decodedDisplayName = ""
		}
	}
	return response, nil
}

func optionalJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

func validJSONStringScalars(raw []byte) bool {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' || !utf8.Valid(raw) {
		return false
	}
	for index := 1; index < len(raw)-1; {
		if raw[index] != '\\' {
			_, size := utf8.DecodeRune(raw[index:])
			if size == 0 || size == 1 && raw[index] >= utf8.RuneSelf {
				return false
			}
			index += size
			continue
		}
		if index+1 >= len(raw)-1 {
			return false
		}
		if raw[index+1] != 'u' {
			index += 2
			continue
		}
		value, ok := parseHexScalar(raw, index+2)
		if !ok {
			return false
		}
		switch {
		case value >= 0xd800 && value <= 0xdbff:
			if index+11 >= len(raw)-1 || raw[index+6] != '\\' || raw[index+7] != 'u' {
				return false
			}
			low, ok := parseHexScalar(raw, index+8)
			if !ok || low < 0xdc00 || low > 0xdfff {
				return false
			}
			index += 12
		case value >= 0xdc00 && value <= 0xdfff:
			return false
		default:
			index += 6
		}
	}
	return true
}

func parseHexScalar(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for _, digit := range raw[start : start+4] {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func validateUniqueJSONKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := validateUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("additional JSON value")
	}
	return nil
}

func validateUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is invalid")
			}
			if _, duplicate := keys[key]; duplicate {
				return errors.New("object key is duplicated")
			}
			keys[key] = struct{}{}
			if err := validateUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := validateUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("JSON delimiter is invalid")
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if closing != json.Delim(map[json.Delim]json.Delim{'{': '}', '[': ']'}[delimiter]) {
		return errors.New("JSON delimiter is invalid")
	}
	return nil
}

func nullableStringPresent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	if bytes.Equal(raw, []byte("null")) {
		return true
	}
	var value string
	return json.Unmarshal(raw, &value) == nil
}

func sourceAgentID(sourceID, profileID string) string {
	identity := "hermes-source-agent:v1\n" +
		strconv.Itoa(len([]byte(sourceID))) + ":" + sourceID +
		strconv.Itoa(len([]byte(profileID))) + ":" + profileID
	digest := sha256.Sum256([]byte(identity))
	return "source-agent:hermes:v1:" + hex.EncodeToString(digest[:])
}

func displayName(candidate, fallback string) string {
	if !normalizedNonblank(candidate) || utf8.RuneCountInString(candidate) > 64 || strings.IndexFunc(candidate, unicode.IsControl) >= 0 {
		return fallback
	}
	return candidate
}

func normalizedNonblank(value string) bool {
	return value != "" && value == strings.TrimSpace(value)
}
