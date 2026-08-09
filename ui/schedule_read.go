package ui

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
)

const defaultScheduleOccurrenceLimit = 50

func (s *Server) handleScheduleList(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduleRead(w) {
		return
	}
	query, ok := scheduleQuery(w, r, "state")
	if !ok {
		return
	}
	filter := ScheduleFilterAll
	if values, present := query["state"]; present {
		switch values[0] {
		case string(ScheduleFilterAll):
			filter = ScheduleFilterAll
		case string(ScheduleFilterActive):
			filter = ScheduleFilterActive
		case string(ScheduleFilterPaused):
			filter = ScheduleFilterPaused
		default:
			scheduleBadRequest(w, "state must be active, paused, or all")
			return
		}
	}
	list, err := s.d.ScheduleRead.List(r.Context(), filter)
	if err != nil {
		scheduleReadError(w, err)
		return
	}
	if list.Items == nil {
		list.Items = []ScheduleItem{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleScheduleGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduleRead(w) {
		return
	}
	if _, ok := scheduleQuery(w, r); !ok {
		return
	}
	detail, err := s.d.ScheduleRead.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		scheduleReadError(w, err)
		return
	}
	if detail.Upcoming == nil {
		detail.Upcoming = []scheduler.Occurrence{}
	}
	if detail.Recent == nil {
		detail.Recent = []scheduler.Occurrence{}
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleScheduleOccurrences(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduleRead(w) {
		return
	}
	query, ok := scheduleQuery(w, r, "limit", "before", "before_id")
	if !ok {
		return
	}
	page := OccurrencePage{Limit: defaultScheduleOccurrenceLimit}
	if values, present := query["limit"]; present {
		limit, err := strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > defaultScheduleOccurrenceLimit {
			scheduleBadRequest(w, "limit must be between 1 and 50")
			return
		}
		page.Limit = limit
	}
	beforeValues, hasBefore := query["before"]
	idValues, hasBeforeID := query["before_id"]
	if hasBefore != hasBeforeID {
		scheduleBadRequest(w, "before and before_id must be supplied together")
		return
	}
	if hasBefore {
		if beforeValues[0] == "" || idValues[0] == "" {
			scheduleBadRequest(w, "before and before_id must be non-empty")
			return
		}
		before, err := time.Parse(time.RFC3339, beforeValues[0])
		if err != nil {
			scheduleBadRequest(w, "before must be RFC3339")
			return
		}
		page.Before, page.BeforeID = before, idValues[0]
	}
	items, err := s.d.ScheduleRead.Occurrences(r.Context(), r.PathValue("id"), page)
	if err != nil {
		scheduleReadError(w, err)
		return
	}
	if items == nil {
		items = []scheduler.Occurrence{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) requireScheduleRead(w http.ResponseWriter) bool {
	if s.d.ScheduleRead == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "schedule read is unavailable"})
		return false
	}
	return true
}

func scheduleQuery(w http.ResponseWriter, r *http.Request, allowed ...string) (url.Values, bool) {
	allowedKeys := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = true
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		scheduleBadRequest(w, "invalid schedule query")
		return nil, false
	}
	for key, values := range query {
		if !allowedKeys[key] || len(values) != 1 || values[0] == "" {
			scheduleBadRequest(w, "invalid schedule query")
			return nil, false
		}
	}
	return query, true
}

func scheduleBadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})
}

func scheduleReadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, store.ErrScheduleCatalogLimit):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "schedule_catalog_limit", "error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "schedule read failed"})
	}
}
