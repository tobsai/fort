-- Each immutable Group membership revision pins the exact participant,
-- Behavior Revision, and Binding Revision that were current when accepted.
-- A participant may be reused by later membership revisions when the Agent's
-- exact binding has not changed; old revisions and target pins remain intact.
create table fort_private.conversation_member_binding (
  account_id uuid not null,
  membership_revision_id text not null,
  conversation_id text not null,
  agent_id text not null,
  behavior_revision_id text not null,
  binding_revision_id text not null,
  participant_id text not null,
  pinned_at timestamptz not null,
  primary key (account_id, membership_revision_id, agent_id),
  foreign key (
    account_id, membership_revision_id, conversation_id, agent_id
  ) references fort_private.conversation_member_revision(
    account_id, membership_revision_id, conversation_id, agent_id
  ),
  foreign key (
    account_id, conversation_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ) references fort_private.conversation_participant(
    account_id, conversation_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  ),
  unique (
    account_id, membership_revision_id, agent_id, behavior_revision_id,
    binding_revision_id, participant_id
  )
);

insert into fort_private.conversation_member_binding (
  account_id, membership_revision_id, conversation_id, agent_id,
  behavior_revision_id, binding_revision_id, participant_id, pinned_at
)
select member.account_id, member.membership_revision_id,
       member.conversation_id, member.agent_id,
       participant.behavior_revision_id, participant.binding_revision_id,
       participant.participant_id, member.created_at
  from fort_private.conversation_member_revision as member
  join fort_private.group_conversation as group_item
    on group_item.account_id = member.account_id
   and group_item.conversation_id = member.conversation_id
  join lateral (
    select evidence.behavior_revision_id, evidence.binding_revision_id,
           evidence.participant_id
      from fort_private.conversation_participant as evidence
     where evidence.account_id = member.account_id
       and evidence.conversation_id = member.conversation_id
       and evidence.agent_id = member.agent_id
       and evidence.created_at <= member.created_at
     order by evidence.created_at desc, evidence.participant_id
     limit 1
  ) as participant on true
on conflict (account_id, membership_revision_id, agent_id) do nothing;

do $membership_backfill$
begin
  if exists (
    select 1
      from fort_private.conversation_member_revision as member
      join fort_private.group_conversation as group_item
        on group_item.account_id = member.account_id
       and group_item.conversation_id = member.conversation_id
      left join fort_private.conversation_member_binding as binding
        on binding.account_id = member.account_id
       and binding.membership_revision_id = member.membership_revision_id
       and binding.agent_id = member.agent_id
     where binding.agent_id is null
  ) then
    raise exception using errcode = '23514', message = 'group_membership_binding_backfill_incomplete';
  end if;
end
$membership_backfill$;

create trigger conversation_member_binding_update_immutable
before update on fort_private.conversation_member_binding
for each row execute function fort_private.reject_immutable_mutation();
create trigger conversation_member_binding_delete_immutable
before delete on fort_private.conversation_member_binding
for each row execute function fort_private.reject_immutable_mutation();

alter table fort_private.conversation_member_binding enable row level security;
alter table fort_private.conversation_member_binding force row level security;
create policy account_isolation on fort_private.conversation_member_binding
for all to fort_gateway
using (account_id = nullif(current_setting('fort.account_id', true), '')::uuid)
with check (account_id = nullif(current_setting('fort.account_id', true), '')::uuid);

grant select, insert on fort_private.conversation_member_binding to fort_gateway;
revoke all privileges on fort_private.conversation_member_binding
  from public, anon, authenticated, service_role;

-- Trigger helpers remain implementation details after this additive migration.
revoke all privileges on function fort_private.reject_immutable_mutation()
  from public, anon, authenticated, service_role, fort_gateway;
