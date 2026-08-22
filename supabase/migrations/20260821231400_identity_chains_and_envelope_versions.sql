-- Bind every execution Revision pair and causal chain in Postgres rather than
-- relying on application preflight checks. All new columns are backfilled by
-- constant defaults except source_execution_attempt_id, which is recovered
-- from the already-authoritative emitter receipt before its exact FK is added.

set lock_timeout = '5s';

alter table fort_private.agent_binding_revision
  add constraint agent_binding_behavior_pair_unique unique (
    account_id, agent_id, behavior_revision_id, binding_revision_id
  );

alter table fort_private.stable_agent
  add constraint stable_agent_current_behavior_binding_fk
  foreign key (
    account_id, agent_id, current_behavior_revision_id,
    current_binding_revision_id
  ) references fort_private.agent_binding_revision(
    account_id, agent_id, behavior_revision_id, binding_revision_id
  ) deferrable initially deferred;

alter table fort_private.conversation_participant
  add constraint conversation_participant_behavior_binding_fk
  foreign key (
    account_id, agent_id, behavior_revision_id, binding_revision_id
  ) references fort_private.agent_binding_revision(
    account_id, agent_id, behavior_revision_id, binding_revision_id
  );

alter table fort_private.handoff
  add column source_execution_attempt_id text;

update fort_private.handoff as handoff
   set source_execution_attempt_id = receipt.source_execution_attempt_id
  from fort_private.handoff_emitter_receipt as receipt
 where handoff.account_id = receipt.account_id
   and handoff.emitter_receipt_id = receipt.emitter_receipt_id
   and handoff.creation_actor_kind = 'agent';

alter table fort_private.handoff_emitter_receipt
  add constraint handoff_emitter_exact_source_command_unique unique (
    account_id, emitter_receipt_id, source_execution_attempt_id,
    source_agent_id, source_behavior_revision_id,
    source_binding_revision_id, structured_command_digest
  );

alter table fort_private.handoff
  add constraint handoff_source_behavior_binding_fk
  foreign key (
    account_id, source_agent_id, source_behavior_revision_id,
    source_binding_revision_id
  ) references fort_private.agent_binding_revision(
    account_id, agent_id, behavior_revision_id, binding_revision_id
  ),
  add constraint handoff_recipient_behavior_binding_fk
  foreign key (
    account_id, recipient_agent_id, recipient_behavior_revision_id,
    recipient_binding_revision_id
  ) references fort_private.agent_binding_revision(
    account_id, agent_id, behavior_revision_id, binding_revision_id
  ),
  add constraint handoff_emitter_exact_source_command_fk
  foreign key (
    account_id, emitter_receipt_id, source_execution_attempt_id,
    source_agent_id, source_behavior_revision_id,
    source_binding_revision_id, command_digest
  ) references fort_private.handoff_emitter_receipt(
    account_id, emitter_receipt_id, source_execution_attempt_id,
    source_agent_id, source_behavior_revision_id,
    source_binding_revision_id, structured_command_digest
  ),
  add constraint handoff_agent_emitter_exact_source check (
    (
      creation_actor_kind = 'agent'
      and emitter_receipt_id is not null
      and source_execution_attempt_id is not null
    ) or (
      creation_actor_kind <> 'agent'
      and emitter_receipt_id is null
      and source_execution_attempt_id is null
    )
  );

alter table fort_private.conversation_message
  add constraint conversation_message_turn_identity_unique unique (
    account_id, turn_id, message_id
  ),
  add constraint conversation_message_handoff_identity_unique unique (
    account_id, handoff_id, message_id
  );

alter table fort_private.handoff
  add constraint handoff_recipient_identity_unique unique (
    account_id, handoff_id, recipient_agent_id,
    recipient_behavior_revision_id, recipient_binding_revision_id
  );

alter table fort_private.handoff
  add constraint handoff_source_conversation_message_fk
  foreign key (account_id, source_conversation_id, source_message_id)
  references fort_private.conversation_message(
    account_id, conversation_id, message_id
  ),
  add constraint handoff_source_turn_message_fk
  foreign key (account_id, source_turn_id, source_message_id)
  references fort_private.conversation_message(account_id, turn_id, message_id),
  add constraint handoff_parent_source_binding_fk
  foreign key (
    account_id, parent_handoff_id, source_agent_id,
    source_behavior_revision_id, source_binding_revision_id
  ) references fort_private.handoff(
    account_id, handoff_id, recipient_agent_id,
    recipient_behavior_revision_id, recipient_binding_revision_id
  ),
  add constraint handoff_parent_result_message_fk
  foreign key (account_id, parent_handoff_id, source_message_id)
  references fort_private.conversation_message(account_id, handoff_id, message_id);

alter table fort_private.routine_revision
  add constraint routine_revision_identity_unique unique (
    account_id, routine_id, routine_revision_id
  ),
  add constraint routine_revision_execution_chain_unique unique (
    account_id, routine_id, routine_revision_id,
    behavior_revision_id, binding_revision_id, result_conversation_id
  ),
  add constraint routine_revision_behavior_binding_fk
  foreign key (
    account_id, agent_id, behavior_revision_id, binding_revision_id
  ) references fort_private.agent_binding_revision(
    account_id, agent_id, behavior_revision_id, binding_revision_id
  );

alter table fort_private.routine_import_receipt
  add constraint routine_import_receipt_revision_fk
  foreign key (account_id, routine_id, routine_revision_id)
  references fort_private.routine_revision(
    account_id, routine_id, routine_revision_id
  );

alter table fort_private.routine_occurrence
  add constraint routine_occurrence_identity_unique unique (
    account_id, routine_occurrence_id, routine_id, routine_revision_id
  ),
  add constraint routine_occurrence_revision_fk
  foreign key (account_id, routine_id, routine_revision_id)
  references fort_private.routine_revision(
    account_id, routine_id, routine_revision_id
  );

alter table fort_private.conversation_target
  add constraint conversation_target_routine_chain_unique unique (
    account_id, target_id, origin_id, run_id
  );

alter table fort_private.routine_run
  add constraint routine_run_occurrence_fk
  foreign key (
    account_id, routine_occurrence_id, routine_id, routine_revision_id
  ) references fort_private.routine_occurrence(
    account_id, routine_occurrence_id, routine_id, routine_revision_id
  ),
  add constraint routine_run_revision_fk
  foreign key (
    account_id, routine_id, routine_revision_id,
    behavior_revision_id, binding_revision_id, result_conversation_id
  ) references fort_private.routine_revision(
    account_id, routine_id, routine_revision_id,
    behavior_revision_id, binding_revision_id, result_conversation_id
  ),
  add constraint routine_run_target_chain_fk
  foreign key (
    account_id, target_id, routine_occurrence_id, routine_run_id
  ) references fort_private.conversation_target(
    account_id, target_id, origin_id, run_id
  );

-- Persist the AEAD envelope version next to every encrypted collaboration
-- value. A constant, non-null default makes this backward-safe for existing
-- rows and old writers while the v1 key ring remains supported.
alter table fort_private.artifact
  add column encryption_envelope_version smallint not null default 1
    check (encryption_envelope_version = 1);
alter table fort_private.artifact_chunk
  add column envelope_version smallint not null default 1
    check (envelope_version = 1);
alter table fort_private.conversation_message
  add column body_envelope_version smallint not null default 1
    check (body_envelope_version = 1);
alter table fort_private.conversation_target
  add column error_envelope_version smallint not null default 1
    check (error_envelope_version = 1);
alter table fort_private.execution_attempt
  add column terminal_receipt_envelope_version smallint not null default 1
    check (terminal_receipt_envelope_version = 1);
alter table fort_private.worker_command
  add column payload_envelope_version smallint not null default 1
    check (payload_envelope_version = 1);
alter table fort_private.approval_receipt
  add column receipt_envelope_version smallint not null default 1
    check (receipt_envelope_version = 1);
alter table fort_private.handoff
  add column requested_result_envelope_version smallint not null default 1
    check (requested_result_envelope_version = 1);
alter table fort_private.routine_import_receipt
  add column fencing_receipt_envelope_version smallint not null default 1
    check (fencing_receipt_envelope_version = 1);
alter table fort_private.ledger_event
  add column sensitive_envelope_version smallint not null default 1
    check (sensitive_envelope_version = 1);

create or replace function fort_private.validate_artifact_chunk_insert()
returns trigger
language plpgsql
set search_path = ''
as $function$
declare
  manifest_state text;
  manifest_chunk_count integer;
  manifest_key_id text;
  manifest_envelope_version smallint;
begin
  select state, expected_chunk_count, encryption_key_id,
         encryption_envelope_version
    into manifest_state, manifest_chunk_count, manifest_key_id,
         manifest_envelope_version
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
  if new.envelope_version <> manifest_envelope_version then
    raise exception using errcode = '23514', message = 'artifact_chunk_version_mismatch';
  end if;

  return new;
end
$function$;

create or replace function fort_private.validate_artifact_transition()
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
  mismatched_versions integer;
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
           count(*) filter (
             where encryption_key_id <> new.encryption_key_id
           )::integer,
           count(*) filter (
             where envelope_version <> new.encryption_envelope_version
           )::integer
      into actual_count, first_index, last_index, total_plaintext,
           total_encoded, mismatched_keys, mismatched_versions
      from fort_private.artifact_chunk
     where account_id = new.account_id and artifact_id = new.artifact_id;

    if actual_count <> new.expected_chunk_count
       or first_index <> 0
       or last_index <> new.expected_chunk_count - 1
       or total_plaintext <> new.expected_plaintext_length
       or total_encoded <> new.expected_encoded_length
       or mismatched_keys <> 0
       or mismatched_versions <> 0 then
      raise exception using errcode = '23514', message = 'artifact_incomplete';
    end if;
  end if;

  return new;
end
$function$;

-- AuthorityGrant is sensitive Handoff authority evidence and already lives in
-- receipt_ciphertext. Keep only the receipt's routing/audit metadata in the
-- clear; this is the one intentionally subtractive operation in the migration.
alter table fort_private.approval_receipt
  drop column authority_delta;

revoke all privileges on function fort_private.validate_artifact_chunk_insert()
  from public, anon, authenticated, service_role, fort_gateway;
revoke all privileges on function fort_private.validate_artifact_transition()
  from public, anon, authenticated, service_role, fort_gateway;
