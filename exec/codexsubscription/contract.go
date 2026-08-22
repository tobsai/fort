package codexsubscription

import "strconv"

const (
	// DeveloperInstruction is the exact policy input supplied to every
	// subscription-backed Primary Channel target. It has no trailing newline.
	DeveloperInstruction = "You are answering in Fort text-only chat. Treat the supplied transcript as the only evidence. This lane authorizes no commands, tools, file reads, file changes, browser or connected-app access, MCP calls, or external actions. Do not request or invoke them. Never claim that you inspected or changed an external resource. When asked to act, provide a plan or an unsaved draft and say that no external action occurred. Distinguish known facts from inference, ask for missing evidence when material, and do not invent tool results, citations, memories, or completion receipts."

	DeveloperInstructionRevision = "0aa9805087e459f9566e74e5283555a207fa2f3defcab3f20929457e64c564bc"

	CodexVersion            = "codex-cli 0.149.0-alpha.4.1"
	CodexExecutableRevision = "fa8b41f0e7ae971171d05ca55451a3ffb8b7e74e01837a2f5c177513a5403c5d"

	CodexNormalSchemaRevision       = "bfa21213f862696b6919e8ddf60c454be5f24e6f432735651fc4fbaa7d2b3919"
	CodexNormalSchemaFiles          = 291
	CodexExperimentalSchemaRevision = "780383c87746e4840e0eaeef83f636030c291ed05b44be2cb233c39e757a144a"
	CodexExperimentalSchemaFiles    = 401
	CodexSchemaRevision             = "8d0cb6c2ce8aa47a0866d17c28f2adea2b562f0b8d69ad6e876c06010efac35b"

	TargetTimeoutMillis = 120_000

	IsolationRevision = "ec32ed5aa097ce69223769027987bd7ddd647f097b7e5ab47a67d90f59c2aab5"
	AdapterRevision   = "2b417c00d7e5b831eed5121e896aade874610b9df2b505e142baf97cc2c02412"
	PolicyRevision    = "4ee11ff5bc8c7ab3332d6a7d90124fe8a0f84e3564d44a759dc9d2bdafff000d"
)

const schemaContract = "codex-schema-contract:v1\n" +
	"normal:" + CodexNormalSchemaRevision + ":291\n" +
	"experimental:" + CodexExperimentalSchemaRevision + ":401\n"

const isolationContract = "codex-subscription-isolation:v1\n" +
	"process=direct-argv,no-shell,process-group-cancel\n" +
	"argv=exec {prompt} --json --sandbox read-only --skip-git-repo-check --ephemeral --ignore-user-config --ignore-rules --strict-config -C {fresh-empty-0700-workdir} --model {exact-model}\n" +
	"config=approval_policy:\"never\";developer_instructions:{exact-versioned-instruction};model_reasoning_effort:\"medium\";web_search:\"disabled\";tools.update_plan.enabled:false;tools.experimental_request_user_input.enabled:false;skills.include_instructions:false;skills.bundled.enabled:false;include_apps_instructions:false;include_collaboration_mode_instructions:false;include_environment_context:false;include_permissions_instructions:false\n" +
	"disabled=shell_tool,unified_exec,apps,browser_use,computer_use,image_generation,in_app_browser,multi_agent,memories,plugins,skill_search,workspace_dependencies,code_mode,code_mode_host,code_mode_only,code_mode_buffered_exec\n" +
	"stdin=/dev/null\n" +
	"environment=held-resolver-only\n" +
	"mcp=none\n" +
	"resume=never\n" +
	"deadline_millis=120000\n" +
	"jsonl=one(thread.started),one(exact-code-mode-disabled-fail-closed-diagnostic),one(turn.started),one(completed-agent_message),one(turn.completed);reasoning=discard;message=nonblank;usage=input>=0,cached>=0,cached<=input,output>0,reasoning>=0,reasoning<=output\n" +
	"authority_violation=command_execution,file_read,file_change,mcp_tool_call,collab_tool_call,web_search,todo_list,dynamic_tool_call,unknown_item\n" +
	"bounds=prompt:65536,line:262144,stdout:1048576,stderr:1048576,events:512,message:262144\n"

const adapterContract = "codex-subscription-adapter:v1\n" +
	"authority=chat_subscription_isolated_v1\n" +
	"runtime=codex_subscription_exec_v1\n" +
	"agent=codex-subscription\n" +
	"profile=codex-subscription:gpt-5.6-sol\n" +
	"adapter_id=model.chat.text-only.codex-subscription\n" +
	"developer_instruction_revision=" + DeveloperInstructionRevision + "\n" +
	"isolation_revision=" + IsolationRevision + "\n" +
	"response=codex-exec-jsonl-v1\n"

const policyContract = "codex-subscription-policy:v1\n" +
	"authority=chat_subscription_isolated_v1\n" +
	"runtime=codex_subscription_exec_v1\n" +
	"policy_id=codex-subscription-chat-v1\n" +
	"model=gpt-5.6-sol\n" +
	"reasoning_effort=medium\n" +
	"reasoning_context=current_turn\n" +
	"request_timeout_millis=120000\n" +
	"developer_instruction_revision=" + DeveloperInstructionRevision + "\n" +
	"account_type=chatgpt\n" +
	"adapter_id=model.chat.text-only.codex-subscription\n" +
	"thread_mode=ephemeral\n" +
	"sandbox_mode=readOnly\n" +
	"approval_policy=never\n" +
	"workdir_mode=empty_per_target\n" +
	"dynamic_tools_mode=none\n" +
	"mcp_mode=none\n" +
	"command_policy=deny_and_fail\n" +
	"file_read_policy=deny_and_fail\n" +
	"isolation_revision=" + IsolationRevision + "\n"

var disabledFeatures = [...]string{
	"shell_tool", "unified_exec", "apps", "browser_use", "computer_use", "image_generation",
	"in_app_browser", "multi_agent", "memories", "plugins", "skill_search", "workspace_dependencies",
	"code_mode", "code_mode_host", "code_mode_only", "code_mode_buffered_exec",
}

// DisabledFeatures returns the immutable feature-denial portion of the
// accepted execution contract. The returned slice is a copy.
func DisabledFeatures() []string {
	return append([]string(nil), disabledFeatures[:]...)
}

// RequiredConfigOverrides returns the exact strict config supplied to Codex.
// The returned slice is a copy and each entry is one value following -c.
func RequiredConfigOverrides() []string {
	return []string{
		`approval_policy="never"`,
		"developer_instructions=" + strconv.Quote(DeveloperInstruction),
		`model_reasoning_effort="medium"`,
		`web_search="disabled"`,
		"tools.update_plan.enabled=false",
		"tools.experimental_request_user_input.enabled=false",
		"skills.include_instructions=false",
		"skills.bundled.enabled=false",
		"include_apps_instructions=false",
		"include_collaboration_mode_instructions=false",
		"include_environment_context=false",
		"include_permissions_instructions=false",
	}
}

// ExecArguments constructs the exact direct argv after the held Codex
// executable path. Prompt, model, and workdir are already validated by the
// caller; no shell parses the returned values.
func ExecArguments(prompt, model, workdir string) []string {
	arguments := []string{
		"exec", prompt, "--json", "--sandbox", "read-only",
		"--skip-git-repo-check", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--strict-config",
		"-C", workdir, "--model", model,
	}
	for _, override := range RequiredConfigOverrides() {
		arguments = append(arguments, "-c", override)
	}
	for _, feature := range disabledFeatures {
		arguments = append(arguments, "--disable", feature)
	}
	return arguments
}
