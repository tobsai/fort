package capability

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
)

const (
	maxCodexAppServerMessageBytes = 1 << 20
	maxCodexAppServerMessages     = 512
	maxCodexModelPages            = 128
)

// CodexAppServerProcess is the private JSONL process boundary used by the
// no-turn inspector. Tests substitute an in-memory transcript; production
// starts the exact executable identity held by CommandResolver.
type CodexAppServerProcess interface {
	io.Reader
	io.Writer
	io.Closer
	ExecutableDigest() string
}

type CodexAppServerStarter interface {
	Start(context.Context) (CodexAppServerProcess, error)
}

// CodexContractVerifier proves the held Codex executable's no-turn app-server
// schema contract. Production verification is intentionally lazy: wiring and
// read-only CLI commands must never run provider commands.
type CodexContractVerifier interface {
	Verify(context.Context) (CodexAppServerContract, error)
}

// ResolverCodexAppServerStarter launches the same immutable staged Codex bytes
// and environment policy used by the ordinary command probes.
type ResolverCodexAppServerStarter struct {
	Resolver *CommandResolver
}

func (s ResolverCodexAppServerStarter) Start(ctx context.Context) (CodexAppServerProcess, error) {
	if s.Resolver == nil {
		return nil, ErrCommandAbsent
	}
	held, err := s.Resolver.Hold("codex")
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, held.Executable(), "app-server", "--stdio")
	command.Env = held.Environment()
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	command.Stderr = &boundedOutput{maximum: 64 << 10}
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	return &execCodexAppServerProcess{
		command: command, stdin: stdin, stdout: stdout, executableDigest: held.Digest(),
	}, nil
}

type execCodexAppServerProcess struct {
	command          *exec.Cmd
	stdin            io.WriteCloser
	stdout           io.ReadCloser
	executableDigest string
	once             sync.Once
}

func (p *execCodexAppServerProcess) ExecutableDigest() string { return p.executableDigest }

func (p *execCodexAppServerProcess) Read(value []byte) (int, error) {
	return p.stdout.Read(value)
}

func (p *execCodexAppServerProcess) Write(value []byte) (int, error) {
	return p.stdin.Write(value)
}

func (p *execCodexAppServerProcess) Close() error {
	p.once.Do(func() {
		_ = p.stdin.Close()
		if p.command.Process != nil {
			_ = p.command.Process.Kill()
		}
		_ = p.stdout.Close()
		_ = p.command.Wait()
	})
	return nil
}

// CodexAppServerContract carries facts verified outside the live JSONL
// session. The inspector copies only these normalized schema/isolation facts;
// account and model readiness always come from the current app-server session.
type CodexAppServerContract struct {
	ExecutableDigest         string
	NormalSchemaDigest       string
	NormalSchemaFiles        int
	ExperimentalSchemaDigest string
	ExperimentalSchemaFiles  int
	GmailIsolationReady      bool
	SupabaseIsolationReady   bool
}

type CodexAppServerInspector struct {
	starter  CodexAppServerStarter
	contract CodexAppServerContract
	verifier CodexContractVerifier

	contractMu       sync.Mutex
	contractVerified bool
	contractFailure  error
	contractRetryAt  time.Time
}

func NewCodexAppServerInspector(starter CodexAppServerStarter, contract CodexAppServerContract) *CodexAppServerInspector {
	return &CodexAppServerInspector{starter: starter, contract: contract, contractVerified: true}
}

// NewVerifiedCodexAppServerInspector defers schema generation until a live
// capability probe needs Codex. A successful proof is immutable for this
// process; a failed proof is briefly cached so one inventory refresh cannot
// fan out into repeated expensive schema generation attempts.
func NewVerifiedCodexAppServerInspector(starter CodexAppServerStarter, verifier CodexContractVerifier) *CodexAppServerInspector {
	return &CodexAppServerInspector{starter: starter, verifier: verifier}
}

func (i *CodexAppServerInspector) Inspect(ctx context.Context) (CodexInspection, error) {
	if err := ctx.Err(); err != nil {
		return CodexInspection{}, err
	}
	if i == nil || i.starter == nil {
		return CodexInspection{}, codexProtocolError()
	}
	contract, err := i.verifiedContract(ctx)
	if err != nil {
		return CodexInspection{}, err
	}
	process, err := i.starter.Start(ctx)
	if err != nil {
		return CodexInspection{}, normalizeCodexInspectorError(ctx, err)
	}
	defer process.Close()
	if contract.ExecutableDigest == "" || process.ExecutableDigest() == "" ||
		process.ExecutableDigest() != contract.ExecutableDigest {
		return CodexInspection{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}

	session := newCodexAppServerSession(process)
	initializeResult, err := session.call(ctx, 1, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "fort", "version": "1"},
		"capabilities": map[string]any{"experimentalApi": true},
	})
	if err != nil {
		return CodexInspection{}, err
	}
	if err := validateCodexInitialize(initializeResult); err != nil {
		return CodexInspection{}, err
	}
	if err := session.notify(ctx, "initialized"); err != nil {
		return CodexInspection{}, err
	}

	accountResult, err := session.call(ctx, 2, "account/read", map[string]any{"refreshToken": false})
	if err != nil {
		return CodexInspection{}, err
	}
	accountReady, accountHandle, err := parseCodexAccount(accountResult)
	if err != nil {
		return CodexInspection{}, err
	}

	configResult, err := session.call(ctx, 3, "config/read", map[string]any{"includeLayers": false})
	if err != nil {
		return CodexInspection{}, err
	}
	configuredModel, err := parseCodexConfiguredModel(configResult)
	if err != nil {
		return CodexInspection{}, err
	}

	models, catalogDefault, err := session.readModels(ctx, 4)
	if err != nil {
		return CodexInspection{}, err
	}
	defaultModel := configuredModel
	if defaultModel == "" {
		defaultModel = catalogDefault
	}
	return CodexInspection{
		AccountReady: accountReady, AccountHandle: accountHandle,
		Models: models, DefaultModel: defaultModel,
		ExecutableDigest:         process.ExecutableDigest(),
		NormalSchemaDigest:       contract.NormalSchemaDigest,
		NormalSchemaFiles:        contract.NormalSchemaFiles,
		ExperimentalSchemaDigest: contract.ExperimentalSchemaDigest,
		ExperimentalSchemaFiles:  contract.ExperimentalSchemaFiles,
		GmailIsolationReady:      contract.GmailIsolationReady,
		SupabaseIsolationReady:   contract.SupabaseIsolationReady,
	}, nil
}

// parseCodexConfiguredModel reads only the effective model selector from the
// app-server's typed config response. model/list's isDefault marks the catalog
// default; it does not reflect a user's config.toml override, so using it alone
// can falsely certify an unavailable configured-default profile.
func parseCodexConfiguredModel(result json.RawMessage) (string, error) {
	var payload struct {
		Config  json.RawMessage `json:"config"`
		Origins json.RawMessage `json:"origins"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || len(payload.Config) == 0 || len(payload.Origins) == 0 {
		return "", codexProtocolError()
	}
	var config *struct {
		Model *string `json:"model"`
	}
	var origins map[string]json.RawMessage
	if err := json.Unmarshal(payload.Config, &config); err != nil || config == nil {
		return "", codexProtocolError()
	}
	if err := json.Unmarshal(payload.Origins, &origins); err != nil || origins == nil {
		return "", codexProtocolError()
	}
	if config.Model == nil {
		return "", nil
	}
	model := *config.Model
	if model == "" || model != strings.TrimSpace(model) {
		return "", codexProtocolError()
	}
	return model, nil
}

const codexContractFailureRetry = 30 * time.Second

func (i *CodexAppServerInspector) verifiedContract(ctx context.Context) (CodexAppServerContract, error) {
	i.contractMu.Lock()
	defer i.contractMu.Unlock()
	if i.contractVerified {
		return i.contract, nil
	}
	if i.verifier == nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}
	now := time.Now()
	if i.contractFailure != nil && now.Before(i.contractRetryAt) {
		return CodexAppServerContract{}, i.contractFailure
	}
	contract, err := i.verifier.Verify(ctx)
	if err != nil {
		err = normalizeCodexContractError(ctx, err)
		if ctx.Err() == nil {
			i.contractFailure = err
			i.contractRetryAt = now.Add(codexContractFailureRetry)
		}
		return CodexAppServerContract{}, err
	}
	i.contract = contract
	i.contractVerified = true
	i.contractFailure = nil
	return contract, nil
}

func normalizeCodexContractError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var probeError *ProbeError
	if errors.As(err, &probeError) && corecap.FirstReason(probeError.Reason) != "" {
		return &ProbeError{Reason: probeError.Reason}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &ProbeError{Reason: corecap.ReasonProbeTimedOut}
	}
	return &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
}

type codexAppServerSession struct {
	process  CodexAppServerProcess
	scanner  *bufio.Scanner
	encoder  *json.Encoder
	messages int
}

func newCodexAppServerSession(process CodexAppServerProcess) *codexAppServerSession {
	scanner := bufio.NewScanner(process)
	scanner.Buffer(make([]byte, 64<<10), maxCodexAppServerMessageBytes)
	return &codexAppServerSession{
		process: process, scanner: scanner, encoder: json.NewEncoder(process),
	}
}

func (s *codexAppServerSession) notify(ctx context.Context, method string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.encoder.Encode(map[string]any{"method": method}); err != nil {
		return normalizeCodexInspectorError(ctx, err)
	}
	return nil
}

func (s *codexAppServerSession) call(ctx context.Context, id int, method string, params map[string]any) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return nil, normalizeCodexInspectorError(ctx, err)
	}
	for s.scanner.Scan() {
		s.messages++
		if s.messages > maxCodexAppServerMessages {
			return nil, codexProtocolError()
		}
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(s.scanner.Bytes(), &envelope); err != nil {
			return nil, codexProtocolError()
		}
		if len(envelope.ID) == 0 {
			if envelope.Method == "" {
				return nil, codexProtocolError()
			}
			continue
		}
		var responseID int
		if err := json.Unmarshal(envelope.ID, &responseID); err != nil || responseID != id {
			return nil, codexProtocolError()
		}
		if len(envelope.Error) != 0 && !bytes.Equal(bytes.TrimSpace(envelope.Error), []byte("null")) {
			return nil, codexProtocolError()
		}
		if len(envelope.Result) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Result), []byte("null")) {
			return nil, codexProtocolError()
		}
		return append(json.RawMessage(nil), envelope.Result...), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.scanner.Err(); err != nil {
		return nil, normalizeCodexInspectorError(ctx, err)
	}
	return nil, codexProtocolError()
}

func (s *codexAppServerSession) readModels(ctx context.Context, firstID int) (map[string]bool, string, error) {
	models := make(map[string]bool)
	seenCursors := make(map[string]bool)
	defaultModel := ""
	defaultRows := 0
	cursor := ""
	for page := 0; page < maxCodexModelPages; page++ {
		params := map[string]any{"includeHidden": true}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := s.call(ctx, firstID+page, "model/list", params)
		if err != nil {
			return nil, "", err
		}
		var payload struct {
			Data       json.RawMessage `json:"data"`
			NextCursor json.RawMessage `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &payload); err != nil || len(payload.Data) == 0 {
			return nil, "", codexProtocolError()
		}
		var rows []struct {
			Model     string `json:"model"`
			IsDefault *bool  `json:"isDefault"`
		}
		if err := json.Unmarshal(payload.Data, &rows); err != nil || rows == nil {
			return nil, "", codexProtocolError()
		}
		for _, row := range rows {
			if row.Model == "" || row.Model != strings.TrimSpace(row.Model) || row.IsDefault == nil {
				return nil, "", codexProtocolError()
			}
			models[row.Model] = true
			if *row.IsDefault {
				defaultRows++
				defaultModel = row.Model
			}
		}

		next, err := parseCodexCursor(payload.NextCursor)
		if err != nil {
			return nil, "", err
		}
		if next == "" {
			if defaultRows != 1 {
				defaultModel = ""
			}
			return models, defaultModel, nil
		}
		if seenCursors[next] {
			return nil, "", codexProtocolError()
		}
		seenCursors[next] = true
		cursor = next
	}
	return nil, "", codexProtocolError()
}

func parseCodexAccount(result json.RawMessage) (bool, string, error) {
	var payload struct {
		Account            json.RawMessage `json:"account"`
		RequiresOpenAIAuth *bool           `json:"requiresOpenaiAuth"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || payload.RequiresOpenAIAuth == nil {
		return false, "", codexProtocolError()
	}
	accountPresent := len(payload.Account) != 0 && !bytes.Equal(bytes.TrimSpace(payload.Account), []byte("null"))
	if accountPresent {
		var account struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload.Account, &account); err != nil || account.Type == "" {
			return false, "", codexProtocolError()
		}
		switch account.Type {
		case "apiKey", "chatgpt", "amazonBedrock":
		default:
			return false, "", codexProtocolError()
		}
	}
	if *payload.RequiresOpenAIAuth && !accountPresent {
		return false, "", nil
	}
	if accountPresent {
		return true, "authenticated", nil
	}
	return true, "not-required", nil
}

func validateCodexInitialize(result json.RawMessage) error {
	var payload struct {
		CodexHome      string `json:"codexHome"`
		PlatformFamily string `json:"platformFamily"`
		PlatformOS     string `json:"platformOs"`
		UserAgent      string `json:"userAgent"`
	}
	if err := json.Unmarshal(result, &payload); err != nil ||
		!filepath.IsAbs(payload.CodexHome) || payload.PlatformFamily != "unix" ||
		payload.PlatformOS != "macos" || payload.UserAgent == "" {
		return codexProtocolError()
	}
	return nil
}

func parseCodexCursor(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var cursor string
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor == "" {
		return "", codexProtocolError()
	}
	return cursor, nil
}

func normalizeCodexInspectorError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return codexProtocolError()
}

func codexProtocolError() error {
	return &ProbeError{Reason: corecap.ReasonCommandContractChanged}
}
