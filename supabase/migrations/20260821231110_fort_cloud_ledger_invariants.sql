-- Additive convergence for environments that applied ledger_v1 before the
-- worker-artifact and exact-attribution invariants were introduced. Fresh
-- installs also run this migration, so every object is recreated idempotently.

create unique index if not exists conversation_message_one_agent_result
  on fort_private.conversation_message (account_id, target_id)
  where target_id is not null and message_kind = 'agent';

alter table fort_private.handoff
  drop constraint if exists handoff_agent_emitter_required;
alter table fort_private.handoff
  add constraint handoff_agent_emitter_required check (
    (
      creation_actor_kind = 'agent' and emitter_receipt_id is not null
      and source_agent_id is not null and creation_actor_id = source_agent_id
    ) or (creation_actor_kind <> 'agent' and emitter_receipt_id is null)
  );

create or replace function fort_private.validate_artifact_chunk_insert()
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

drop trigger if exists artifact_chunk_insert_invariant
  on fort_private.artifact_chunk;
create trigger artifact_chunk_insert_invariant
before insert on fort_private.artifact_chunk
for each row execute function fort_private.validate_artifact_chunk_insert();

drop trigger if exists artifact_chunk_update_immutable
  on fort_private.artifact_chunk;
create trigger artifact_chunk_update_immutable
before update on fort_private.artifact_chunk
for each row execute function fort_private.reject_immutable_mutation();

drop trigger if exists artifact_chunk_delete_immutable
  on fort_private.artifact_chunk;
create trigger artifact_chunk_delete_immutable
before delete on fort_private.artifact_chunk
for each row execute function fort_private.reject_immutable_mutation();
