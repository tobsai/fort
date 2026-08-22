// Package controlapi contains stateless HTTP handlers for Fort's cloud control
// plane. Durable state and execution are supplied through explicit ports; a
// handler must not start a scheduler, runtime, or permanent listener.
package controlapi

import (
	"encoding/json"
	"net/http"
	"regexp"
)

var (
	commitPattern        = regexp.MustCompile(`^[0-9a-f]{40}$`)
	schemaVersionPattern = regexp.MustCompile(`^[0-9]{14}$`)
)

// BuildInfo identifies the exact compatible deployment pair. AuthorityMode is
// intentionally explicit so clients never infer the write owner from reachability.
type BuildInfo struct {
	Commit         string
	SchemaVersion  string
	APIMinVersion  int
	APIMaxVersion  int
	AuthorityEpoch int64
	AuthorityMode  string
}

// HealthHandler reports only public deployment compatibility metadata.
func HealthHandler(info BuildInfo) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		if !info.valid() {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "deployment_metadata_unavailable"})
			return
		}

		writeJSON(response, http.StatusOK, struct {
			Status         string `json:"status"`
			Commit         string `json:"commit"`
			SchemaVersion  string `json:"schema_version"`
			APIMinVersion  int    `json:"api_min_version"`
			APIMaxVersion  int    `json:"api_max_version"`
			AuthorityEpoch int64  `json:"authority_epoch"`
			AuthorityMode  string `json:"authority_mode"`
		}{
			Status:         "ok",
			Commit:         info.Commit,
			SchemaVersion:  info.SchemaVersion,
			APIMinVersion:  info.APIMinVersion,
			APIMaxVersion:  info.APIMaxVersion,
			AuthorityEpoch: info.AuthorityEpoch,
			AuthorityMode:  info.AuthorityMode,
		})
	})
}

func (info BuildInfo) valid() bool {
	if !commitPattern.MatchString(info.Commit) || !schemaVersionPattern.MatchString(info.SchemaVersion) {
		return false
	}
	if info.APIMinVersion < 1 || info.APIMaxVersion < info.APIMinVersion || info.AuthorityEpoch < 1 {
		return false
	}
	return info.AuthorityMode == "legacy_v1_write" || info.AuthorityMode == "cloud_v2_write"
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
