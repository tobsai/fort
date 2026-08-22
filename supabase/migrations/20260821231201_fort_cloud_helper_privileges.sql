-- Trigger helpers are implementation details, never callable Fort APIs.
-- Explicit revocation is required because an additive migration may run under
-- a migration role whose default privileges differ from the bootstrap owner.
revoke all privileges on function fort_private.validate_artifact_chunk_insert()
  from public, anon, authenticated, service_role, fort_gateway;
