package hermesbot

import (
	"context"
	"io"
)

const (
	HermesVersion  = "0.20.5"
	HermesRevision = "fcbd1076a93841fa88855acce810e342a5b78101"

	InventoryContractID       = "source-agent.inventory.hermes-bot.v1"
	InventoryContractRevision = "ed131bdba193ddebe4f4445296dcba9282f1d6672d2f8396fa94dd8c38959d3b"
)

// InventoryContractManifest is the immutable behavior input whose SHA-256 is
// InventoryContractRevision.
const InventoryContractManifest = InventoryContractID + "\n" +
	"hermes_version=" + HermesVersion + "\n" +
	"hermes_revision=" + HermesRevision + "\n" +
	"rpc=jsonrpc2,profiles.list,include_sessions:false,bot_mode_protocol:true\n" +
	"profile_schema=name,path,is_default,model:string|null,provider:string|null,description,display_name,skill_count,ui_meta?:object,ui_meta_revisions?:object,has_avatar\n" +
	"profile_ids=lower_ascii_1_to_64,reserved:hermes|test|tmp|root|sudo,default:allowed\n" +
	"identity=source-qualified-length-prefixed-sha256\n" +
	"projection=opaque_profile_id,normalized_display_name,one_utc_observation\n" +
	"limits=response_bytes:1048576,profiles:256,deadline_ms:5000,display_scalars:64\n" +
	"result=allocated,canonical_profile_sort,no_partial\n" +
	"readiness=ready:false,capabilities:allocated_empty\n" +
	"evidence=profile_discovered,execution_adapter_not_approved\n" +
	"resource_order=provider_credentials,filesystem,browser_sessions,framework_sessions,source_memory,tool_configuration\n" +
	"resource_sharing=unknown,machine_shared,machine_shared,profile_scoped,unknown,profile_scoped\n" +
	"attestation=connection_identity,launcher_sha256,code_root_sha256,hermes_version,hermes_revision\n" +
	"decoder=required_known_fields,duplicate_keys_rejected,invalid_utf8_rejected\n" +
	"privacy=discard_path_model_provider_description_ui_metadata,closed_errors\n" +
	"body=bounded,close_interrupts_read,join_on_cancel\n"

// LocalProfileRosterTransport is the sole external-dependency seam for the
// inventory adapter. It performs one bounded local RPC exchange and returns
// the out-of-band source measurements bound to that exchange. A returned Body
// must permit a concurrent Close to promptly interrupt Read; the adapter joins
// an interrupted read before it returns.
type LocalProfileRosterTransport interface {
	RoundTrip(context.Context, []byte) (VerifiedRPCExchange, error)
}

type VerifiedRPCExchange struct {
	ConnectionIdentity string
	LauncherDigest     string
	CodeRootDigest     string
	HermesVersion      string
	HermesRevision     string
	Body               io.ReadCloser
}

type LocalSourceContract struct {
	ConnectionIdentity string
	LauncherDigest     string
	CodeRootDigest     string
	HermesVersion      string
	HermesRevision     string
}
