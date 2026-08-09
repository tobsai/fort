package codexsubscription

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestExportedContractRevisionsMatchCanonicalBytes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		got   string
	}{
		{name: "developer instruction", input: DeveloperInstruction, got: DeveloperInstructionRevision},
		{name: "isolation", input: isolationContract, got: IsolationRevision},
		{name: "adapter", input: adapterContract, got: AdapterRevision},
		{name: "policy", input: policyContract, got: PolicyRevision},
		{name: "schema", input: schemaContract, got: CodexSchemaRevision},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			digest := sha256.Sum256([]byte(test.input))
			if want := hex.EncodeToString(digest[:]); test.got != want {
				t.Fatalf("revision = %q, want %q", test.got, want)
			}
		})
	}
}

func TestExportedContractProvidesExactRuntimeInvocation(t *testing.T) {
	args := ExecArguments("prompt", "gpt-5.6-sol", "/private/empty")
	want := []string{
		"exec", "prompt", "--json", "--sandbox", "read-only",
		"--skip-git-repo-check", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--strict-config",
		"-C", "/private/empty", "--model", "gpt-5.6-sol",
		"-c", `approval_policy="never"`,
		"-c", `developer_instructions="` + DeveloperInstruction + `"`,
		"-c", `model_reasoning_effort="medium"`,
		"-c", `web_search="disabled"`,
		"-c", "tools.update_plan.enabled=false",
		"-c", "tools.experimental_request_user_input.enabled=false",
		"-c", "skills.include_instructions=false",
		"-c", "skills.bundled.enabled=false",
		"-c", "include_apps_instructions=false",
		"-c", "include_collaboration_mode_instructions=false",
		"-c", "include_environment_context=false",
		"-c", "include_permissions_instructions=false",
		"--disable", "shell_tool", "--disable", "unified_exec", "--disable", "apps",
		"--disable", "browser_use", "--disable", "computer_use", "--disable", "image_generation",
		"--disable", "in_app_browser", "--disable", "multi_agent", "--disable", "memories",
		"--disable", "plugins", "--disable", "skill_search", "--disable", "workspace_dependencies",
		"--disable", "code_mode", "--disable", "code_mode_host",
		"--disable", "code_mode_only", "--disable", "code_mode_buffered_exec",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("arguments = %#v", args)
	}
	if got, want := DisabledFeatures(), []string{
		"shell_tool", "unified_exec", "apps", "browser_use", "computer_use", "image_generation",
		"in_app_browser", "multi_agent", "memories", "plugins", "skill_search", "workspace_dependencies",
		"code_mode", "code_mode_host", "code_mode_only", "code_mode_buffered_exec",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("disabled features = %#v", got)
	}
}

func TestCurrentCodexContractPinsApprovedInstalledEvidence(t *testing.T) {
	if CodexVersion != "codex-cli 0.147.0-alpha.6.5" ||
		CodexExecutableRevision != "e4432c0c085e4a2e5b9cf982e4dd2ebdb44ed33c422827b6e6c64353778e773b" ||
		CodexNormalSchemaRevision != "be9de4ce887d8338794dbd330202fb5bdbd43316cc73b721efacf4542c8607b9" ||
		CodexNormalSchemaFiles != 285 ||
		CodexExperimentalSchemaRevision != "2b6fb0da3b974ed25389d021f274900837c3ae5d406ba85be8cde45a21d0214f" ||
		CodexExperimentalSchemaFiles != 361 {
		t.Fatal("Codex contract pins changed without an explicit catalog revision")
	}
}
