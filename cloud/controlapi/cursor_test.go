package controlapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestCursorHandlerReturnsBoundedOrderedPageForSignedAccount(t *testing.T) {
	t.Parallel()

	body := `{"after_cursor":"cursor-12"}`
	now := time.Unix(1_787_331_600, 0).UTC()
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.events.read")
	reader := &cursorReaderFake{page: controlapi.CursorPage{
		Events: []controlapi.CursorEvent{
			{Cursor: "cursor-13", Kind: "message.created", Data: map[string]string{"message_id": "message:13"}},
			{Cursor: "cursor-14", Kind: "target.finished", Data: map[string]string{"target_id": "target:14"}},
		},
		NextCursor: "cursor-14",
	}}
	handler := controlapi.RequireServiceAssertion(verifier, "owner.events.read", controlapi.CursorHandler(reader))
	request := httptest.NewRequest(http.MethodPost, "/api/v2/events/cursor", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	request.Header.Set("X-Fort-Account-ID", "forged")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if reader.accountID != "4af424a4-d81a-47d5-a495-400868883b86" || reader.afterCursor != "cursor-12" {
		t.Fatalf("reader input = account %q cursor %q", reader.accountID, reader.afterCursor)
	}
	var response struct {
		Events     []controlapi.CursorEvent `json:"events"`
		NextCursor string                   `json:"next_cursor"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Events) != 2 || response.Events[0].Cursor != "cursor-13" || response.NextCursor != "cursor-14" {
		t.Fatalf("cursor response = %+v", response)
	}
}

func TestCursorHandlerRejectsInvalidOrOversizePagesWithoutInliningThem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		page controlapi.CursorPage
	}{
		{
			name: "duplicate cursor",
			page: controlapi.CursorPage{Events: []controlapi.CursorEvent{
				{Cursor: "cursor-13", Kind: "one", Data: map[string]string{}},
				{Cursor: "cursor-13", Kind: "two", Data: map[string]string{}},
			}, NextCursor: "cursor-13"},
		},
		{
			name: "oversize",
			page: controlapi.CursorPage{Events: []controlapi.CursorEvent{
				{Cursor: "cursor-13", Kind: "message.created", Data: strings.Repeat("x", controlapi.MaximumCursorPageBytes)},
			}, NextCursor: "cursor-13"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reader := &cursorReaderFake{page: test.page}
			recorder := serveSignedCursor(t, reader, `{"after_cursor":"cursor-12"}`)

			if recorder.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d; body length=%d", recorder.Code, http.StatusBadGateway, recorder.Body.Len())
			}
			if recorder.Body.Len() > 256 || strings.Contains(recorder.Body.String(), strings.Repeat("x", 100)) {
				t.Fatalf("invalid page was inlined: body length=%d", recorder.Body.Len())
			}
		})
	}
}

func TestCursorHandlerReturnsEmptyArrayAndFailsClosedOnReaderError(t *testing.T) {
	t.Parallel()

	empty := serveSignedCursor(t, &cursorReaderFake{page: controlapi.CursorPage{NextCursor: "cursor-12"}}, `{"after_cursor":"cursor-12"}`)
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"events":[]`) {
		t.Fatalf("empty page = status %d body %q", empty.Code, empty.Body.String())
	}

	unavailable := serveSignedCursor(t, &cursorReaderFake{err: errors.New("database unavailable")}, `{"after_cursor":"cursor-12"}`)
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("reader error status = %d, want %d", unavailable.Code, http.StatusServiceUnavailable)
	}
}

func serveSignedCursor(t *testing.T, reader controlapi.CursorReader, body string) *httptest.ResponseRecorder {
	t.Helper()
	now := time.Unix(1_787_331_600, 0).UTC()
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.events.read")
	handler := controlapi.RequireServiceAssertion(verifier, "owner.events.read", controlapi.CursorHandler(reader))
	request := httptest.NewRequest(http.MethodPost, "/api/v2/events/cursor", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

type cursorReaderFake struct {
	page        controlapi.CursorPage
	err         error
	accountID   string
	afterCursor string
}

func (reader *cursorReaderFake) ReadCursorPage(_ context.Context, accountID, afterCursor string) (controlapi.CursorPage, error) {
	reader.accountID = accountID
	reader.afterCursor = afterCursor
	return reader.page, reader.err
}
