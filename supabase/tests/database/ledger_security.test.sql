begin;

create extension if not exists pgtap with schema extensions;

select plan(26);

select ok(
  not has_schema_privilege('anon', 'fort_private', 'usage'),
  'anon has no private schema access'
);
select ok(
  not has_schema_privilege('authenticated', 'fort_private', 'usage'),
  'authenticated has no private schema access'
);
select ok(
  not has_schema_privilege('service_role', 'fort_private', 'usage'),
  'service_role has no private schema access'
);
select ok(
  has_schema_privilege('fort_gateway', 'fort_private', 'usage'),
  'only the server runtime role receives private schema usage'
);

select is(
  (select count(*)::integer
     from pg_catalog.pg_tables
    where schemaname = 'fort_private'
      and (
        has_table_privilege('anon', quote_ident(schemaname) || '.' || quote_ident(tablename), 'select')
        or has_table_privilege('authenticated', quote_ident(schemaname) || '.' || quote_ident(tablename), 'select')
        or has_table_privilege('service_role', quote_ident(schemaname) || '.' || quote_ident(tablename), 'select')
      )),
  0,
  'Data API roles have no Fort table privileges'
);

select is(
  (select count(*)::integer
     from pg_catalog.pg_sequences
    where schemaname = 'fort_private'
      and (
        has_sequence_privilege('anon', quote_ident(schemaname) || '.' || quote_ident(sequencename), 'usage')
        or has_sequence_privilege('authenticated', quote_ident(schemaname) || '.' || quote_ident(sequencename), 'usage')
        or has_sequence_privilege('service_role', quote_ident(schemaname) || '.' || quote_ident(sequencename), 'usage')
      )),
  0,
  'Data API roles have no Fort sequence privileges'
);

select is(
  (select count(*)::integer
     from pg_catalog.pg_proc as procedure
     join pg_catalog.pg_namespace as namespace on namespace.oid = procedure.pronamespace
    where namespace.nspname = 'fort_private'
      and (
        has_function_privilege('anon', procedure.oid, 'execute')
        or has_function_privilege('authenticated', procedure.oid, 'execute')
        or has_function_privilege('service_role', procedure.oid, 'execute')
        or has_function_privilege('fort_gateway', procedure.oid, 'execute')
      )),
  0,
  'private helper functions are not callable APIs'
);

create table fort_private._default_table_probe (id bigint);
create sequence fort_private._default_sequence_probe;

select ok(
  not has_table_privilege('anon', 'fort_private._default_table_probe', 'select')
  and not has_table_privilege('authenticated', 'fort_private._default_table_probe', 'select')
  and not has_table_privilege('service_role', 'fort_private._default_table_probe', 'select')
  and not has_table_privilege('fort_gateway', 'fort_private._default_table_probe', 'select'),
  'future Fort tables are not auto-granted'
);
select ok(
  not has_sequence_privilege('anon', 'fort_private._default_sequence_probe', 'usage')
  and not has_sequence_privilege('authenticated', 'fort_private._default_sequence_probe', 'usage')
  and not has_sequence_privilege('service_role', 'fort_private._default_sequence_probe', 'usage')
  and not has_sequence_privilege('fort_gateway', 'fort_private._default_sequence_probe', 'usage'),
  'future Fort sequences are not auto-granted'
);
select ok(
  not exists (
    select 1
      from pg_catalog.pg_default_acl as defaults
      join pg_catalog.pg_roles as owner on owner.oid = defaults.defaclrole
      join pg_catalog.pg_namespace as namespace on namespace.oid = defaults.defaclnamespace
      cross join lateral aclexplode(defaults.defaclacl) as privilege
     where owner.rolname = 'postgres'
       and namespace.nspname = 'fort_private'
       and defaults.defaclobjtype = 'f'
       and privilege.grantee = 0
       and privilege.privilege_type = 'EXECUTE'
  ),
  'future postgres-owned Fort functions are not executable by PUBLIC'
);

insert into fort_private.fort_account(account_id, normalized_email)
values
  ('00000000-0000-4000-8000-00000000000a', 'a@example.com'),
  ('00000000-0000-4000-8000-00000000000b', 'b@example.com');

create temporary table rls_observation (
  observation text primary key,
  value text not null
) on commit drop;
grant select, insert on rls_observation to fort_gateway;

set local role fort_gateway;
select set_config('fort.account_id', '00000000-0000-4000-8000-00000000000a', true);

insert into rls_observation(observation, value)
select 'a_account_count', count(*)::text from fort_private.fort_account;
insert into rls_observation(observation, value)
select 'a_email', normalized_email from fort_private.fort_account;

insert into fort_private.service_assertion_nonce(
  account_id, key_id, nonce, expires_at, claimed_at
) values (
  '00000000-0000-4000-8000-00000000000a', 'key-a', 'nonce-a',
  clock_timestamp() + interval '5 minutes', clock_timestamp()
);

insert into rls_observation(observation, value)
select 'a_nonce_count', count(*)::text from fort_private.service_assertion_nonce;

do $duplicate_nonce$
begin
  begin
    insert into fort_private.service_assertion_nonce(
      account_id, key_id, nonce, expires_at, claimed_at
    ) values (
      '00000000-0000-4000-8000-00000000000a', 'key-a', 'nonce-a',
      clock_timestamp() + interval '5 minutes', clock_timestamp()
    );
    insert into rls_observation values ('duplicate_nonce_rejected', 'false');
  exception when unique_violation then
    insert into rls_observation values ('duplicate_nonce_rejected', 'true');
  end;
end
$duplicate_nonce$;

do $cross_account_nonce$
begin
  begin
    insert into fort_private.service_assertion_nonce(
      account_id, key_id, nonce, expires_at, claimed_at
    ) values (
      '00000000-0000-4000-8000-00000000000b', 'key-b', 'nonce-b',
      clock_timestamp() + interval '5 minutes', clock_timestamp()
    );
    insert into rls_observation values ('cross_account_nonce_rejected', 'false');
  exception when insufficient_privilege then
    insert into rls_observation values ('cross_account_nonce_rejected', 'true');
  end;
end
$cross_account_nonce$;

select set_config('fort.account_id', '00000000-0000-4000-8000-00000000000b', true);
insert into rls_observation(observation, value)
select 'b_account_count', count(*)::text from fort_private.fort_account;
insert into rls_observation(observation, value)
select 'b_email', normalized_email from fort_private.fort_account;
insert into rls_observation(observation, value)
select 'b_nonce_count', count(*)::text from fort_private.service_assertion_nonce;

select set_config('fort.account_id', '', true);
insert into rls_observation(observation, value)
select 'blank_account_count', count(*)::text from fort_private.fort_account;
insert into rls_observation(observation, value)
select 'blank_nonce_count', count(*)::text from fort_private.service_assertion_nonce;

select set_config('fort.account_id', 'malformed-account-id', true);
do $malformed_account$
begin
  begin
    perform count(*) from fort_private.fort_account;
    insert into rls_observation values ('malformed_account_rejected', 'false');
  exception when invalid_text_representation then
    insert into rls_observation values ('malformed_account_rejected', 'true');
  end;
end
$malformed_account$;

reset role;

select is(
  (select value from rls_observation where observation = 'a_account_count'),
  '1',
  'account A reads only account A'
);
select is(
  (select value from rls_observation where observation = 'a_email'),
  'a@example.com',
  'account A cannot observe account B identity'
);
select is(
  (select value from rls_observation where observation = 'a_nonce_count'),
  '1',
  'account A sees its claimed service assertion nonce'
);
select is(
  (select value from rls_observation where observation = 'duplicate_nonce_rejected'),
  'true',
  'a duplicate key ID and nonce is rejected as a replay'
);
select is(
  (select value from rls_observation where observation = 'cross_account_nonce_rejected'),
  'true',
  'account A cannot claim a nonce for account B'
);
select is(
  (select value from rls_observation where observation = 'b_account_count'),
  '1',
  'the same transaction can be explicitly rebound to account B'
);
select is(
  (select value from rls_observation where observation = 'b_email'),
  'b@example.com',
  'rebinding to account B exposes no account A row'
);
select is(
  (select value from rls_observation where observation = 'b_nonce_count'),
  '0',
  'account B cannot observe account A nonce claims'
);
select is(
  (select value from rls_observation where observation = 'blank_account_count'),
  '0',
  'a blank transaction identity fails closed'
);
select is(
  (select value from rls_observation where observation = 'blank_nonce_count'),
  '0',
  'blank identity exposes no replay claims'
);
select is(
  (select value from rls_observation where observation = 'malformed_account_rejected'),
  'true',
  'a malformed account UUID is rejected'
);

select is(
  (select count(*)::integer
     from pg_catalog.pg_class as relation
     join pg_catalog.pg_namespace as namespace on namespace.oid = relation.relnamespace
    where namespace.nspname = 'fort_private'
      and relation.relkind in ('r', 'p')
      and (not relation.relrowsecurity or not relation.relforcerowsecurity)),
  1,
  'only the transaction-local default-privilege probe lacks RLS'
);

select ok(
  not has_table_privilege('fort_gateway', 'fort_private.fort_account', 'delete'),
  'fort_gateway cannot delete account evidence'
);
select ok(
  not has_table_privilege('fort_gateway', 'fort_private.ledger_event', 'update'),
  'fort_gateway cannot update append-only events'
);
select ok(
  not has_table_privilege('fort_gateway', 'fort_private.service_assertion_nonce', 'update')
  and not has_table_privilege('fort_gateway', 'fort_private.service_assertion_nonce', 'delete'),
  'claimed assertion nonces are immutable'
);
select ok(
  has_table_privilege('fort_gateway', 'fort_private.execution_source_config_observation', 'insert')
  and not has_table_privilege('fort_gateway', 'fort_private.execution_source_config_observation', 'update')
  and not has_table_privilege('fort_gateway', 'fort_private.execution_source_config_observation', 'delete'),
  'runtime can append but cannot rewrite Execution Source observations'
);

select * from finish();
rollback;
