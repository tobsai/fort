package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/tobsai/fort/cloud/controlapi"
)

// Handler is the bounded Vercel Go Function entrypoint for /api/health.
func Handler(response http.ResponseWriter, request *http.Request) {
	healthHandler(os.Getenv).ServeHTTP(response, request)
}

func healthHandler(getenv func(string) string) http.Handler {
	apiMinVersion, _ := strconv.Atoi(getenv("FORT_API_MIN_VERSION"))
	apiMaxVersion, _ := strconv.Atoi(getenv("FORT_API_MAX_VERSION"))
	authorityEpoch, _ := strconv.ParseInt(getenv("FORT_AUTHORITY_EPOCH"), 10, 64)
	commit := getenv("VERCEL_GIT_COMMIT_SHA")
	if commit == "" {
		commit = getenv("FORT_BUILD_COMMIT")
	}

	return controlapi.HealthHandler(controlapi.BuildInfo{
		Commit:         commit,
		SchemaVersion:  getenv("FORT_SCHEMA_VERSION"),
		APIMinVersion:  apiMinVersion,
		APIMaxVersion:  apiMaxVersion,
		AuthorityEpoch: authorityEpoch,
		AuthorityMode:  getenv("FORT_AUTHORITY_MODE"),
	})
}
