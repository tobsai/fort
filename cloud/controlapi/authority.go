package controlapi

import (
	"strconv"
	"strings"
)

const CloudWriteAuthorityMode = "cloud_v2_write"

// CloudWriteAuthorityActive is the fail-closed deployment fence shared by
// owner commands and execution workers. Reads remain available in rollback
// mode, but no v2 mutation may reach Postgres until one exact positive epoch
// designates the cloud control plane as the sole write authority.
func CloudWriteAuthorityActive(getenv func(string) string) bool {
	if getenv == nil || getenv("FORT_AUTHORITY_MODE") != CloudWriteAuthorityMode {
		return false
	}
	epoch := getenv("FORT_AUTHORITY_EPOCH")
	if epoch == "" || strings.Trim(epoch, "0123456789") != "" || epoch[0] == '0' {
		return false
	}
	value, err := strconv.ParseUint(epoch, 10, 63)
	return err == nil && value > 0
}
