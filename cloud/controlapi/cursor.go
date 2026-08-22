package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const MaximumCursorPageBytes = 1 << 20

type CursorEvent struct {
	Cursor string `json:"cursor"`
	Kind   string `json:"kind"`
	Data   any    `json:"data"`
}

type CursorPage struct {
	Events     []CursorEvent `json:"events"`
	NextCursor string        `json:"next_cursor"`
}

// CursorReader returns durable events after one exact cursor. Implementations
// may long-poll, but must return before the bounded request context expires.
type CursorReader interface {
	ReadCursorPage(context.Context, string, string) (CursorPage, error)
}

// CursorHandler serves the bounded JSON half of the Node SSE reconnect loop.
func CursorHandler(reader CursorReader) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		if !ok {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "service_assertion_required"})
			return
		}
		var input struct {
			AfterCursor string `json:"after_cursor"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || !decoderAtEOF(decoder) || !validEventCursor(input.AfterCursor) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "cursor_request_invalid"})
			return
		}
		if reader == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "cursor_unavailable"})
			return
		}
		page, err := reader.ReadCursorPage(request.Context(), accountID, input.AfterCursor)
		if err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "cursor_unavailable"})
			return
		}
		if page.Events == nil {
			page.Events = []CursorEvent{}
		}
		if !validCursorPage(input.AfterCursor, page) {
			writeJSON(response, http.StatusBadGateway, map[string]string{"code": "cursor_page_invalid"})
			return
		}
		payload, err := json.Marshal(page)
		if err != nil {
			writeJSON(response, http.StatusBadGateway, map[string]string{"code": "cursor_page_invalid"})
			return
		}
		payload = append(payload, '\n')
		if len(payload) > MaximumCursorPageBytes {
			writeJSON(response, http.StatusBadGateway, map[string]string{"code": "cursor_page_limit"})
			return
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(payload)
	})
}

func validCursorPage(afterCursor string, page CursorPage) bool {
	if !validEventCursor(page.NextCursor) {
		return false
	}
	if len(page.Events) == 0 {
		return page.NextCursor == afterCursor
	}
	seen := make(map[string]struct{}, len(page.Events)+1)
	seen[afterCursor] = struct{}{}
	for _, event := range page.Events {
		if !validEventCursor(event.Cursor) || strings.TrimSpace(event.Kind) == "" || event.Kind != strings.TrimSpace(event.Kind) {
			return false
		}
		if _, exists := seen[event.Cursor]; exists {
			return false
		}
		seen[event.Cursor] = struct{}{}
	}
	return page.Events[len(page.Events)-1].Cursor == page.NextCursor
}

func validEventCursor(cursor string) bool {
	return len(cursor) > 0 && len(cursor) <= 1024 && !strings.ContainsAny(cursor, "\r\n\x00")
}

func decoderAtEOF(decoder *json.Decoder) bool {
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}
