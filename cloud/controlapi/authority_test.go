package controlapi_test

import (
	"testing"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestCloudWriteAuthorityRequiresExactModeAndPositiveEpoch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		mode  string
		epoch string
		want  bool
	}{
		{name: "active", mode: "cloud_v2_write", epoch: "8", want: true},
		{name: "legacy", mode: "legacy_v1_write", epoch: "7"},
		{name: "missing mode", epoch: "8"},
		{name: "future mode", mode: "cloud_v3_write", epoch: "8"},
		{name: "missing epoch", mode: "cloud_v2_write"},
		{name: "zero epoch", mode: "cloud_v2_write", epoch: "0"},
		{name: "negative epoch", mode: "cloud_v2_write", epoch: "-1"},
		{name: "noncanonical epoch", mode: "cloud_v2_write", epoch: "08"},
		{name: "whitespace", mode: " cloud_v2_write", epoch: "8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := map[string]string{
				"FORT_AUTHORITY_MODE":  test.mode,
				"FORT_AUTHORITY_EPOCH": test.epoch,
			}
			if got := controlapi.CloudWriteAuthorityActive(func(key string) string { return values[key] }); got != test.want {
				t.Fatalf("CloudWriteAuthorityActive() = %t, want %t", got, test.want)
			}
		})
	}
}
