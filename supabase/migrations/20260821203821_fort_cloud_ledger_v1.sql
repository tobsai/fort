-- Fort cloud ledger v1.
--
-- All operational data lives in an unexposed schema and is reachable only by
-- the server-side fort_gateway database role. The role intentionally has no
-- password in migration history; deployment supplies credentials separately.

do $fort_role$
declare
  unsafe_role boolean;
begin
  if not exists (select 1 from pg_roles where rolname = 'fort_gateway') then
    create role fort_gateway login nosuperuser nocreatedb nocreaterole
      noinherit noreplication nobypassrls;
  else
    select not (
      rollogin and not rolsuper and not rolcreatedb and not rolcreaterole
      and not rolinherit and not rolreplication and not rolbypassrls
    )
      into unsafe_role
      from pg_roles
     where rolname = 'fort_gateway';

    if unsafe_role then
      raise exception using
        errcode = '42501',
        message = 'fort_gateway_role_drift';
    end if;
  end if;
end
$fort_role$;

-- Migration operators may assume the runtime role for RLS acceptance tests;
-- fort_gateway itself remains NOINHERIT and NOBYPASSRLS.
grant fort_gateway to postgres;

create schema if not exists fort_private authorization postgres;

revoke all on schema fort_private from public, anon, authenticated, service_role;
revoke all privileges on all tables in schema fort_private
  from public, anon, authenticated, service_role;
revoke all privileges on all sequences in schema fort_private
  from public, anon, authenticated, service_role;
revoke all privileges on all functions in schema fort_private
  from public, anon, authenticated, service_role;

-- Opt out of the legacy public-schema auto-exposure behavior as well. Fort
-- deliberately creates no Data API in public.
alter default privileges for role postgres in schema public
  revoke select, insert, update, delete on tables
  from anon, authenticated, service_role;
alter default privileges for role postgres in schema public
  revoke usage, select on sequences from anon, authenticated, service_role;
alter default privileges for role postgres in schema public
  revoke execute on functions from public, anon, authenticated, service_role;

alter default privileges for role postgres in schema fort_private
  revoke all on tables from public, anon, authenticated, service_role, fort_gateway;
alter default privileges for role postgres in schema fort_private
  revoke all on sequences from public, anon, authenticated, service_role, fort_gateway;
alter default privileges for role postgres in schema fort_private
  revoke execute on functions from public, anon, authenticated, service_role, fort_gateway;

create table fort_private.fort_account (
  account_id uuid primary key,
  normalized_email text not null,
  state text not null default 'open' check (state in ('open', 'suspended', 'closed')),
  created_at timestamptz not null default clock_timestamp(),
  updated_at timestamptz not null default clock_timestamp(),
  constraint fort_account_email_normalized check (
    normalized_email = lower(btrim(normalized_email)) and normalized_email like '%@%'
  )
);
create unique index fort_account_email_unique
  on fort_private.fort_account (lower(normalized_email));

create table fort_private.worker (
  account_id uuid not null,
  worker_id text not null check (btrim(worker_id) <> ''),
  machine_id text not null check (btrim(machine_id) <> ''),
  display_name text not null check (btrim(display_name) <> ''),
  identity_key_digest text not null check (identity_key_digest ~ '^[0-9a-f]{64}$'),
  enrollment_token_hash text not null check (enrollment_token_hash ~ '^[0-9a-f]{64}$'),
  state text not null check (state in ('enrolled', 'offline', 'revoked')),
  enrolled_at timestamptz not null,
  last_seen_at timestamptz,
  revoked_at timestamptz,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, worker_id),
  unique (account_id, machine_id),
  unique (account_id, identity_key_digest),
  unique (account_id, enrollment_token_hash),
  foreign key (account_id) references fort_private.fort_account(account_id),
  constraint worker_revocation_consistent check (
    (state = 'revoked' and revoked_at is not null)
    or (state <> 'revoked' and revoked_at is null)
  )
);

create table fort_private.worker_capability_revision (
  account_id uuid not null,
  capability_revision_id text not null check (btrim(capability_revision_id) <> ''),
  worker_id text not null,
  revision integer not null check (revision > 0),
  capability_evidence jsonb not null check (jsonb_typeof(capability_evidence) = 'object'),
  evidence_digest text not null check (evidence_digest ~ '^[0-9a-f]{64}$'),
  observed_at timestamptz not null,
  primary key (account_id, capability_revision_id),
  unique (account_id, worker_id, revision),
  unique (account_id, worker_id, capability_revision_id),
  foreign key (account_id, worker_id)
    references fort_private.worker(account_id, worker_id)
);

create table fort_private.execution_source (
  account_id uuid not null,
  execution_source_id text not null check (btrim(execution_source_id) <> ''),
  worker_id text not null,
  framework_family text not null check (btrim(framework_family) <> ''),
  gateway_id text not null check (btrim(gateway_id) <> ''),
  instance_id text not null check (btrim(instance_id) <> ''),
  display_name text not null check (btrim(display_name) <> ''),
  resource_sharing jsonb not null check (jsonb_typeof(resource_sharing) = 'object'),
  source_config_digest text not null check (source_config_digest ~ '^[0-9a-f]{64}$'),
  discovered_at timestamptz not null,
  primary key (account_id, execution_source_id),
  unique (account_id, framework_family, gateway_id, instance_id),
  unique (account_id, worker_id, execution_source_id),
  foreign key (account_id, worker_id)
    references fort_private.worker(account_id, worker_id)
);

-- Source inventory can observe configuration drift after a Binding has been
-- accepted. These observations are append-only and ordered by a database
-- sequence so a delayed or forged wall clock cannot become "latest".
create table fort_private.execution_source_config_observation (
  account_id uuid not null,
  observation_id text not null check (btrim(observation_id) <> ''),
  observation_sequence bigint generated always as identity,
  execution_source_id text not null,
  source_config_digest text not null check (source_config_digest ~ '^[0-9a-f]{64}$'),
  observed_by text not null check (btrim(observed_by) <> ''),
  observed_at timestamptz not null,
  primary key (account_id, observation_id),
  unique (account_id, observation_sequence),
  foreign key (account_id, execution_source_id)
    references fort_private.execution_source(account_id, execution_source_id)
);
create index execution_source_config_observation_latest
  on fort_private.execution_source_config_observation
    (account_id, execution_source_id, observation_sequence desc);

create table fort_private.source_agent (
  account_id uuid not null,
  source_agent_id text not null check (btrim(source_agent_id) <> ''),
  execution_source_id text not null,
  opaque_source_agent_id text not null check (btrim(opaque_source_agent_id) <> ''),
  display_name text not null check (btrim(display_name) <> ''),
  inventory_digest text not null check (inventory_digest ~ '^[0-9a-f]{64}$'),
  discovered_at timestamptz not null,
  primary key (account_id, source_agent_id),
  unique (account_id, execution_source_id, opaque_source_agent_id),
  unique (account_id, execution_source_id, source_agent_id),
  foreign key (account_id, execution_source_id)
    references fort_private.execution_source(account_id, execution_source_id)
);

-- Circular Agent/revision/Conversation references are deferrable so one
-- account-scoped create command can insert the entire aggregate atomically.
create table fort_private.stable_agent (
  account_id uuid not null,
  agent_id text not null check (btrim(agent_id) <> ''),
  state text not null check (state in ('open', 'archived')),
  current_profile_revision_id text not null,
  current_behavior_revision_id text not null,
  current_binding_revision_id text not null,
  canonical_conversation_id text not null,
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, agent_id),
  unique (account_id, canonical_conversation_id),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

create table fort_private.agent_profile_revision (
  account_id uuid not null,
  profile_revision_id text not null check (btrim(profile_revision_id) <> ''),
  agent_id text not null,
  revision integer not null check (revision > 0),
  name text not null check (btrim(name) <> ''),
  title text not null,
  avatar_url text not null,
  hidden boolean not null default false,
  pinned boolean not null default false,
  sort_order integer not null default 0,
  created_by text not null check (btrim(created_by) <> ''),
  created_at timestamptz not null,
  primary key (account_id, profile_revision_id),
  unique (account_id, agent_id, revision),
  unique (account_id, agent_id, profile_revision_id),
  foreign key (account_id, agent_id)
    references fort_private.stable_agent(account_id, agent_id)
    deferrable initially deferred
);

create table fort_private.agent_behavior_revision (
  account_id uuid not null,
  behavior_revision_id text not null check (btrim(behavior_revision_id) <> ''),
  agent_id text not null,
  revision integer not null check (revision > 0),
  role text not null,
  standing_instructions text not null,
  enabled_skills jsonb not null check (jsonb_typeof(enabled_skills) = 'array'),
  enabled_tools jsonb not null check (jsonb_typeof(enabled_tools) = 'array'),
  prompt_material text not null,
  behavior_digest text not null check (behavior_digest ~ '^[0-9a-f]{64}$'),
  created_by text not null check (btrim(created_by) <> ''),
  created_at timestamptz not null,
  primary key (account_id, behavior_revision_id),
  unique (account_id, agent_id, revision),
  unique (account_id, agent_id, behavior_revision_id),
  foreign key (account_id, agent_id)
    references fort_private.stable_agent(account_id, agent_id)
    deferrable initially deferred
);

create table fort_private.agent_binding_revision (
  account_id uuid not null,
  binding_revision_id text not null check (btrim(binding_revision_id) <> ''),
  agent_id text not null,
  revision integer not null check (revision > 0),
  behavior_revision_id text not null,
  execution_source_id text not null,
  source_agent_id text not null,
  worker_id text not null,
  seat_id text not null check (btrim(seat_id) <> ''),
  fort_profile text not null check (btrim(fort_profile) <> ''),
  provider text not null check (btrim(provider) <> ''),
  requested_model text not null check (btrim(requested_model) <> ''),
  resolved_model text not null check (btrim(resolved_model) <> ''),
  adapter_id text not null check (btrim(adapter_id) <> ''),
  adapter_revision text not null check (btrim(adapter_revision) <> ''),
  source_config_digest text not null check (source_config_digest ~ '^[0-9a-f]{64}$'),
  authority_id text not null check (btrim(authority_id) <> ''),
  authority_revision text not null check (btrim(authority_revision) <> ''),
  policy_id text not null check (btrim(policy_id) <> ''),
  policy_revision text not null check (btrim(policy_revision) <> ''),
  session_behavior text not null check (btrim(session_behavior) <> ''),
  memory_behavior text not null check (btrim(memory_behavior) <> ''),
  capability_evidence jsonb not null check (jsonb_typeof(capability_evidence) = 'object'),
  readiness_contract_id text not null check (btrim(readiness_contract_id) <> ''),
  readiness_contract_revision text not null check (btrim(readiness_contract_revision) <> ''),
  supersedes_binding_revision_id text,
  activated_at timestamptz not null,
  primary key (account_id, binding_revision_id),
  unique (account_id, agent_id, revision),
  unique (account_id, agent_id, binding_revision_id),
  foreign key (account_id, agent_id)
    references fort_private.stable_agent(account_id, agent_id)
    deferrable initially deferred,
  foreign key (account_id, agent_id, behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, execution_source_id, source_agent_id)
    references fort_private.source_agent(account_id, execution_source_id, source_agent_id),
  foreign key (account_id, worker_id, execution_source_id)
    references fort_private.execution_source(account_id, worker_id, execution_source_id),
  foreign key (account_id, agent_id, supersedes_binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id)
);

-- A Binding Revision is immutable, so retirement/supersession evidence lives
-- in its own append-only transition record. This prevents an accepted
-- Behavior change or explicit Rebind from rewriting the predecessor snapshot.
create table fort_private.agent_binding_transition (
  account_id uuid not null,
  agent_id text not null,
  kind text not null check (kind in ('behavior', 'rebind')),
  previous_behavior_revision_id text not null,
  successor_behavior_revision_id text not null,
  previous_binding_revision_id text not null,
  successor_binding_revision_id text not null,
  preview_digest text not null check (preview_digest ~ '^[0-9a-f]{64}$'),
  non_transferable_resources jsonb not null
    check (jsonb_typeof(non_transferable_resources) = 'array'),
  readiness_evidence jsonb not null
    check (jsonb_typeof(readiness_evidence) = 'array'),
  authority_evidence jsonb not null
    check (jsonb_typeof(authority_evidence) = 'array'),
  accepted_by text not null check (btrim(accepted_by) <> ''),
  accepted_at timestamptz not null,
  primary key (account_id, agent_id, successor_binding_revision_id),
  unique (account_id, agent_id, previous_binding_revision_id),
  foreign key (account_id, agent_id, previous_behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, agent_id, successor_behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, agent_id, previous_binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id),
  foreign key (account_id, agent_id, successor_binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id)
);

alter table fort_private.stable_agent
  add constraint stable_agent_current_profile_fk
  foreign key (account_id, agent_id, current_profile_revision_id)
  references fort_private.agent_profile_revision(account_id, agent_id, profile_revision_id)
  deferrable initially deferred,
  add constraint stable_agent_current_behavior_fk
  foreign key (account_id, agent_id, current_behavior_revision_id)
  references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id)
  deferrable initially deferred,
  add constraint stable_agent_current_binding_fk
  foreign key (account_id, agent_id, current_binding_revision_id)
  references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id)
  deferrable initially deferred;

create table fort_private.conversation (
  account_id uuid not null,
  conversation_id text not null check (btrim(conversation_id) <> ''),
  kind text not null check (kind in ('agent', 'group', 'legacy')),
  title text not null,
  state text not null check (state in ('open', 'archived')),
  current_membership_revision_id text not null,
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, conversation_id),
  unique (account_id, conversation_id, current_membership_revision_id),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

-- A Group is a stable product identity distinct from the Conversation that
-- owns its transcript and membership revisions.
create table fort_private.group_conversation (
  account_id uuid not null,
  group_id text not null check (btrim(group_id) <> ''),
  conversation_id text not null,
  created_at timestamptz not null,
  primary key (account_id, group_id),
  unique (account_id, conversation_id),
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id)
    deferrable initially deferred
);

create table fort_private.agent_conversation (
  account_id uuid not null,
  agent_id text not null,
  conversation_id text not null,
  kind text not null check (kind in ('canonical', 'secondary')),
  created_at timestamptz not null,
  primary key (account_id, agent_id, conversation_id),
  unique (account_id, conversation_id),
  unique (account_id, agent_id, conversation_id, kind),
  foreign key (account_id, agent_id)
    references fort_private.stable_agent(account_id, agent_id)
    deferrable initially deferred,
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id)
    deferrable initially deferred
);
create unique index agent_conversation_one_canonical
  on fort_private.agent_conversation (account_id, agent_id)
  where kind = 'canonical';

-- Pinning is presentation state, not Conversation activity and not Agent
-- identity. Each change appends a revision so unpinning preserves audit
-- evidence and cannot reorder immutable transcript history.
create table fort_private.agent_conversation_pin (
  account_id uuid not null,
  agent_id text not null,
  conversation_id text not null,
  revision integer not null check (revision > 0),
  pinned boolean not null,
  changed_by text not null check (btrim(changed_by) <> ''),
  changed_at timestamptz not null,
  primary key (account_id, agent_id, conversation_id, revision),
  foreign key (account_id, agent_id, conversation_id)
    references fort_private.agent_conversation(account_id, agent_id, conversation_id)
);

alter table fort_private.stable_agent
  add constraint stable_agent_canonical_conversation_fk
  foreign key (account_id, canonical_conversation_id)
  references fort_private.conversation(account_id, conversation_id)
  deferrable initially deferred;

create table fort_private.conversation_membership_revision (
  account_id uuid not null,
  membership_revision_id text not null check (btrim(membership_revision_id) <> ''),
  conversation_id text not null,
  revision integer not null check (revision > 0),
  command_digest text not null check (command_digest ~ '^[0-9a-f]{64}$'),
  created_by text not null check (btrim(created_by) <> ''),
  created_at timestamptz not null,
  primary key (account_id, membership_revision_id),
  unique (account_id, conversation_id, revision),
  unique (account_id, conversation_id, membership_revision_id),
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id)
    deferrable initially deferred
);

create table fort_private.conversation_member_revision (
  account_id uuid not null,
  membership_revision_id text not null,
  conversation_id text not null,
  agent_id text not null,
  position integer not null check (position >= 0 and position < 6),
  added_by text not null check (btrim(added_by) <> ''),
  created_at timestamptz not null,
  primary key (account_id, membership_revision_id, agent_id),
  unique (account_id, membership_revision_id, position),
  unique (account_id, membership_revision_id, conversation_id, agent_id),
  foreign key (account_id, conversation_id, membership_revision_id)
    references fort_private.conversation_membership_revision(
      account_id, conversation_id, membership_revision_id
    )
    deferrable initially deferred,
  foreign key (account_id, agent_id)
    references fort_private.stable_agent(account_id, agent_id)
);

alter table fort_private.conversation
  add constraint conversation_current_membership_fk
  foreign key (account_id, conversation_id, current_membership_revision_id)
  references fort_private.conversation_membership_revision(
    account_id, conversation_id, membership_revision_id
  )
  deferrable initially deferred;

create table fort_private.conversation_participant (
  account_id uuid not null,
  participant_id text not null check (btrim(participant_id) <> ''),
  conversation_id text not null,
  agent_id text not null,
  behavior_revision_id text not null,
  binding_revision_id text not null,
  seat_snapshot jsonb not null check (jsonb_typeof(seat_snapshot) = 'object'),
  authority_snapshot jsonb not null check (jsonb_typeof(authority_snapshot) = 'object'),
  snapshot_digest text not null check (snapshot_digest ~ '^[0-9a-f]{64}$'),
  created_at timestamptz not null,
  primary key (account_id, participant_id),
  unique (account_id, conversation_id, agent_id, binding_revision_id),
  unique (
    account_id, conversation_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ),
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  foreign key (account_id, agent_id, behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, agent_id, binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id)
);

create table fort_private.delegation_grant (
  account_id uuid not null,
  delegation_grant_id text not null check (btrim(delegation_grant_id) <> ''),
  source_kind text not null check (source_kind in ('human_turn', 'direct_handoff', 'routine_occurrence')),
  source_id text not null check (btrim(source_id) <> ''),
  authority_grant jsonb not null check (jsonb_typeof(authority_grant) = 'object'),
  grant_digest text not null check (grant_digest ~ '^[0-9a-f]{64}$'),
  maximum_agent_messages integer not null check (maximum_agent_messages between 1 and 10),
  maximum_handoff_depth integer not null check (maximum_handoff_depth between 0 and 3),
  hard_deadline timestamptz not null,
  created_by text not null check (btrim(created_by) <> ''),
  created_at timestamptz not null,
  primary key (account_id, delegation_grant_id),
  unique (account_id, source_kind, source_id),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

create table fort_private.artifact (
  account_id uuid not null,
  artifact_id text not null check (btrim(artifact_id) <> ''),
  execution_attempt_id text not null,
  kind text not null check (kind in ('context', 'output')),
  state text not null check (state in ('uploading', 'finalized', 'failed')),
  expected_chunk_count integer not null check (expected_chunk_count between 1 and 64),
  expected_plaintext_length bigint not null check (
    expected_plaintext_length >= 0 and expected_plaintext_length <= 134217728
  ),
  expected_encoded_length bigint not null check (
    expected_encoded_length >= 0 and expected_encoded_length <= 268435456
  ),
  logical_digest text not null check (logical_digest ~ '^[0-9a-f]{64}$'),
  encryption_key_id text not null check (
    encryption_key_id = btrim(encryption_key_id) and
    length(encryption_key_id) between 1 and 256 and
    encryption_key_id !~ '[[:space:]]'
  ),
  created_at timestamptz not null,
  finalized_at timestamptz,
  primary key (account_id, artifact_id),
  unique (account_id, execution_attempt_id, artifact_id),
  constraint artifact_finalization_consistent check (
    (state = 'finalized' and finalized_at is not null)
    or (state <> 'finalized' and finalized_at is null)
  )
);

create table fort_private.artifact_chunk (
  account_id uuid not null,
  artifact_id text not null,
  chunk_index integer not null check (chunk_index >= 0 and chunk_index < 64),
  ciphertext bytea not null,
  encoded_length integer not null check (encoded_length > 0 and encoded_length <= 4194304),
  plaintext_length integer not null check (plaintext_length >= 0 and plaintext_length <= 2097152),
  encryption_key_id text not null check (
    encryption_key_id = btrim(encryption_key_id) and
    length(encryption_key_id) between 1 and 256 and
    encryption_key_id !~ '[[:space:]]'
  ),
  nonce bytea not null check (octet_length(nonce) between 12 and 64),
  authenticated_digest text not null check (authenticated_digest ~ '^[0-9a-f]{64}$'),
  created_at timestamptz not null,
  primary key (account_id, artifact_id, chunk_index),
  foreign key (account_id, artifact_id)
    references fort_private.artifact(account_id, artifact_id),
  constraint artifact_chunk_encoded_length_matches check (
    octet_length(ciphertext) = encoded_length
  )
);

create table fort_private.context_manifest (
  account_id uuid not null,
  context_manifest_id text not null check (btrim(context_manifest_id) <> ''),
  purpose text not null check (purpose in ('turn', 'handoff', 'routine')),
  manifest_digest text not null check (manifest_digest ~ '^[0-9a-f]{64}$'),
  created_by text not null check (btrim(created_by) <> ''),
  created_at timestamptz not null,
  primary key (account_id, context_manifest_id),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

create table fort_private.conversation_message (
  account_id uuid not null,
  message_id bigint generated always as identity,
  conversation_id text not null,
  turn_id text,
  target_id text,
  handoff_id text,
  routine_run_id text,
  message_kind text not null check (
    message_kind in ('human', 'agent', 'system', 'handoff_result', 'routine_result')
  ),
  author_kind text not null check (author_kind in ('human', 'agent', 'system')),
  author_id text not null check (btrim(author_id) <> ''),
  author_agent_id text,
  body_ciphertext bytea not null,
  body_key_id text not null check (btrim(body_key_id) <> ''),
  body_nonce bytea not null check (octet_length(body_nonce) >= 12),
  body_digest text not null check (body_digest ~ '^[0-9a-f]{64}$'),
  body_plaintext_length integer not null check (
    body_plaintext_length >= 0 and body_plaintext_length <= 4194304
  ),
  created_at timestamptz not null,
  primary key (account_id, message_id),
  unique (account_id, conversation_id, message_id),
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  foreign key (account_id, author_agent_id)
    references fort_private.stable_agent(account_id, agent_id),
  constraint message_author_agent_consistent check (
    (author_kind = 'agent' and author_agent_id is not null)
    or (author_kind <> 'agent' and author_agent_id is null)
  ),
  constraint message_handoff_kind_consistent check (
    (message_kind = 'handoff_result' and handoff_id is not null)
    or (message_kind <> 'handoff_result' and handoff_id is null)
  ),
  constraint message_routine_kind_consistent check (
    (message_kind = 'routine_result' and routine_run_id is not null)
    or (message_kind <> 'routine_result' and routine_run_id is null)
  )
);
create unique index conversation_message_one_handoff_result
  on fort_private.conversation_message (account_id, handoff_id)
  where handoff_id is not null;
create unique index conversation_message_one_routine_result
  on fort_private.conversation_message (account_id, routine_run_id)
  where routine_run_id is not null;
create unique index conversation_message_one_agent_result
  on fort_private.conversation_message (account_id, target_id)
  where target_id is not null and message_kind = 'agent';
create index conversation_message_order
  on fort_private.conversation_message (account_id, conversation_id, message_id);

create table fort_private.context_manifest_message (
  account_id uuid not null,
  context_manifest_id text not null,
  ordinal integer not null check (ordinal >= 0 and ordinal < 256),
  message_id bigint not null,
  primary key (account_id, context_manifest_id, ordinal),
  unique (account_id, context_manifest_id, message_id),
  foreign key (account_id, context_manifest_id)
    references fort_private.context_manifest(account_id, context_manifest_id),
  foreign key (account_id, message_id)
    references fort_private.conversation_message(account_id, message_id)
);

create table fort_private.context_manifest_artifact (
  account_id uuid not null,
  context_manifest_id text not null,
  ordinal integer not null check (ordinal >= 0 and ordinal < 256),
  artifact_id text not null,
  primary key (account_id, context_manifest_id, ordinal),
  unique (account_id, context_manifest_id, artifact_id),
  foreign key (account_id, context_manifest_id)
    references fort_private.context_manifest(account_id, context_manifest_id),
  foreign key (account_id, artifact_id)
    references fort_private.artifact(account_id, artifact_id)
);

create table fort_private.conversation_turn (
  account_id uuid not null,
  turn_id text not null check (btrim(turn_id) <> ''),
  conversation_id text not null,
  client_turn_id text not null check (btrim(client_turn_id) <> ''),
  idempotency_key text not null check (btrim(idempotency_key) <> ''),
  command_digest text not null check (command_digest ~ '^[0-9a-f]{64}$'),
  kind text not null check (kind in ('human_direct', 'human_group', 'handoff', 'routine')),
  prompt_message_id bigint not null,
  through_message_id bigint not null,
  membership_revision_id text not null,
  context_manifest_id text not null,
  delegation_grant_id text not null,
  concurrency_policy text not null check (concurrency_policy in ('serial', 'parallel')),
  cancellation_policy jsonb not null check (jsonb_typeof(cancellation_policy) = 'object'),
  approval_policy jsonb not null check (jsonb_typeof(approval_policy) = 'object'),
  maximum_agent_messages integer not null check (maximum_agent_messages between 1 and 10),
  maximum_handoff_depth integer not null check (maximum_handoff_depth between 0 and 3),
  cost_limit_classification text not null check (
    cost_limit_classification in ('hard', 'informational', 'unknown')
  ),
  token_limit_classification text not null check (
    token_limit_classification in ('hard', 'informational', 'unknown')
  ),
  cost_limit numeric(20, 6),
  token_limit bigint,
  hard_deadline timestamptz not null,
  state text not null check (state in ('open', 'settled', 'needs_you', 'canceled')),
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, turn_id),
  unique (account_id, conversation_id, client_turn_id),
  unique (account_id, conversation_id, idempotency_key),
  unique (account_id, turn_id, membership_revision_id),
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  foreign key (account_id, conversation_id, membership_revision_id)
    references fort_private.conversation_membership_revision(
      account_id, conversation_id, membership_revision_id
    ),
  foreign key (account_id, prompt_message_id)
    references fort_private.conversation_message(account_id, message_id)
    deferrable initially deferred,
  foreign key (account_id, context_manifest_id)
    references fort_private.context_manifest(account_id, context_manifest_id),
  foreign key (account_id, delegation_grant_id)
    references fort_private.delegation_grant(account_id, delegation_grant_id),
  constraint turn_snapshot_order check (through_message_id >= prompt_message_id),
  constraint turn_limit_values check (
    (cost_limit is null or cost_limit >= 0) and
    (token_limit is null or token_limit >= 0)
  )
);

create table fort_private.conversation_target (
  account_id uuid not null,
  target_id text not null check (btrim(target_id) <> ''),
  turn_id text not null,
  conversation_id text not null,
  agent_id text not null,
  membership_revision_id text not null,
  target_kind text not null check (target_kind in ('initial', 'handoff', 'routine')),
  origin_id text not null check (btrim(origin_id) <> ''),
  run_id text not null check (btrim(run_id) <> ''),
  state text not null check (
    state in (
      'queued', 'claimed', 'working', 'needs_you', 'cancel_requested',
      'canceled', 'succeeded', 'failed', 'lease_expired'
    )
  ),
  attempt_count integer not null default 0 check (attempt_count >= 0),
  error_code text,
  error_ciphertext bytea,
  error_key_id text,
  error_nonce bytea,
  error_digest text,
  hard_deadline timestamptz not null,
  cancellation_policy jsonb not null check (jsonb_typeof(cancellation_policy) = 'object'),
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, target_id),
  unique (account_id, run_id),
  unique (account_id, target_id, agent_id, membership_revision_id),
  unique (
    account_id, target_id, conversation_id, agent_id, membership_revision_id
  ),
  foreign key (account_id, turn_id, membership_revision_id)
    references fort_private.conversation_turn(account_id, turn_id, membership_revision_id),
  foreign key (account_id, membership_revision_id, conversation_id, agent_id)
    references fort_private.conversation_member_revision(
      account_id, membership_revision_id, conversation_id, agent_id
    ),
  constraint target_error_envelope_consistent check (
    (error_ciphertext is null and error_key_id is null and error_nonce is null and error_digest is null)
    or (
      error_ciphertext is not null and btrim(error_key_id) <> '' and
      octet_length(error_nonce) >= 12 and error_digest ~ '^[0-9a-f]{64}$'
    )
  )
);
create unique index conversation_target_one_initial_per_agent
  on fort_private.conversation_target (account_id, turn_id, agent_id)
  where target_kind = 'initial';
create index conversation_target_queue
  on fort_private.conversation_target (account_id, state, created_at, target_id)
  where state in ('queued', 'lease_expired');

create table fort_private.conversation_target_binding (
  account_id uuid not null,
  target_id text not null,
  conversation_id text not null,
  agent_id text not null,
  behavior_revision_id text not null,
  binding_revision_id text not null,
  participant_id text not null,
  membership_revision_id text not null,
  pinned_at timestamptz not null,
  primary key (account_id, target_id),
  foreign key (
    account_id, target_id, conversation_id, agent_id, membership_revision_id
  )
    references fort_private.conversation_target(
      account_id, target_id, conversation_id, agent_id, membership_revision_id
    ),
  foreign key (
    account_id, agent_id, behavior_revision_id
  ) references fort_private.agent_behavior_revision(
    account_id, agent_id, behavior_revision_id
  ),
  foreign key (
    account_id, agent_id, binding_revision_id
  ) references fort_private.agent_binding_revision(
    account_id, agent_id, binding_revision_id
  ),
  foreign key (
    account_id, conversation_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ) references fort_private.conversation_participant(
    account_id, conversation_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ),
  unique (
    account_id, target_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  )
);

create table fort_private.execution_attempt (
  account_id uuid not null,
  execution_attempt_id text not null check (btrim(execution_attempt_id) <> ''),
  target_id text not null,
  attempt_number integer not null check (attempt_number > 0),
  agent_id text not null,
  behavior_revision_id text not null,
  binding_revision_id text not null,
  participant_id text not null,
  worker_id text,
  worker_capability_revision_id text,
  state text not null check (
    state in (
      'queued', 'leased', 'working', 'needs_you', 'cancel_requested',
      'canceled', 'succeeded', 'failed', 'lease_expired'
    )
  ),
  provider_thread_id text,
  provider_terminal_status text,
  observed_runtime jsonb check (observed_runtime is null or jsonb_typeof(observed_runtime) = 'object'),
  usage_evidence jsonb check (usage_evidence is null or jsonb_typeof(usage_evidence) = 'object'),
  terminal_receipt_id text,
  terminal_receipt_ciphertext bytea,
  terminal_receipt_key_id text,
  terminal_receipt_nonce bytea,
  terminal_receipt_digest text,
  started_at timestamptz,
  terminal_at timestamptz,
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, execution_attempt_id),
  unique (account_id, terminal_receipt_id),
  unique (account_id, target_id, attempt_number),
  unique (
    account_id, execution_attempt_id, target_id, agent_id,
    behavior_revision_id, binding_revision_id
  ),
  unique (
    account_id, execution_attempt_id, agent_id,
    behavior_revision_id, binding_revision_id
  ),
  foreign key (account_id, target_id)
    references fort_private.conversation_target(account_id, target_id),
  foreign key (
    account_id, target_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ) references fort_private.conversation_target_binding(
    account_id, target_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ),
  foreign key (account_id, worker_id, worker_capability_revision_id)
    references fort_private.worker_capability_revision(
      account_id, worker_id, capability_revision_id
    ),
  constraint attempt_worker_evidence_consistent check (
    (worker_id is null and worker_capability_revision_id is null)
    or (worker_id is not null and worker_capability_revision_id is not null)
  ),
  constraint attempt_terminal_receipt_consistent check (
    (
      terminal_receipt_id is null and terminal_receipt_ciphertext is null and terminal_receipt_key_id is null and
      terminal_receipt_nonce is null and terminal_receipt_digest is null
    ) or (
      btrim(terminal_receipt_id) <> '' and terminal_receipt_ciphertext is not null and btrim(terminal_receipt_key_id) <> '' and
      octet_length(terminal_receipt_nonce) >= 12 and
      terminal_receipt_digest ~ '^[0-9a-f]{64}$'
    )
  ),
  constraint attempt_terminal_time_consistent check (
    (state in ('canceled', 'succeeded', 'failed') and terminal_at is not null)
    or (state not in ('canceled', 'succeeded', 'failed') and terminal_at is null)
  )
);

alter table fort_private.artifact
  add constraint artifact_execution_attempt_fk
  foreign key (account_id, execution_attempt_id)
  references fort_private.execution_attempt(account_id, execution_attempt_id);

create table fort_private.worker_lease (
  account_id uuid not null,
  lease_id text not null check (btrim(lease_id) <> ''),
  fence_token bigint generated always as identity,
  worker_id text not null,
  execution_attempt_id text not null,
  target_id text not null,
  agent_id text not null,
  behavior_revision_id text not null,
  binding_revision_id text not null,
  state text not null check (state in ('active', 'released', 'expired', 'revoked')),
  claimed_at timestamptz not null,
  heartbeat_at timestamptz not null,
  expires_at timestamptz not null,
  released_at timestamptz,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, lease_id),
  unique (account_id, fence_token),
  unique (account_id, execution_attempt_id, lease_id),
  unique (account_id, execution_attempt_id, lease_id, fence_token),
  unique (
    account_id, execution_attempt_id, lease_id, fence_token,
    target_id, worker_id
  ),
  foreign key (
    account_id, execution_attempt_id, target_id, agent_id,
    behavior_revision_id, binding_revision_id
  ) references fort_private.execution_attempt(
    account_id, execution_attempt_id, target_id, agent_id,
    behavior_revision_id, binding_revision_id
  ),
  foreign key (account_id, worker_id)
    references fort_private.worker(account_id, worker_id),
  constraint worker_lease_time_order check (
    heartbeat_at >= claimed_at and expires_at > heartbeat_at
  ),
  constraint worker_lease_release_consistent check (
    (state = 'active' and released_at is null)
    or (state <> 'active' and released_at is not null)
  )
);
create unique index worker_lease_one_active_target
  on fort_private.worker_lease (account_id, target_id)
  where state = 'active';
create unique index worker_lease_one_active_attempt
  on fort_private.worker_lease (account_id, execution_attempt_id)
  where state = 'active';

create table fort_private.worker_cancellation_ack (
  account_id uuid not null,
  cancellation_ack_id text not null check (btrim(cancellation_ack_id) <> ''),
  target_id text not null,
  execution_attempt_id text not null,
  lease_id text not null,
  fence_token bigint not null,
  worker_id text not null,
  machine_id text not null check (btrim(machine_id) <> ''),
  idempotency_key text not null check (btrim(idempotency_key) <> ''),
  acknowledged_at timestamptz not null,
  primary key (account_id, cancellation_ack_id),
  unique (account_id, execution_attempt_id),
  unique (account_id, worker_id, idempotency_key),
  foreign key (account_id, target_id)
    references fort_private.conversation_target(account_id, target_id),
  constraint worker_cancellation_ack_exact_lease_fk
  foreign key (
    account_id, execution_attempt_id, lease_id, fence_token,
    target_id, worker_id
  )
    references fort_private.worker_lease(
      account_id, execution_attempt_id, lease_id, fence_token,
      target_id, worker_id
    ),
  foreign key (account_id, worker_id)
    references fort_private.worker(account_id, worker_id)
);

create table fort_private.worker_command (
  account_id uuid not null,
  worker_command_id text not null check (btrim(worker_command_id) <> ''),
  worker_id text not null,
  target_id text,
  execution_attempt_id text,
  lease_id text,
  kind text not null check (kind in ('start', 'cancel', 'probe', 'fence')),
  idempotency_key text not null check (btrim(idempotency_key) <> ''),
  command_digest text not null check (command_digest ~ '^[0-9a-f]{64}$'),
  payload_ciphertext bytea not null,
  payload_key_id text not null check (btrim(payload_key_id) <> ''),
  payload_nonce bytea not null check (octet_length(payload_nonce) >= 12),
  state text not null check (state in ('queued', 'claimed', 'completed', 'failed', 'canceled')),
  created_at timestamptz not null,
  claimed_at timestamptz,
  completed_at timestamptz,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, worker_command_id),
  unique (account_id, worker_id, idempotency_key),
  foreign key (account_id, worker_id)
    references fort_private.worker(account_id, worker_id),
  foreign key (account_id, target_id)
    references fort_private.conversation_target(account_id, target_id),
  foreign key (account_id, execution_attempt_id)
    references fort_private.execution_attempt(account_id, execution_attempt_id),
  foreign key (account_id, lease_id)
    references fort_private.worker_lease(account_id, lease_id)
);

create table fort_private.approval_receipt (
  account_id uuid not null,
  approval_receipt_id text not null check (btrim(approval_receipt_id) <> ''),
  subject_kind text not null check (subject_kind in ('handoff', 'routine', 'target')),
  subject_id text not null check (btrim(subject_id) <> ''),
  decision text not null check (decision in ('approved', 'denied')),
  authority_delta jsonb not null check (jsonb_typeof(authority_delta) = 'object'),
  receipt_ciphertext bytea not null,
  receipt_key_id text not null check (btrim(receipt_key_id) <> ''),
  receipt_nonce bytea not null check (octet_length(receipt_nonce) >= 12),
  receipt_digest text not null check (receipt_digest ~ '^[0-9a-f]{64}$'),
  decided_by text not null check (btrim(decided_by) <> ''),
  decided_at timestamptz not null,
  primary key (account_id, approval_receipt_id),
  unique (account_id, subject_kind, subject_id, approval_receipt_id),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

create table fort_private.handoff_emitter_receipt (
  account_id uuid not null,
  emitter_receipt_id text not null check (btrim(emitter_receipt_id) <> ''),
  source_execution_attempt_id text not null,
  source_agent_id text not null,
  source_behavior_revision_id text not null,
  source_binding_revision_id text not null,
  emitter_adapter_id text not null check (btrim(emitter_adapter_id) <> ''),
  emitter_adapter_revision text not null check (btrim(emitter_adapter_revision) <> ''),
  structured_command_digest text not null check (structured_command_digest ~ '^[0-9a-f]{64}$'),
  emitted_at timestamptz not null,
  primary key (account_id, emitter_receipt_id),
  foreign key (
    account_id, source_execution_attempt_id, source_agent_id,
    source_behavior_revision_id, source_binding_revision_id
  ) references fort_private.execution_attempt(
    account_id, execution_attempt_id, agent_id,
    behavior_revision_id, binding_revision_id
  )
);

create table fort_private.handoff (
  account_id uuid not null,
  handoff_id text not null check (btrim(handoff_id) <> ''),
  idempotency_key text not null check (btrim(idempotency_key) <> ''),
  command_digest text not null check (command_digest ~ '^[0-9a-f]{64}$'),
  state text not null check (
    state in (
      'requested', 'needs_approval', 'queued', 'working', 'needs_you',
      'canceled', 'succeeded', 'failed'
    )
  ),
  creation_actor_kind text not null check (creation_actor_kind in ('human', 'agent', 'routine')),
  creation_actor_id text not null check (btrim(creation_actor_id) <> ''),
  emitter_receipt_id text,
  source_turn_id text,
  source_message_id bigint not null,
  source_agent_id text,
  source_behavior_revision_id text,
  source_binding_revision_id text,
  recipient_agent_id text not null,
  recipient_behavior_revision_id text not null,
  recipient_binding_revision_id text not null,
  source_conversation_id text not null,
  output_conversation_id text not null,
  context_manifest_id text not null,
  requested_result_ciphertext bytea not null,
  requested_result_key_id text not null check (btrim(requested_result_key_id) <> ''),
  requested_result_nonce bytea not null check (octet_length(requested_result_nonce) >= 12),
  requested_result_digest text not null check (requested_result_digest ~ '^[0-9a-f]{64}$'),
  delegation_grant_id text not null,
  handoff_policy jsonb not null check (jsonb_typeof(handoff_policy) = 'object'),
  effective_authority jsonb not null check (jsonb_typeof(effective_authority) = 'object'),
  effective_authority_digest text not null check (effective_authority_digest ~ '^[0-9a-f]{64}$'),
  approval_required boolean not null,
  approval_receipt_id text,
  budget_classification text not null check (
    budget_classification in ('hard', 'informational', 'unknown')
  ),
  parent_handoff_id text,
  group_turn_id text,
  depth integer not null check (depth between 1 and 3),
  hard_deadline timestamptz not null,
  target_id text,
  canceled_at timestamptz,
  terminal_at timestamptz,
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, handoff_id),
  unique (account_id, idempotency_key),
  unique (account_id, target_id),
  unique (account_id, handoff_id, target_id),
  foreign key (account_id, source_turn_id)
    references fort_private.conversation_turn(account_id, turn_id),
  foreign key (account_id, source_message_id)
    references fort_private.conversation_message(account_id, message_id),
  foreign key (account_id, source_agent_id, source_behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, source_agent_id, source_binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id),
  foreign key (account_id, recipient_agent_id, recipient_behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, recipient_agent_id, recipient_binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id),
  foreign key (account_id, source_conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  foreign key (account_id, output_conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  foreign key (account_id, context_manifest_id)
    references fort_private.context_manifest(account_id, context_manifest_id),
  foreign key (account_id, delegation_grant_id)
    references fort_private.delegation_grant(account_id, delegation_grant_id),
  foreign key (account_id, approval_receipt_id)
    references fort_private.approval_receipt(account_id, approval_receipt_id),
  foreign key (account_id, emitter_receipt_id)
    references fort_private.handoff_emitter_receipt(account_id, emitter_receipt_id),
  foreign key (account_id, parent_handoff_id)
    references fort_private.handoff(account_id, handoff_id),
  foreign key (account_id, group_turn_id)
    references fort_private.conversation_turn(account_id, turn_id),
  foreign key (account_id, target_id)
    references fort_private.conversation_target(account_id, target_id)
    deferrable initially deferred,
  constraint handoff_no_self_handoff check (source_agent_id <> recipient_agent_id),
  constraint handoff_source_agent_evidence_consistent check (
    (
      source_agent_id is null and source_behavior_revision_id is null
      and source_binding_revision_id is null
    ) or (
      source_agent_id is not null and source_behavior_revision_id is not null
      and source_binding_revision_id is not null
    )
  ),
  constraint handoff_agent_emitter_required check (
    (
      creation_actor_kind = 'agent' and emitter_receipt_id is not null
      and source_agent_id is not null and creation_actor_id = source_agent_id
    ) or (creation_actor_kind <> 'agent' and emitter_receipt_id is null)
  ),
  constraint handoff_approval_consistent check (
    (approval_required and (approval_receipt_id is not null or state = 'needs_approval'))
    or (not approval_required and approval_receipt_id is null)
  ),
  constraint handoff_target_consistent check (
    (state in ('requested', 'needs_approval') and target_id is null)
    or state = 'canceled'
    or (state not in ('requested', 'needs_approval', 'canceled') and target_id is not null)
  ),
  constraint handoff_terminal_time_consistent check (
    (state in ('canceled', 'succeeded', 'failed') and terminal_at is not null)
    or (state not in ('canceled', 'succeeded', 'failed') and terminal_at is null)
  )
);

create table fort_private.handoff_attempt (
  account_id uuid not null,
  handoff_id text not null,
  execution_attempt_id text not null,
  attempt_number integer not null check (attempt_number > 0),
  recipient_agent_id text not null,
  recipient_behavior_revision_id text not null,
  recipient_binding_revision_id text not null,
  created_at timestamptz not null,
  primary key (account_id, handoff_id, attempt_number),
  unique (account_id, execution_attempt_id),
  foreign key (account_id, handoff_id)
    references fort_private.handoff(account_id, handoff_id),
  foreign key (
    account_id, execution_attempt_id, recipient_agent_id,
    recipient_behavior_revision_id, recipient_binding_revision_id
  ) references fort_private.execution_attempt(
    account_id, execution_attempt_id, agent_id,
    behavior_revision_id, binding_revision_id
  )
);

create table fort_private.handoff_projection (
  account_id uuid not null,
  handoff_id text not null,
  conversation_id text not null,
  projection_kind text not null check (projection_kind in ('source', 'recipient', 'group')),
  projected_at timestamptz not null,
  primary key (account_id, handoff_id, conversation_id),
  foreign key (account_id, handoff_id)
    references fort_private.handoff(account_id, handoff_id),
  foreign key (account_id, conversation_id)
    references fort_private.conversation(account_id, conversation_id)
);

create table fort_private.source_routine_projection (
  account_id uuid not null,
  source_routine_projection_id text not null check (btrim(source_routine_projection_id) <> ''),
  execution_source_id text not null,
  opaque_source_routine_id text not null check (btrim(opaque_source_routine_id) <> ''),
  projection_revision integer not null check (projection_revision > 0),
  authority text not null default 'source_native' check (authority = 'source_native'),
  schedule_snapshot jsonb not null check (jsonb_typeof(schedule_snapshot) = 'object'),
  projection_digest text not null check (projection_digest ~ '^[0-9a-f]{64}$'),
  last_occurrence_at timestamptz,
  next_occurrence_at timestamptz,
  observed_at timestamptz not null,
  primary key (account_id, source_routine_projection_id),
  unique (
    account_id, execution_source_id, opaque_source_routine_id, projection_revision
  ),
  foreign key (account_id, execution_source_id)
    references fort_private.execution_source(account_id, execution_source_id)
);

create table fort_private.routine (
  account_id uuid not null,
  routine_id text not null check (btrim(routine_id) <> ''),
  agent_id text not null,
  authority text not null default 'fort_cloud' check (authority = 'fort_cloud'),
  state text not null check (state in ('active', 'paused', 'paused_needs_revalidation', 'archived')),
  current_revision_id text not null,
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, routine_id),
  unique (account_id, agent_id, routine_id),
  foreign key (account_id, agent_id)
    references fort_private.stable_agent(account_id, agent_id)
);

create table fort_private.routine_revision (
  account_id uuid not null,
  routine_revision_id text not null check (btrim(routine_revision_id) <> ''),
  routine_id text not null,
  agent_id text not null,
  revision integer not null check (revision > 0),
  behavior_revision_id text not null,
  binding_revision_id text not null,
  trigger_kind text not null check (trigger_kind in ('cron', 'once', 'event')),
  schedule_expression text not null check (btrim(schedule_expression) <> ''),
  timezone text not null check (btrim(timezone) <> ''),
  next_occurrence_at timestamptz,
  input_source jsonb not null check (jsonb_typeof(input_source) = 'object'),
  freshness_policy jsonb not null check (jsonb_typeof(freshness_policy) = 'object'),
  expected_result text not null,
  result_conversation_id text not null,
  approval_policy jsonb not null check (jsonb_typeof(approval_policy) = 'object'),
  missing_input_policy text not null check (missing_input_policy in ('skip', 'needs_you', 'fail')),
  retry_policy jsonb not null check (jsonb_typeof(retry_policy) = 'object'),
  catch_up_policy jsonb not null check (jsonb_typeof(catch_up_policy) = 'object'),
  lateness_policy jsonb not null check (jsonb_typeof(lateness_policy) = 'object'),
  binding_compatibility jsonb not null check (jsonb_typeof(binding_compatibility) = 'object'),
  command_digest text not null check (command_digest ~ '^[0-9a-f]{64}$'),
  supersedes_routine_revision_id text,
  created_at timestamptz not null,
  primary key (account_id, routine_revision_id),
  unique (account_id, routine_id, revision),
  unique (account_id, routine_id, agent_id, routine_revision_id),
  foreign key (account_id, agent_id, routine_id)
    references fort_private.routine(account_id, agent_id, routine_id)
    deferrable initially deferred,
  foreign key (account_id, agent_id, behavior_revision_id)
    references fort_private.agent_behavior_revision(account_id, agent_id, behavior_revision_id),
  foreign key (account_id, agent_id, binding_revision_id)
    references fort_private.agent_binding_revision(account_id, agent_id, binding_revision_id),
  foreign key (account_id, result_conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  foreign key (account_id, routine_id, agent_id, supersedes_routine_revision_id)
    references fort_private.routine_revision(
      account_id, routine_id, agent_id, routine_revision_id
    )
);

alter table fort_private.routine
  add constraint routine_current_revision_fk
  foreign key (account_id, routine_id, agent_id, current_revision_id)
  references fort_private.routine_revision(
    account_id, routine_id, agent_id, routine_revision_id
  )
  deferrable initially deferred;

create table fort_private.routine_import_receipt (
  account_id uuid not null,
  routine_import_receipt_id text not null check (btrim(routine_import_receipt_id) <> ''),
  source_routine_projection_id text not null,
  routine_id text not null,
  routine_revision_id text not null,
  source_disabled_at timestamptz not null,
  exact_last_source_occurrence_at timestamptz,
  exact_next_source_occurrence_at timestamptz,
  fencing_receipt_ciphertext bytea not null,
  fencing_receipt_key_id text not null check (btrim(fencing_receipt_key_id) <> ''),
  fencing_receipt_nonce bytea not null check (octet_length(fencing_receipt_nonce) >= 12),
  fencing_receipt_digest text not null check (fencing_receipt_digest ~ '^[0-9a-f]{64}$'),
  imported_at timestamptz not null,
  primary key (account_id, routine_import_receipt_id),
  unique (account_id, source_routine_projection_id),
  unique (account_id, routine_id, routine_revision_id),
  foreign key (account_id, source_routine_projection_id)
    references fort_private.source_routine_projection(
      account_id, source_routine_projection_id
    ),
  foreign key (account_id, routine_revision_id)
    references fort_private.routine_revision(account_id, routine_revision_id),
  foreign key (account_id, routine_id)
    references fort_private.routine(account_id, routine_id)
);

create table fort_private.routine_occurrence (
  account_id uuid not null,
  routine_occurrence_id text not null check (btrim(routine_occurrence_id) <> ''),
  routine_id text not null,
  routine_revision_id text not null,
  scheduled_for timestamptz not null,
  is_test boolean not null default false,
  state text not null check (
    state in ('scheduled', 'queued', 'working', 'missed_needs_attention', 'succeeded', 'failed', 'canceled')
  ),
  idempotency_key text not null check (btrim(idempotency_key) <> ''),
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, routine_occurrence_id),
  unique (account_id, routine_id, scheduled_for),
  unique (account_id, routine_id, idempotency_key),
  foreign key (account_id, routine_id)
    references fort_private.routine(account_id, routine_id),
  foreign key (account_id, routine_revision_id)
    references fort_private.routine_revision(account_id, routine_revision_id)
);

create table fort_private.routine_run (
  account_id uuid not null,
  routine_run_id text not null check (btrim(routine_run_id) <> ''),
  routine_occurrence_id text not null,
  routine_id text not null,
  routine_revision_id text not null,
  behavior_revision_id text not null,
  binding_revision_id text not null,
  target_id text not null,
  execution_attempt_id text,
  result_conversation_id text not null,
  state text not null check (state in ('queued', 'working', 'needs_you', 'canceled', 'succeeded', 'failed')),
  failure_code text,
  next_action jsonb check (next_action is null or jsonb_typeof(next_action) = 'object'),
  terminal_at timestamptz,
  created_at timestamptz not null,
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, routine_run_id),
  unique (account_id, routine_occurrence_id),
  unique (account_id, target_id),
  foreign key (account_id, routine_occurrence_id)
    references fort_private.routine_occurrence(account_id, routine_occurrence_id),
  foreign key (account_id, routine_revision_id)
    references fort_private.routine_revision(account_id, routine_revision_id),
  foreign key (account_id, target_id)
    references fort_private.conversation_target(account_id, target_id),
  foreign key (account_id, execution_attempt_id)
    references fort_private.execution_attempt(account_id, execution_attempt_id),
  foreign key (account_id, result_conversation_id)
    references fort_private.conversation(account_id, conversation_id),
  constraint routine_run_terminal_time_consistent check (
    (state in ('canceled', 'succeeded', 'failed') and terminal_at is not null)
    or (state not in ('canceled', 'succeeded', 'failed') and terminal_at is null)
  )
);

create table fort_private.schedule_tick_watermark (
  account_id uuid not null,
  scheduler_id text not null check (btrim(scheduler_id) <> ''),
  last_success_at timestamptz not null,
  last_tick_id text not null check (btrim(last_tick_id) <> ''),
  updated_at timestamptz not null default clock_timestamp(),
  primary key (account_id, scheduler_id),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

create table fort_private.service_assertion_nonce (
  account_id uuid not null,
  key_id text not null check (btrim(key_id) <> ''),
  nonce text not null check (btrim(nonce) <> ''),
  expires_at timestamptz not null,
  claimed_at timestamptz not null,
  primary key (account_id, key_id, nonce),
  foreign key (account_id) references fort_private.fort_account(account_id),
  constraint service_assertion_nonce_time_order check (expires_at > claimed_at)
);

create table fort_private.idempotency_record (
  account_id uuid not null,
  scope text not null check (btrim(scope) <> ''),
  idempotency_key text not null check (btrim(idempotency_key) <> ''),
  command_digest text not null check (command_digest ~ '^[0-9a-f]{64}$'),
  result_kind text not null check (btrim(result_kind) <> ''),
  result_id text not null check (btrim(result_id) <> ''),
  response_digest text not null check (response_digest ~ '^[0-9a-f]{64}$'),
  created_at timestamptz not null,
  primary key (account_id, scope, idempotency_key),
  foreign key (account_id) references fort_private.fort_account(account_id)
);

create table fort_private.ledger_event (
  account_id uuid not null,
  event_id bigint generated always as identity,
  aggregate_kind text not null check (btrim(aggregate_kind) <> ''),
  aggregate_id text not null check (btrim(aggregate_id) <> ''),
  event_type text not null check (btrim(event_type) <> ''),
  turn_id text,
  target_id text,
  execution_attempt_id text,
  worker_id text,
  event_metadata jsonb not null default '{}'::jsonb check (jsonb_typeof(event_metadata) = 'object'),
  sensitive_ciphertext bytea,
  sensitive_key_id text,
  sensitive_nonce bytea,
  sensitive_digest text,
  created_at timestamptz not null,
  primary key (account_id, event_id),
  foreign key (account_id, turn_id)
    references fort_private.conversation_turn(account_id, turn_id),
  foreign key (account_id, target_id)
    references fort_private.conversation_target(account_id, target_id),
  foreign key (account_id, execution_attempt_id)
    references fort_private.execution_attempt(account_id, execution_attempt_id),
  foreign key (account_id, worker_id)
    references fort_private.worker(account_id, worker_id),
  constraint ledger_event_sensitive_envelope_consistent check (
    (
      sensitive_ciphertext is null and sensitive_key_id is null and
      sensitive_nonce is null and sensitive_digest is null
    ) or (
      sensitive_ciphertext is not null and btrim(sensitive_key_id) <> '' and
      octet_length(sensitive_nonce) >= 12 and sensitive_digest ~ '^[0-9a-f]{64}$'
    )
  )
);
create index ledger_event_cursor
  on fort_private.ledger_event (account_id, event_id);
create index ledger_event_aggregate
  on fort_private.ledger_event (account_id, aggregate_kind, aggregate_id, event_id);

alter table fort_private.conversation_message
  add constraint conversation_message_turn_fk
  foreign key (account_id, turn_id)
  references fort_private.conversation_turn(account_id, turn_id)
  deferrable initially deferred,
  add constraint conversation_message_target_fk
  foreign key (account_id, target_id)
  references fort_private.conversation_target(account_id, target_id)
  deferrable initially deferred,
  add constraint conversation_message_handoff_fk
  foreign key (account_id, handoff_id)
  references fort_private.handoff(account_id, handoff_id)
  deferrable initially deferred,
  add constraint conversation_message_routine_run_fk
  foreign key (account_id, routine_run_id)
  references fort_private.routine_run(account_id, routine_run_id)
  deferrable initially deferred;

create function fort_private.reject_immutable_mutation()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  raise exception using
    errcode = '23514',
    message = 'fort_immutable:' || tg_table_name;
end
$function$;

do $immutable_triggers$
declare
  relation_name text;
begin
  foreach relation_name in array array[
    'worker_capability_revision',
    'execution_source',
    'execution_source_config_observation',
    'source_agent',
    'agent_profile_revision',
    'agent_behavior_revision',
    'agent_binding_revision',
    'agent_binding_transition',
    'agent_conversation',
    'agent_conversation_pin',
    'group_conversation',
    'conversation_membership_revision',
    'conversation_member_revision',
    'conversation_participant',
    'delegation_grant',
    'artifact_chunk',
    'context_manifest',
    'context_manifest_message',
    'context_manifest_artifact',
    'conversation_message',
    'conversation_target_binding',
    'approval_receipt',
    'handoff_emitter_receipt',
    'handoff_attempt',
    'handoff_projection',
    'source_routine_projection',
    'routine_revision',
    'routine_import_receipt',
    'service_assertion_nonce',
    'worker_cancellation_ack',
    'idempotency_record',
    'ledger_event'
  ]
  loop
    execute format(
      'create trigger %I before update or delete on fort_private.%I '
      'for each row execute function fort_private.reject_immutable_mutation()',
      relation_name || '_immutable',
      relation_name
    );
  end loop;
end
$immutable_triggers$;

create function fort_private.protect_stable_agent_identity()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.account_id is distinct from old.account_id
     or new.agent_id is distinct from old.agent_id
     or new.canonical_conversation_id is distinct from old.canonical_conversation_id
     or new.created_at is distinct from old.created_at then
    raise exception using errcode = '23514', message = 'stable_agent_identity_immutable';
  end if;
  return new;
end
$function$;
create trigger stable_agent_identity_immutable
before update on fort_private.stable_agent
for each row execute function fort_private.protect_stable_agent_identity();
create trigger stable_agent_delete_immutable
before delete on fort_private.stable_agent
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_conversation_identity()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.account_id is distinct from old.account_id
     or new.conversation_id is distinct from old.conversation_id
     or new.kind is distinct from old.kind
     or new.created_at is distinct from old.created_at then
    raise exception using errcode = '23514', message = 'conversation_identity_immutable';
  end if;
  return new;
end
$function$;
create trigger conversation_identity_immutable
before update on fort_private.conversation
for each row execute function fort_private.protect_conversation_identity();
create trigger conversation_delete_immutable
before delete on fort_private.conversation
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_turn_snapshot()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - 'state' - 'updated_at')
     is distinct from (to_jsonb(old) - 'state' - 'updated_at') then
    raise exception using errcode = '23514', message = 'conversation_turn_snapshot_immutable';
  end if;
  return new;
end
$function$;
create trigger conversation_turn_snapshot_immutable
before update on fort_private.conversation_turn
for each row execute function fort_private.protect_turn_snapshot();
create trigger conversation_turn_delete_immutable
before delete on fort_private.conversation_turn
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_target_binding()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - array[
        'state', 'attempt_count', 'error_code', 'error_ciphertext',
        'error_key_id', 'error_nonce', 'error_digest', 'updated_at'
      ]::text[])
     is distinct from
     (to_jsonb(old) - array[
        'state', 'attempt_count', 'error_code', 'error_ciphertext',
        'error_key_id', 'error_nonce', 'error_digest', 'updated_at'
      ]::text[]) then
    raise exception using errcode = '23514', message = 'conversation_target_binding_immutable';
  end if;
  return new;
end
$function$;
create trigger conversation_target_binding_immutable
before update on fort_private.conversation_target
for each row execute function fort_private.protect_target_binding();
create trigger conversation_target_delete_immutable
before delete on fort_private.conversation_target
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_attempt_binding()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.account_id is distinct from old.account_id
     or new.execution_attempt_id is distinct from old.execution_attempt_id
     or new.target_id is distinct from old.target_id
     or new.attempt_number is distinct from old.attempt_number
     or new.agent_id is distinct from old.agent_id
     or new.behavior_revision_id is distinct from old.behavior_revision_id
     or new.binding_revision_id is distinct from old.binding_revision_id
     or new.participant_id is distinct from old.participant_id
     or new.created_at is distinct from old.created_at then
    raise exception using errcode = '23514', message = 'execution_attempt_binding_immutable';
  end if;
  if old.worker_id is not null and (
    new.worker_id is distinct from old.worker_id
    or new.worker_capability_revision_id is distinct from old.worker_capability_revision_id
  ) then
    raise exception using errcode = '23514', message = 'execution_attempt_worker_immutable';
  end if;
  if old.terminal_receipt_id is not null and new is distinct from old then
    raise exception using errcode = '23514', message = 'execution_attempt_terminal_immutable';
  end if;
  return new;
end
$function$;
create trigger execution_attempt_binding_immutable
before update on fort_private.execution_attempt
for each row execute function fort_private.protect_attempt_binding();
create trigger execution_attempt_delete_immutable
before delete on fort_private.execution_attempt
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_worker_identity()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.account_id is distinct from old.account_id
     or new.worker_id is distinct from old.worker_id
     or new.machine_id is distinct from old.machine_id
     or new.identity_key_digest is distinct from old.identity_key_digest
     or new.enrollment_token_hash is distinct from old.enrollment_token_hash
     or new.enrolled_at is distinct from old.enrolled_at then
    raise exception using errcode = '23514', message = 'worker_identity_immutable';
  end if;
  if old.state = 'revoked' and new is distinct from old then
    raise exception using errcode = '23514', message = 'worker_revocation_terminal';
  end if;
  return new;
end
$function$;
create trigger worker_identity_immutable
before update on fort_private.worker
for each row execute function fort_private.protect_worker_identity();
create trigger worker_delete_immutable
before delete on fort_private.worker
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_worker_lease()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - array[
        'state', 'heartbeat_at', 'expires_at', 'released_at', 'updated_at'
      ]::text[])
     is distinct from
     (to_jsonb(old) - array[
        'state', 'heartbeat_at', 'expires_at', 'released_at', 'updated_at'
      ]::text[]) then
    raise exception using errcode = '23514', message = 'worker_lease_identity_immutable';
  end if;
  if old.state <> 'active' and new is distinct from old then
    raise exception using errcode = '23514', message = 'worker_lease_terminal';
  end if;
  if new.heartbeat_at < old.heartbeat_at or new.expires_at < old.expires_at then
    raise exception using errcode = '23514', message = 'worker_lease_clock_regression';
  end if;
  return new;
end
$function$;
create trigger worker_lease_invariants
before update on fort_private.worker_lease
for each row execute function fort_private.protect_worker_lease();
create trigger worker_lease_delete_immutable
before delete on fort_private.worker_lease
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_worker_command()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - array[
        'state', 'claimed_at', 'completed_at', 'updated_at'
      ]::text[])
     is distinct from
     (to_jsonb(old) - array[
        'state', 'claimed_at', 'completed_at', 'updated_at'
      ]::text[]) then
    raise exception using errcode = '23514', message = 'worker_command_identity_immutable';
  end if;
  return new;
end
$function$;
create trigger worker_command_identity_immutable
before update on fort_private.worker_command
for each row execute function fort_private.protect_worker_command();
create trigger worker_command_delete_immutable
before delete on fort_private.worker_command
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.validate_agent_canonical_conversation()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  checked_account uuid;
  checked_agent text;
  canonical_id text;
  agent_state text;
begin
  if tg_op = 'DELETE' then
    checked_account := old.account_id;
    checked_agent := old.agent_id;
  else
    checked_account := new.account_id;
    checked_agent := new.agent_id;
  end if;

  select agent.canonical_conversation_id, agent.state
    into canonical_id, agent_state
    from fort_private.stable_agent as agent
   where agent.account_id = checked_account and agent.agent_id = checked_agent;

  if not found then
    if tg_op = 'DELETE' then return old; else return new; end if;
  end if;

  if not exists (
    select 1
      from fort_private.agent_conversation as relation
      join fort_private.conversation as conversation
        on conversation.account_id = relation.account_id
       and conversation.conversation_id = relation.conversation_id
     where relation.account_id = checked_account
       and relation.agent_id = checked_agent
       and relation.conversation_id = canonical_id
       and relation.kind = 'canonical'
       and conversation.kind = 'agent'
       and (agent_state <> 'open' or conversation.state = 'open')
  ) then
    raise exception using errcode = '23514', message = 'stable_agent_canonical_invariant';
  end if;

  if tg_op = 'DELETE' then return old; else return new; end if;
end
$function$;

create constraint trigger stable_agent_canonical_check
after insert or update on fort_private.stable_agent
deferrable initially deferred
for each row execute function fort_private.validate_agent_canonical_conversation();
create constraint trigger agent_conversation_canonical_check
after insert or update or delete on fort_private.agent_conversation
deferrable initially deferred
for each row execute function fort_private.validate_agent_canonical_conversation();

create function fort_private.validate_membership_revision()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  checked_account uuid;
  checked_revision text;
  checked_conversation text;
  conversation_kind text;
  member_count integer;
begin
  if tg_op = 'DELETE' then
    checked_account := old.account_id;
  else
    checked_account := new.account_id;
  end if;

  if tg_table_name = 'conversation' then
    if tg_op = 'DELETE' then
      checked_revision := old.current_membership_revision_id;
    else
      checked_revision := new.current_membership_revision_id;
    end if;
  else
    if tg_op = 'DELETE' then
      checked_revision := old.membership_revision_id;
    else
      checked_revision := new.membership_revision_id;
    end if;
  end if;

  select revision.conversation_id, conversation.kind
    into checked_conversation, conversation_kind
    from fort_private.conversation_membership_revision as revision
    join fort_private.conversation as conversation
      on conversation.account_id = revision.account_id
     and conversation.conversation_id = revision.conversation_id
   where revision.account_id = checked_account
     and revision.membership_revision_id = checked_revision;

  if not found then
    if tg_op = 'DELETE' then return old; else return new; end if;
  end if;

  select count(*)::integer
    into member_count
    from fort_private.conversation_member_revision as member
   where member.account_id = checked_account
     and member.membership_revision_id = checked_revision;

  if conversation_kind = 'group' and member_count not between 2 and 6 then
    raise exception using errcode = '23514', message = 'group_membership_size_invariant';
  elsif conversation_kind = 'agent' then
    if member_count <> 1 or not exists (
      select 1
        from fort_private.conversation_member_revision as member
        join fort_private.agent_conversation as relation
          on relation.account_id = member.account_id
         and relation.agent_id = member.agent_id
         and relation.conversation_id = member.conversation_id
       where member.account_id = checked_account
         and member.membership_revision_id = checked_revision
         and member.conversation_id = checked_conversation
    ) then
      raise exception using errcode = '23514', message = 'agent_conversation_membership_invariant';
    end if;
  elsif conversation_kind = 'legacy' and member_count > 6 then
    raise exception using errcode = '23514', message = 'legacy_membership_size_invariant';
  end if;

  if tg_op = 'DELETE' then return old; else return new; end if;
end
$function$;

create constraint trigger membership_revision_shape_check
after insert or update or delete on fort_private.conversation_membership_revision
deferrable initially deferred
for each row execute function fort_private.validate_membership_revision();
create constraint trigger membership_member_shape_check
after insert or update or delete on fort_private.conversation_member_revision
deferrable initially deferred
for each row execute function fort_private.validate_membership_revision();
create constraint trigger conversation_current_membership_shape_check
after insert or update on fort_private.conversation
deferrable initially deferred
for each row execute function fort_private.validate_membership_revision();

create function fort_private.reject_active_group_membership_change()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.current_membership_revision_id is distinct from old.current_membership_revision_id
     and old.kind = 'group'
     and exists (
       select 1
         from fort_private.conversation_turn as turn
        where turn.account_id = old.account_id
          and turn.conversation_id = old.conversation_id
          and turn.state in ('open', 'needs_you')
     ) then
    raise exception using errcode = '23514', message = 'group_turn_membership_frozen';
  end if;
  return new;
end
$function$;
create trigger conversation_group_membership_frozen
before update of current_membership_revision_id on fort_private.conversation
for each row execute function fort_private.reject_active_group_membership_change();

create function fort_private.validate_artifact_transition()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  actual_count integer;
  first_index integer;
  last_index integer;
  total_plaintext bigint;
  total_encoded bigint;
  mismatched_keys integer;
begin
  if old.state in ('finalized', 'failed') and new is distinct from old then
    raise exception using errcode = '23514', message = 'artifact_terminal';
  end if;

  if (to_jsonb(new) - 'state' - 'finalized_at')
     is distinct from (to_jsonb(old) - 'state' - 'finalized_at') then
    raise exception using errcode = '23514', message = 'artifact_manifest_immutable';
  end if;

  if new.state = 'finalized' and old.state <> 'finalized' then
    select count(*)::integer,
           min(chunk_index),
           max(chunk_index),
           coalesce(sum(plaintext_length), 0),
           coalesce(sum(encoded_length), 0),
           count(*) filter (where encryption_key_id <> new.encryption_key_id)::integer
      into actual_count, first_index, last_index, total_plaintext, total_encoded, mismatched_keys
      from fort_private.artifact_chunk
     where account_id = new.account_id and artifact_id = new.artifact_id;

    if actual_count <> new.expected_chunk_count
       or first_index <> 0
       or last_index <> new.expected_chunk_count - 1
       or total_plaintext <> new.expected_plaintext_length
       or total_encoded <> new.expected_encoded_length
       or mismatched_keys <> 0 then
      raise exception using errcode = '23514', message = 'artifact_incomplete';
    end if;
  end if;

  return new;
end
$function$;
create trigger artifact_transition_invariant
before update on fort_private.artifact
for each row execute function fort_private.validate_artifact_transition();
create trigger artifact_delete_immutable
before delete on fort_private.artifact
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.validate_artifact_chunk_insert()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  manifest_state text;
  manifest_chunk_count integer;
  manifest_key_id text;
begin
  select state, expected_chunk_count, encryption_key_id
    into manifest_state, manifest_chunk_count, manifest_key_id
    from fort_private.artifact
   where account_id = new.account_id and artifact_id = new.artifact_id;

  if not found then
    raise exception using errcode = '23503', message = 'artifact_not_found';
  end if;
  if manifest_state <> 'uploading' then
    raise exception using errcode = '23514', message = 'artifact_terminal';
  end if;
  if new.chunk_index >= manifest_chunk_count then
    raise exception using errcode = '23514', message = 'artifact_chunk_out_of_range';
  end if;
  if new.encryption_key_id <> manifest_key_id then
    raise exception using errcode = '23514', message = 'artifact_chunk_key_mismatch';
  end if;

  return new;
end
$function$;
create trigger artifact_chunk_insert_invariant
before insert on fort_private.artifact_chunk
for each row execute function fort_private.validate_artifact_chunk_insert();
create trigger artifact_chunk_update_immutable
before update on fort_private.artifact_chunk
for each row execute function fort_private.reject_immutable_mutation();
create trigger artifact_chunk_delete_immutable
before delete on fort_private.artifact_chunk
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.require_finalized_manifest_artifact()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if not exists (
    select 1
      from fort_private.artifact
     where account_id = new.account_id
       and artifact_id = new.artifact_id
       and state = 'finalized'
  ) then
    raise exception using errcode = '23514', message = 'context_artifact_not_finalized';
  end if;
  return new;
end
$function$;
create trigger context_manifest_artifact_finalized
before insert on fort_private.context_manifest_artifact
for each row execute function fort_private.require_finalized_manifest_artifact();

create function fort_private.validate_handoff_chain()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  parent_row fort_private.handoff%rowtype;
  grant_depth integer;
  grant_deadline timestamptz;
begin
  select maximum_handoff_depth, hard_deadline
    into grant_depth, grant_deadline
    from fort_private.delegation_grant
   where account_id = new.account_id
     and delegation_grant_id = new.delegation_grant_id;

  if new.depth > grant_depth or new.hard_deadline > grant_deadline then
    raise exception using errcode = '23514', message = 'handoff_delegation_bound_exceeded';
  end if;

  if new.parent_handoff_id is null then
    if new.depth <> 1 then
      raise exception using errcode = '23514', message = 'handoff_root_depth_invalid';
    end if;
  else
    select * into strict parent_row
      from fort_private.handoff
     where account_id = new.account_id
       and handoff_id = new.parent_handoff_id;

    if new.depth <> parent_row.depth + 1
       or new.delegation_grant_id <> parent_row.delegation_grant_id
       or new.source_agent_id <> parent_row.recipient_agent_id
       or new.group_turn_id is distinct from parent_row.group_turn_id
       or new.hard_deadline > parent_row.hard_deadline then
      raise exception using errcode = '23514', message = 'handoff_parent_invariant';
    end if;

    if exists (
      with recursive ancestors as (
        select parent.handoff_id, parent.parent_handoff_id,
               parent.source_agent_id, parent.recipient_agent_id
          from fort_private.handoff as parent
         where parent.account_id = new.account_id
           and parent.handoff_id = new.parent_handoff_id
        union all
        select parent.handoff_id, parent.parent_handoff_id,
               parent.source_agent_id, parent.recipient_agent_id
          from fort_private.handoff as parent
          join ancestors on ancestors.parent_handoff_id = parent.handoff_id
         where parent.account_id = new.account_id
      )
      select 1 from ancestors
       where source_agent_id = new.recipient_agent_id
          or recipient_agent_id = new.recipient_agent_id
    ) then
      raise exception using errcode = '23514', message = 'handoff_cycle';
    end if;
  end if;

  return new;
end
$function$;
create trigger handoff_chain_invariant
before insert on fort_private.handoff
for each row execute function fort_private.validate_handoff_chain();

create function fort_private.protect_handoff_identity()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - array[
        'state', 'approval_receipt_id', 'effective_authority',
        'effective_authority_digest', 'target_id', 'canceled_at',
        'terminal_at', 'updated_at'
      ]::text[])
     is distinct from
     (to_jsonb(old) - array[
        'state', 'approval_receipt_id', 'effective_authority',
        'effective_authority_digest', 'target_id', 'canceled_at',
        'terminal_at', 'updated_at'
      ]::text[]) then
    raise exception using errcode = '23514', message = 'handoff_identity_immutable';
  end if;
  if old.approval_receipt_id is not null
     and new.approval_receipt_id is distinct from old.approval_receipt_id then
    raise exception using errcode = '23514', message = 'handoff_approval_immutable';
  end if;
  if old.target_id is not null and new.target_id is distinct from old.target_id then
    raise exception using errcode = '23514', message = 'handoff_target_immutable';
  end if;
  if old.state in ('canceled', 'succeeded', 'failed') and new is distinct from old then
    raise exception using errcode = '23514', message = 'handoff_terminal';
  end if;
  return new;
end
$function$;
create trigger handoff_identity_immutable
before update on fort_private.handoff
for each row execute function fort_private.protect_handoff_identity();
create trigger handoff_delete_immutable
before delete on fort_private.handoff
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.validate_handoff_target()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.approval_required and new.target_id is not null and not exists (
    select 1
      from fort_private.approval_receipt as receipt
     where receipt.account_id = new.account_id
       and receipt.approval_receipt_id = new.approval_receipt_id
       and receipt.subject_kind = 'handoff'
       and receipt.subject_id = new.handoff_id
       and receipt.decision = 'approved'
  ) then
    raise exception using errcode = '23514', message = 'handoff_approval_not_effective';
  end if;

  if new.target_id is not null and not exists (
    select 1
      from fort_private.conversation_target as target
      join fort_private.conversation_target_binding as binding
        on binding.account_id = target.account_id
       and binding.target_id = target.target_id
     where target.account_id = new.account_id
       and target.target_id = new.target_id
       and target.target_kind = 'handoff'
       and target.origin_id = new.handoff_id
       and target.agent_id = new.recipient_agent_id
       and binding.behavior_revision_id = new.recipient_behavior_revision_id
       and binding.binding_revision_id = new.recipient_binding_revision_id
  ) then
    raise exception using errcode = '23514', message = 'handoff_target_mismatch';
  end if;
  return new;
end
$function$;
create constraint trigger handoff_target_check
after insert or update on fort_private.handoff
deferrable initially deferred
for each row execute function fort_private.validate_handoff_target();

create function fort_private.validate_handoff_result()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  checked_account uuid;
  checked_handoff text;
  handoff_state text;
  output_conversation text;
  result_count integer;
begin
  if tg_op = 'DELETE' then
    checked_account := old.account_id;
    checked_handoff := old.handoff_id;
  else
    checked_account := new.account_id;
    checked_handoff := new.handoff_id;
  end if;

  if checked_handoff is null then
    if tg_op = 'DELETE' then return old; else return new; end if;
  end if;

  select state, output_conversation_id
    into handoff_state, output_conversation
    from fort_private.handoff
   where account_id = checked_account and handoff_id = checked_handoff;
  if not found then
    if tg_op = 'DELETE' then return old; else return new; end if;
  end if;

  select count(*)::integer into result_count
    from fort_private.conversation_message
   where account_id = checked_account
     and handoff_id = checked_handoff
     and message_kind = 'handoff_result'
     and conversation_id = output_conversation;

  if handoff_state = 'succeeded' and result_count <> 1 then
    raise exception using errcode = '23514', message = 'handoff_result_missing';
  elsif handoff_state <> 'succeeded' and result_count <> 0 then
    raise exception using errcode = '23514', message = 'handoff_result_before_success';
  end if;

  if tg_op = 'DELETE' then return old; else return new; end if;
end
$function$;
create constraint trigger handoff_result_from_handoff_check
after insert or update on fort_private.handoff
deferrable initially deferred
for each row execute function fort_private.validate_handoff_result();
create constraint trigger handoff_result_from_message_check
after insert or update or delete on fort_private.conversation_message
deferrable initially deferred
for each row
execute function fort_private.validate_handoff_result();

create function fort_private.protect_routine_identity()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - 'state' - 'current_revision_id' - 'updated_at')
     is distinct from (to_jsonb(old) - 'state' - 'current_revision_id' - 'updated_at') then
    raise exception using errcode = '23514', message = 'routine_identity_immutable';
  end if;
  return new;
end
$function$;
create trigger routine_identity_immutable
before update on fort_private.routine
for each row execute function fort_private.protect_routine_identity();
create trigger routine_delete_immutable
before delete on fort_private.routine
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.pause_routines_for_agent_revision()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.current_behavior_revision_id is distinct from old.current_behavior_revision_id
     or new.current_binding_revision_id is distinct from old.current_binding_revision_id then
    update fort_private.routine
       set state = 'paused_needs_revalidation', updated_at = clock_timestamp()
     where account_id = new.account_id
       and agent_id = new.agent_id
       and state = 'active';
  end if;
  return new;
end
$function$;
create trigger stable_agent_revision_pauses_routines
after update of current_behavior_revision_id, current_binding_revision_id
on fort_private.stable_agent
for each row execute function fort_private.pause_routines_for_agent_revision();

create function fort_private.protect_routine_occurrence()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - 'state' - 'updated_at')
     is distinct from (to_jsonb(old) - 'state' - 'updated_at') then
    raise exception using errcode = '23514', message = 'routine_occurrence_identity_immutable';
  end if;
  return new;
end
$function$;
create trigger routine_occurrence_identity_immutable
before update on fort_private.routine_occurrence
for each row execute function fort_private.protect_routine_occurrence();
create trigger routine_occurrence_delete_immutable
before delete on fort_private.routine_occurrence
for each row execute function fort_private.reject_immutable_mutation();

create function fort_private.protect_routine_run()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if (to_jsonb(new) - array[
        'state', 'execution_attempt_id', 'failure_code', 'next_action',
        'terminal_at', 'updated_at'
      ]::text[])
     is distinct from
     (to_jsonb(old) - array[
        'state', 'execution_attempt_id', 'failure_code', 'next_action',
        'terminal_at', 'updated_at'
      ]::text[]) then
    raise exception using errcode = '23514', message = 'routine_run_identity_immutable';
  end if;
  if old.execution_attempt_id is not null
     and new.execution_attempt_id is distinct from old.execution_attempt_id then
    raise exception using errcode = '23514', message = 'routine_run_attempt_immutable';
  end if;
  if old.state in ('canceled', 'succeeded', 'failed') and new is distinct from old then
    raise exception using errcode = '23514', message = 'routine_run_terminal';
  end if;
  return new;
end
$function$;
create trigger routine_run_identity_immutable
before update on fort_private.routine_run
for each row execute function fort_private.protect_routine_run();
create trigger routine_run_delete_immutable
before delete on fort_private.routine_run
for each row execute function fort_private.reject_immutable_mutation();

alter table fort_private.routine_revision
  add constraint routine_revision_execution_snapshot_unique
  unique (
    account_id, routine_id, routine_revision_id,
    behavior_revision_id, binding_revision_id, result_conversation_id
  );
alter table fort_private.routine_run
  add constraint routine_run_revision_snapshot_fk
  foreign key (
    account_id, routine_id, routine_revision_id,
    behavior_revision_id, binding_revision_id, result_conversation_id
  ) references fort_private.routine_revision(
    account_id, routine_id, routine_revision_id,
    behavior_revision_id, binding_revision_id, result_conversation_id
  );

create function fort_private.validate_routine_result()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  checked_account uuid;
  checked_run text;
  run_state text;
  output_conversation text;
  result_count integer;
begin
  if tg_op = 'DELETE' then
    checked_account := old.account_id;
    checked_run := old.routine_run_id;
  else
    checked_account := new.account_id;
    checked_run := new.routine_run_id;
  end if;

  if checked_run is null then
    if tg_op = 'DELETE' then return old; else return new; end if;
  end if;

  select state, result_conversation_id
    into run_state, output_conversation
    from fort_private.routine_run
   where account_id = checked_account and routine_run_id = checked_run;
  if not found then
    if tg_op = 'DELETE' then return old; else return new; end if;
  end if;

  select count(*)::integer into result_count
    from fort_private.conversation_message
   where account_id = checked_account
     and routine_run_id = checked_run
     and message_kind = 'routine_result'
     and conversation_id = output_conversation;

  if run_state = 'succeeded' and result_count <> 1 then
    raise exception using errcode = '23514', message = 'routine_result_missing';
  elsif run_state <> 'succeeded' and result_count <> 0 then
    raise exception using errcode = '23514', message = 'routine_result_before_success';
  end if;

  if tg_op = 'DELETE' then return old; else return new; end if;
end
$function$;
create constraint trigger routine_result_from_run_check
after insert or update on fort_private.routine_run
deferrable initially deferred
for each row execute function fort_private.validate_routine_result();
create constraint trigger routine_result_from_message_check
after insert or update or delete on fort_private.conversation_message
deferrable initially deferred
for each row
execute function fort_private.validate_routine_result();

create function fort_private.validate_routine_target()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if not exists (
    select 1
      from fort_private.conversation_target as target
      join fort_private.conversation_target_binding as binding
        on binding.account_id = target.account_id
       and binding.target_id = target.target_id
     where target.account_id = new.account_id
       and target.target_id = new.target_id
       and target.target_kind = 'routine'
       and target.origin_id = new.routine_occurrence_id
       and binding.behavior_revision_id = new.behavior_revision_id
       and binding.binding_revision_id = new.binding_revision_id
  ) then
    raise exception using errcode = '23514', message = 'routine_target_mismatch';
  end if;
  return new;
end
$function$;
create constraint trigger routine_target_check
after insert or update on fort_private.routine_run
deferrable initially deferred
for each row execute function fort_private.validate_routine_target();

create function fort_private.protect_schedule_watermark()
returns trigger
language plpgsql
set search_path = ''
as $function$
begin
  if new.account_id is distinct from old.account_id
     or new.scheduler_id is distinct from old.scheduler_id then
    raise exception using errcode = '23514', message = 'schedule_watermark_identity_immutable';
  end if;
  if new.last_success_at <= old.last_success_at then
    raise exception using errcode = '23514', message = 'schedule_watermark_not_monotonic';
  end if;
  return new;
end
$function$;
create trigger schedule_watermark_monotonic
before update on fort_private.schedule_tick_watermark
for each row execute function fort_private.protect_schedule_watermark();
create trigger schedule_watermark_delete_immutable
before delete on fort_private.schedule_tick_watermark
for each row execute function fort_private.reject_immutable_mutation();

-- No table in fort_private is directly exposed. RLS is still enabled and
-- forced as a second boundary around the least-privilege runtime role. A
-- missing, blank, or malformed transaction-local account setting returns no
-- rows (or errors on malformed UUID) rather than falling back to another
-- account.
do $rls$
declare
  relation_name text;
begin
  for relation_name in
    select tablename from pg_catalog.pg_tables where schemaname = 'fort_private'
  loop
    execute format('alter table fort_private.%I enable row level security', relation_name);
    execute format('alter table fort_private.%I force row level security', relation_name);
    execute format(
      'create policy account_isolation on fort_private.%I '
      'for all to fort_gateway '
      'using (account_id = nullif(current_setting(''fort.account_id'', true), '''')::uuid) '
      'with check (account_id = nullif(current_setting(''fort.account_id'', true), '''')::uuid)',
      relation_name
    );
  end loop;
end
$rls$;

grant usage on schema fort_private to fort_gateway;
grant select on all tables in schema fort_private to fort_gateway;
grant insert on table
  fort_private.worker,
  fort_private.worker_capability_revision,
  fort_private.execution_source,
  fort_private.execution_source_config_observation,
  fort_private.source_agent,
  fort_private.stable_agent,
  fort_private.agent_profile_revision,
  fort_private.agent_behavior_revision,
  fort_private.agent_binding_revision,
  fort_private.agent_binding_transition,
  fort_private.conversation,
  fort_private.group_conversation,
  fort_private.agent_conversation,
  fort_private.agent_conversation_pin,
  fort_private.conversation_membership_revision,
  fort_private.conversation_member_revision,
  fort_private.conversation_participant,
  fort_private.delegation_grant,
  fort_private.artifact,
  fort_private.artifact_chunk,
  fort_private.context_manifest,
  fort_private.context_manifest_message,
  fort_private.context_manifest_artifact,
  fort_private.conversation_message,
  fort_private.conversation_turn,
  fort_private.conversation_target,
  fort_private.conversation_target_binding,
  fort_private.execution_attempt,
  fort_private.worker_lease,
  fort_private.worker_cancellation_ack,
  fort_private.worker_command,
  fort_private.approval_receipt,
  fort_private.handoff_emitter_receipt,
  fort_private.handoff,
  fort_private.handoff_attempt,
  fort_private.handoff_projection,
  fort_private.source_routine_projection,
  fort_private.routine,
  fort_private.routine_revision,
  fort_private.routine_import_receipt,
  fort_private.routine_occurrence,
  fort_private.routine_run,
  fort_private.schedule_tick_watermark,
  fort_private.service_assertion_nonce,
  fort_private.idempotency_record,
  fort_private.ledger_event
to fort_gateway;
grant update on table
  fort_private.worker,
  fort_private.stable_agent,
  fort_private.conversation,
  fort_private.artifact,
  fort_private.conversation_turn,
  fort_private.conversation_target,
  fort_private.execution_attempt,
  fort_private.worker_lease,
  fort_private.worker_command,
  fort_private.handoff,
  fort_private.routine,
  fort_private.routine_occurrence,
  fort_private.routine_run,
  fort_private.schedule_tick_watermark
to fort_gateway;
grant usage, select on all sequences in schema fort_private to fort_gateway;

revoke execute on all functions in schema fort_private
  from public, anon, authenticated, service_role, fort_gateway;
revoke all on schema fort_private from public, anon, authenticated, service_role;
revoke all privileges on all tables in schema fort_private
  from public, anon, authenticated, service_role;
revoke all privileges on all sequences in schema fort_private
  from public, anon, authenticated, service_role;
