# SPEC-015: Production Deploy Pipeline

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** HIGH — Fort needs a clean production deployment path  
**Depends on:** All specs (cumulative)

---

## Problem

Fort is currently deployed to Railway via a single Dockerfile, but the setup is fragile:
- No environment variable validation on startup (silent failures)
- No database migration system (schema changes require manual intervention)
- Build includes test files (wasteful)
- No proper process management (no graceful shutdown, no signal handling)
- Health endpoint returns static JSON (doesn't surface real health)
- No rate limiting on WS connections
- No CORS configuration

## Goals

1. Validated startup: fail fast with clear error if required env vars missing
2. Migration system: numbered SQL migration files applied in order, idempotent
3. Production Dockerfile: multi-stage build, no test files, non-root user
4. Graceful shutdown: SIGTERM drains in-flight tasks, closes DB, exits cleanly
5. Connection rate limiting: max 10 WS connections per IP, configurable
6. CORS configuration: `FORT_ALLOWED_ORIGINS` env var
7. Request ID tracing: every WS message and HTTP request gets a UUID for log correlation

## Non-Goals (v1)

- Blue-green deploys
- Database backups (separate concern)
- Horizontal scaling

---

## Design

### Environment Validator

File: `packages/core/src/config/env.ts`

```typescript
export function validateEnv(): FortEnvConfig {
  const errors: string[] = [];
  
  const config = {
    nodeEnv: process.env.NODE_ENV ?? 'development',
    port: parseInt(process.env.PORT ?? '3001'),
    sessionSecret: requireEnv('SESSION_SECRET', errors),
    googleClientId: process.env.GOOGLE_CLIENT_ID,        // optional
    googleClientSecret: process.env.GOOGLE_CLIENT_SECRET, // optional
    allowedEmails: process.env.FORT_ALLOWED_EMAILS?.split(',') ?? [],
    allowedOrigins: process.env.FORT_ALLOWED_ORIGINS?.split(',') ?? ['http://localhost:5173'],
    maxWsConnectionsPerIp: parseInt(process.env.FORT_MAX_WS_PER_IP ?? '10'),
    anthropicApiKey: process.env.ANTHROPIC_API_KEY,      // optional (can be set via LLM providers)
  };
  
  if (errors.length > 0) {
    console.error('FATAL: Missing required environment variables:');
    errors.forEach(e => console.error(`  - ${e}`));
    process.exit(1);
  }
  
  return config;
}
```

### Migration System

Directory: `packages/core/src/db/migrations/`

Files named: `001_initial_schema.sql`, `002_add_source_agent_id.sql`, etc.

```typescript
export class MigrationRunner {
  constructor(db: Database.Database) {}
  
  run(): void   // runs all pending migrations in order
  
  private getApplied(): number[]
  private applyMigration(version: number, sql: string): void
}
```

Schema: `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT)`.

Fort.ts calls `migrationRunner.run()` before any other DB initialization.

### Production Dockerfile (multi-stage)

```dockerfile
# Stage 1: Build
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
COPY packages/ ./packages/
RUN npm ci --workspaces
RUN npm run build --workspaces

# Stage 2: Runtime
FROM node:22-alpine AS runtime
WORKDIR /app
RUN addgroup -S fort && adduser -S fort -G fort
COPY --from=builder /app/packages/core/dist ./packages/core/dist
COPY --from=builder /app/packages/dashboard/dist ./packages/dashboard/dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/packages/core/package.json ./packages/core/
USER fort
EXPOSE 3001
CMD ["node", "packages/core/dist/index.js"]
```

### Graceful Shutdown

In `packages/core/src/index.ts` (entrypoint):

```typescript
const gracefulShutdown = async (signal: string) => {
  console.log(`Received ${signal}, shutting down gracefully...`);
  
  // 1. Stop accepting new WS connections
  wsServer.close();
  
  // 2. Wait for in-flight tasks (max 30s)
  await fort.drainTasks(30000);
  
  // 3. Stop scheduler
  fort.scheduler.stop();
  
  // 4. Close DB connections
  fort.close();
  
  console.log('Shutdown complete');
  process.exit(0);
};

process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
process.on('SIGINT', () => gracefulShutdown('SIGINT'));
```

### WS Rate Limiting

In WS upgrade handler:

```typescript
const connectionsByIp = new Map<string, number>();

wss.on('connection', (ws, req) => {
  const ip = req.headers['x-forwarded-for']?.split(',')[0] ?? req.socket.remoteAddress;
  const count = connectionsByIp.get(ip) ?? 0;
  
  if (count >= config.maxWsConnectionsPerIp) {
    ws.close(1008, 'Too many connections');
    return;
  }
  
  connectionsByIp.set(ip, count + 1);
  ws.on('close', () => connectionsByIp.set(ip, (connectionsByIp.get(ip) ?? 1) - 1));
  
  // ... existing connection handling
});
```

### CORS

Apply `cors` middleware with `FORT_ALLOWED_ORIGINS` env var. Default: localhost:5173 for development.

---

## Test Criteria

- validateEnv() exits process when SESSION_SECRET missing
- MigrationRunner applies migrations in order, skips applied
- MigrationRunner is idempotent (run twice → same result)
- Graceful shutdown closes DB and exits cleanly
- WS rate limiter rejects connections over limit
- All existing 587 tests still pass

---

## Railway Config Update

Update `railway.json`:
```json
{
  "$schema": "https://railway.app/railway.schema.json",
  "build": { "builder": "DOCKERFILE" },
  "deploy": {
    "healthcheckPath": "/health",
    "healthcheckTimeout": 30,
    "restartPolicyType": "ON_FAILURE",
    "restartPolicyMaxRetries": 3
  }
}
```
