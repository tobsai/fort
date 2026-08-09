package codexsubscription

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	coreruntime "github.com/tobsai/fort/core/runtime"
)

const (
	maxPromptBytes    = 65_536
	maxJSONLLineBytes = 256 << 10
	maxStdoutBytes    = 1 << 20
	maxStderrBytes    = 1 << 20
	maxJSONLEvents    = 512
	maxMessageBytes   = 256 << 10
)

type streamFailure struct {
	code string
}

func (f *streamFailure) Error() string { return f.code }

type parseResult struct {
	threadID string
	message  string
	usage    coreruntime.ProviderUsage
	err      error
}

type jsonlEvent struct {
	Type     string          `json:"type"`
	ThreadID string          `json:"thread_id"`
	Item     json.RawMessage `json:"item"`
	Usage    *jsonlUsage     `json:"usage"`
}

type jsonlItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Text    string `json:"text"`
	Message string `json:"message"`
}

type jsonlUsage struct {
	InputTokens           *int64 `json:"input_tokens"`
	CachedInputTokens     *int64 `json:"cached_input_tokens"`
	OutputTokens          *int64 `json:"output_tokens"`
	ReasoningTokens       *int64 `json:"reasoning_tokens"`
	ReasoningOutputTokens *int64 `json:"reasoning_output_tokens"`
}

const codeModeFailClosedDiagnostic = "Code Mode is unavailable because code-mode host is disabled. Code mode will fail closed; enable `features.code_mode_host` and install `codex-code-mode-host`."

func parseJSONL(source io.Reader) parseResult {
	if source == nil {
		return parseResult{err: providerFailure()}
	}
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 64<<10), maxJSONLLineBytes)
	var result parseResult
	var total, events int
	var threadStarted, codeModeDenied, turnStarted, turnCompleted bool
	var messages int
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		total += len(line) + 1
		events++
		if len(bytes.TrimSpace(line)) == 0 || total > maxStdoutBytes || events > maxJSONLEvents {
			return parseResult{err: providerFailure()}
		}
		var event jsonlEvent
		if err := json.Unmarshal(line, &event); err != nil || event.Type == "" {
			return parseResult{err: providerFailure()}
		}
		if turnCompleted {
			return parseResult{err: providerFailure()}
		}
		switch event.Type {
		case "thread.started":
			if threadStarted || turnStarted || strings.TrimSpace(event.ThreadID) == "" {
				return parseResult{err: providerFailure()}
			}
			threadStarted = true
			result.threadID = event.ThreadID
		case "turn.started":
			if !threadStarted || turnStarted {
				return parseResult{err: providerFailure()}
			}
			turnStarted = true
		case "item.started", "item.completed":
			if !threadStarted || len(event.Item) == 0 {
				return parseResult{err: providerFailure()}
			}
			var item jsonlItem
			if err := json.Unmarshal(event.Item, &item); err != nil || strings.TrimSpace(item.ID) == "" || item.Type == "" {
				return parseResult{err: providerFailure()}
			}
			if !turnStarted {
				if event.Type != "item.completed" || codeModeDenied || item.Type != "error" ||
					item.Text != "" || item.Message != codeModeFailClosedDiagnostic {
					return parseResult{err: providerFailure()}
				}
				codeModeDenied = true
				continue
			}
			if activeItem(item.Type) {
				return parseResult{err: authorityFailure()}
			}
			switch item.Type {
			case "reasoning":
				// Inert reasoning is intentionally discarded.
			case "agent_message":
				if event.Type == "item.completed" {
					messages++
					if messages != 1 || strings.TrimSpace(item.Text) == "" || len(item.Text) > maxMessageBytes {
						return parseResult{err: providerFailure()}
					}
					result.message = item.Text
				}
			case "error":
				return parseResult{err: providerFailure()}
			default:
				return parseResult{err: authorityFailure()}
			}
		case "turn.completed":
			if !turnStarted || turnCompleted || messages != 1 || event.Usage == nil {
				return parseResult{err: providerFailure()}
			}
			usage, err := normalizeUsage(event.Usage)
			if err != nil {
				return parseResult{err: providerFailure()}
			}
			result.usage = usage
			turnCompleted = true
		case "turn.failed", "turn.cancelled", "error":
			return parseResult{err: providerFailure()}
		default:
			return parseResult{err: providerFailure()}
		}
	}
	if err := scanner.Err(); err != nil {
		return parseResult{err: providerFailure()}
	}
	if !threadStarted || !codeModeDenied || !turnStarted || !turnCompleted || messages != 1 {
		return parseResult{err: providerFailure()}
	}
	return result
}

func activeItem(itemType string) bool {
	switch itemType {
	case "command_execution", "file_change", "mcp_tool_call", "collab_tool_call",
		"web_search", "todo_list", "dynamic_tool_call":
		return true
	default:
		return strings.Contains(itemType, "tool_call") || strings.Contains(itemType, "command") ||
			strings.Contains(itemType, "file_read")
	}
}

func normalizeUsage(value *jsonlUsage) (coreruntime.ProviderUsage, error) {
	if value == nil || value.InputTokens == nil || value.CachedInputTokens == nil || value.OutputTokens == nil {
		return coreruntime.ProviderUsage{}, fmt.Errorf("missing usage")
	}
	reasoning := int64(0)
	if value.ReasoningTokens != nil && value.ReasoningOutputTokens != nil && *value.ReasoningTokens != *value.ReasoningOutputTokens {
		return coreruntime.ProviderUsage{}, fmt.Errorf("conflicting reasoning usage")
	}
	if value.ReasoningOutputTokens != nil {
		reasoning = *value.ReasoningOutputTokens
	} else if value.ReasoningTokens != nil {
		reasoning = *value.ReasoningTokens
	}
	result := coreruntime.ProviderUsage{
		InputTokens: *value.InputTokens, CachedInputTokens: *value.CachedInputTokens,
		OutputTokens: *value.OutputTokens, ReasoningTokens: reasoning,
	}
	if result.InputTokens < 0 || result.CachedInputTokens < 0 || result.OutputTokens <= 0 || result.ReasoningTokens < 0 ||
		result.CachedInputTokens > result.InputTokens || result.ReasoningTokens > result.OutputTokens {
		return coreruntime.ProviderUsage{}, fmt.Errorf("negative usage")
	}
	return result, nil
}

func drainBounded(source io.Reader, maximum int64) error {
	if source == nil {
		return nil
	}
	written, err := io.Copy(io.Discard, io.LimitReader(source, maximum+1))
	if err != nil || written > maximum {
		return providerFailure()
	}
	return nil
}

func providerFailure() error  { return &streamFailure{code: coreruntime.ErrorProviderFailed} }
func authorityFailure() error { return &streamFailure{code: coreruntime.ErrorChatAuthorityViolation} }
