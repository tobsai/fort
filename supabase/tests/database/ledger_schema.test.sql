begin;

create extension if not exists pgtap with schema extensions;

select plan(64);

create function pg_temp.has_named_constraint(
  schema_name name,
  table_name name,
  constraint_name name,
  description text
) returns text
language sql
as $$
  select ok(
    exists (
      select 1
        from pg_catalog.pg_constraint as constraint_item
        join pg_catalog.pg_class as relation
          on relation.oid = constraint_item.conrelid
        join pg_catalog.pg_namespace as namespace
          on namespace.oid = relation.relnamespace
       where namespace.nspname = schema_name
         and relation.relname = table_name
         and constraint_item.conname = constraint_name
    ),
    description
  )
$$;

select has_schema('fort_private', 'Fort ledger is isolated in a private schema');
select has_table('fort_private', 'stable_agent', 'stable Agents are durable ledger roots');
select has_table('fort_private', 'agent_profile_revision', 'Agent presentation history is revisioned');
select has_table('fort_private', 'agent_behavior_revision', 'Agent behavior is revisioned');
select has_table('fort_private', 'agent_binding_revision', 'Agent execution bindings are revisioned');
select pg_temp.has_named_constraint(
  'fort_private',
  'stable_agent',
  'stable_agent_current_behavior_binding_fk',
  'an Agent current Binding is database-bound to its current Behavior'
);
select has_table('fort_private', 'agent_binding_transition', 'Agent binding changes retain append-only acceptance evidence');
select has_trigger(
  'fort_private',
  'agent_binding_transition',
  'agent_binding_transition_immutable',
  'Agent binding transition evidence is immutable'
);
select has_table('fort_private', 'agent_conversation_pin', 'secondary Conversation pins retain revisioned evidence');
select has_trigger(
  'fort_private',
  'agent_conversation_pin',
  'agent_conversation_pin_immutable',
  'Agent Conversation pin revisions are immutable'
);
select has_table(
  'fort_private',
  'execution_source_config_observation',
  'Execution Source configuration observations are durable'
);
select has_column(
  'fort_private',
  'execution_source_config_observation',
  'observation_sequence',
  'Execution Source observations have database-defined append order'
);
select has_trigger(
  'fort_private',
  'execution_source_config_observation',
  'execution_source_config_observation_immutable',
  'Execution Source configuration observations are append-only'
);
select has_table('fort_private', 'conversation', 'Conversations are durable ledger roots');
select pg_temp.has_named_constraint(
  'fort_private',
  'conversation_participant',
  'conversation_participant_behavior_binding_fk',
  'Conversation participant evidence binds one exact Behavior and Binding pair'
);
select has_table('fort_private', 'group_conversation', 'stable Group identities map one-to-one to Conversations');
select has_table('fort_private', 'conversation_membership_revision', 'Conversation membership sets are revisioned');
select has_table('fort_private', 'conversation_member_binding', 'Group membership revisions pin exact Agent execution evidence');
select has_trigger(
  'fort_private',
  'conversation_member_binding',
  'conversation_member_binding_update_immutable',
  'Group membership binding evidence cannot be rewritten'
);
select has_trigger(
  'fort_private',
  'conversation_member_binding',
  'conversation_member_binding_delete_immutable',
  'Group membership binding evidence cannot be deleted'
);
select has_table('fort_private', 'conversation_target', 'turn targets are durable');
select has_column('fort_private', 'conversation_turn', 'cancellation_policy', 'Group Turns freeze an exact cancellation policy');
select has_column('fort_private', 'conversation_turn', 'approval_policy', 'Group Turns freeze an exact approval policy');
select has_table('fort_private', 'execution_attempt', 'execution attempts are durable');
select has_column('fort_private', 'execution_attempt', 'terminal_receipt_id', 'terminal receipts have stable identities');
select has_table('fort_private', 'artifact', 'large context and output artifacts have manifests');
select has_table('fort_private', 'artifact_chunk', 'artifact chunks are durable');
select has_trigger(
  'fort_private',
  'artifact',
  'artifact_transition_invariant',
  'artifact finalization is validated by the database'
);
select has_trigger(
  'fort_private',
  'artifact',
  'artifact_delete_immutable',
  'artifact manifests cannot be deleted'
);
select has_trigger(
  'fort_private',
  'artifact_chunk',
  'artifact_chunk_insert_invariant',
  'artifact chunks must match an uploading manifest'
);
select has_trigger(
  'fort_private',
  'artifact_chunk',
  'artifact_chunk_update_immutable',
  'artifact chunks cannot be changed after insertion'
);
select has_trigger(
  'fort_private',
  'artifact_chunk',
  'artifact_chunk_delete_immutable',
  'artifact chunks cannot be deleted after insertion'
);
select has_index(
  'fort_private',
  'conversation_message',
  'conversation_message_one_agent_result',
  'one successful ordinary target can author only one authoritative Agent message'
);
select has_table('fort_private', 'handoff', 'Handoffs are durable commands');
select has_column(
  'fort_private',
  'handoff',
  'source_execution_attempt_id',
  'Agent-created Handoffs retain their exact source execution attempt'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_source_behavior_binding_fk',
  'a Handoff source Binding is database-bound to its Behavior'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_recipient_behavior_binding_fk',
  'a Handoff recipient Binding is database-bound to its Behavior'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_emitter_exact_source_command_fk',
  'an Agent Handoff binds its emitter receipt to the exact source attempt, revisions, and command'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_source_conversation_message_fk',
  'a Handoff source message belongs to its source Conversation'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_source_turn_message_fk',
  'a Handoff source Turn is the Turn that owns its source message'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_parent_source_binding_fk',
  'a nested Handoff source is its parent recipient revision pair'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'handoff',
  'handoff_parent_result_message_fk',
  'a nested Handoff starts from its parent authoritative result message'
);
select ok(
  (
    select position(
      'creation_actor_id = source_agent_id' in pg_get_constraintdef(oid)
    ) > 0
      from pg_catalog.pg_constraint
     where conrelid = 'fort_private.handoff'::regclass
       and conname = 'handoff_agent_emitter_required'
  ),
  'Agent-created Handoffs bind the creation actor to the exact source Agent'
);
select has_table('fort_private', 'handoff_projection', 'Handoff projections are reference-only records');
select has_table('fort_private', 'routine', 'Agent-owned Routines are durable');
select pg_temp.has_named_constraint(
  'fort_private',
  'routine_revision',
  'routine_revision_behavior_binding_fk',
  'a Routine Revision Binding is database-bound to its Behavior'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'routine_import_receipt',
  'routine_import_receipt_revision_fk',
  'a Routine import receipt binds one Revision of the same Routine'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'routine_occurrence',
  'routine_occurrence_revision_fk',
  'a Routine occurrence binds one Revision of the same Routine'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'routine_run',
  'routine_run_occurrence_fk',
  'a Routine run binds its exact occurrence, Routine, and Revision chain'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'routine_run',
  'routine_run_revision_fk',
  'a Routine run binds its exact Behavior, Binding, and result Conversation to its Revision'
);
select pg_temp.has_named_constraint(
  'fort_private',
  'routine_run',
  'routine_run_target_chain_fk',
  'a Routine run binds its target origin and run identity to the same occurrence and run'
);
select has_table('fort_private', 'source_routine_projection', 'source-native Routines are projections only');
select has_table('fort_private', 'worker', 'execution workers are enrolled durably');
select has_table('fort_private', 'worker_lease', 'worker claims use durable leases');
select has_table('fort_private', 'worker_cancellation_ack', 'worker cancellation acknowledgements are durable');
select has_column('fort_private', 'worker_cancellation_ack', 'fence_token', 'cancellation acknowledgements pin the exact lease fence');
select has_trigger(
  'fort_private',
  'worker_cancellation_ack',
  'worker_cancellation_ack_immutable',
  'worker cancellation acknowledgements are append-only'
);
select is(
  (
    select pg_get_constraintdef(oid)
      from pg_catalog.pg_constraint
     where conrelid = 'fort_private.worker_cancellation_ack'::regclass
       and conname = 'worker_cancellation_ack_exact_lease_fk'
  ),
  'FOREIGN KEY (account_id, execution_attempt_id, lease_id, fence_token, target_id, worker_id) REFERENCES fort_private.worker_lease(account_id, execution_attempt_id, lease_id, fence_token, target_id, worker_id)',
  'cancellation acknowledgements pin the exact lease target and worker identity'
);
select has_table('fort_private', 'ledger_event', 'the event cursor is durable and append-only');
select has_table('fort_private', 'service_assertion_nonce', 'service assertion replay claims are durable');

select is(
  (
    select count(*)::integer
      from information_schema.columns
     where table_schema = 'fort_private'
       and (table_name, column_name) in (
         ('artifact', 'encryption_envelope_version'),
         ('artifact_chunk', 'envelope_version'),
         ('conversation_message', 'body_envelope_version'),
         ('conversation_target', 'error_envelope_version'),
         ('execution_attempt', 'terminal_receipt_envelope_version'),
         ('worker_command', 'payload_envelope_version'),
         ('approval_receipt', 'receipt_envelope_version'),
         ('handoff', 'requested_result_envelope_version'),
         ('routine_import_receipt', 'fencing_receipt_envelope_version'),
         ('ledger_event', 'sensitive_envelope_version')
       )
       and data_type = 'smallint'
       and is_nullable = 'NO'
       and column_default like '1%'
  ),
  10,
  'every encrypted collaboration body, receipt, and artifact persists envelope version one for old and new rows'
);

select ok(
  not exists (
    select 1
      from information_schema.columns
     where table_schema = 'fort_private'
       and table_name = 'approval_receipt'
       and column_name = 'authority_delta'
  ),
  'Approval Receipts do not duplicate encrypted authority in plaintext'
);

select is(
  (select rolbypassrls from pg_roles where rolname = 'fort_gateway'),
  false,
  'fort_gateway cannot bypass RLS'
);

select is(
  (select count(*)::integer
     from pg_catalog.pg_class as relation
     join pg_catalog.pg_namespace as namespace on namespace.oid = relation.relnamespace
    where namespace.nspname = 'fort_private'
      and relation.relkind in ('r', 'p')
      and (not relation.relrowsecurity or not relation.relforcerowsecurity)),
  0,
  'every Fort table enables and forces RLS'
);

select * from finish();
rollback;
