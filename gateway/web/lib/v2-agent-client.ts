import "server-only";

import { createHash, createHmac, randomBytes } from "node:crypto";

import type { OwnerSession } from "@/lib/v2-events";

const AGENTS_PATH = "/api/v2/agents?state=open";
const AGENTS_ROUTE_CLASS = "owner.agents.list";
const CONTROL_AUDIENCE = "fort-control";
const SERVICE_ASSERTION_HEADER = "X-Fort-Service-Assertion";
const MAX_FUNCTION_BODY_BYTES = 4 * 1024 * 1024;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const KEY_ID_PATTERN = /^[A-Za-z0-9._-]{1,128}$/;
const NONCE_PATTERN = /^[A-Za-z0-9_-]{32,256}$/;

export interface AgentRecordWire {
  agent: {
    id: string;
    account_id: string;
    state: "open" | "archived";
    canonical_conversation_id?: string;
  };
  profile: {
    name: string;
    title?: string;
    avatar_url?: string;
    pinned?: boolean;
    hidden?: boolean;
  };
  binding: {
    provider: string;
    requested_model: string;
    resolved_model?: string;
    computer_id?: string;
    cloud_runtime?: string;
  };
  home: {
    id: string;
    title: string;
    state: "open" | "archived";
  };
  [key: string]: unknown;
}

interface AgentClientOptions {
  origin: string;
  accountByEmail: Readonly<Record<string, string>>;
  key: Uint8Array;
  keyID: string;
  ttlSeconds: number;
  nowSeconds?: () => number;
  nonce?: () => string;
  fetch?: (input: string | URL | Request, init?: RequestInit) => Promise<Response>;
}

interface AgentClient {
  list(input: { owner: OwnerSession; signal?: AbortSignal }): Promise<AgentRecordWire[]>;
}

type Environment = Record<string, string | undefined>;

export function createSignedFortControlAgentClient(options: AgentClientOptions): AgentClient {
  const origin = immutableOrigin(options.origin);
  const accounts = validatedAccounts(options.accountByEmail);
  if (!KEY_ID_PATTERN.test(options.keyID) || options.key.byteLength < 32) throw unavailable();
  if (!Number.isSafeInteger(options.ttlSeconds) || options.ttlSeconds < 1 || options.ttlSeconds > 60) {
    throw unavailable();
  }
  const key = new Uint8Array(options.key);
  const nowSeconds = options.nowSeconds ?? (() => Math.floor(Date.now() / 1_000));
  const makeNonce = options.nonce ?? (() => randomBytes(24).toString("base64url"));
  const fetcher = options.fetch ?? fetch;

  return {
    async list({ owner, signal }) {
      const normalizedEmail = owner.normalizedEmail.trim().toLowerCase();
      const accountID = normalizedEmail === owner.normalizedEmail ? accounts[normalizedEmail] : undefined;
      if (!accountID) throw unavailable();
      const issuedAtSeconds = nowSeconds();
      const token = issueAgentListAssertion({
        accountID,
        issuedAtSeconds,
        expiresAtSeconds: issuedAtSeconds + options.ttlSeconds,
        key,
        keyID: options.keyID,
        nonce: makeNonce(),
      });

      let response: Response;
      try {
        response = await fetcher(`${origin}${AGENTS_PATH}`, {
          method: "GET",
          headers: { [SERVICE_ASSERTION_HEADER]: token },
          cache: "no-store",
          signal,
        });
      } catch {
        throw readFailed();
      }
      if (!response.ok) throw readFailed();
      const contentLength = Number(response.headers.get("content-length"));
      if (Number.isFinite(contentLength) && contentLength > MAX_FUNCTION_BODY_BYTES) throw readFailed();
      const payload = new Uint8Array(await response.arrayBuffer());
      if (payload.byteLength > MAX_FUNCTION_BODY_BYTES) throw readFailed();
      let decoded: unknown;
      try {
        decoded = JSON.parse(new TextDecoder().decode(payload)) as unknown;
      } catch {
        throw readFailed();
      }
      if (!Array.isArray(decoded)) throw readFailed();
      return decoded.map(parseAgentRecordWire);
    },
  };
}

export function createSignedFortControlAgentClientFromEnvironment(
  environment: Environment = process.env,
): AgentClient {
  try {
    const rawAccounts = environment.FORT_CONTROL_ACCOUNT_MAP;
    const rawKey = environment.FORT_CONTROL_ASSERTION_KEY_B64URL;
    if (!rawAccounts || !rawKey) throw unavailable();
    const accountByEmail = JSON.parse(rawAccounts) as unknown;
    if (!isRecord(accountByEmail)) throw unavailable();
    return createSignedFortControlAgentClient({
      origin: environment.FORT_CONTROL_ORIGIN ?? "",
      accountByEmail: recordOfStrings(accountByEmail),
      key: decodeBase64url(rawKey),
      keyID: environment.FORT_CONTROL_ASSERTION_KID ?? "",
      ttlSeconds: optionalTTL(environment.FORT_CONTROL_ASSERTION_TTL_SECONDS),
    });
  } catch {
    return { async list() { throw unavailable(); } };
  }
}

function issueAgentListAssertion(input: {
  accountID: string;
  issuedAtSeconds: number;
  expiresAtSeconds: number;
  key: Uint8Array;
  keyID: string;
  nonce: string;
}): string {
  if (
    !UUID_PATTERN.test(input.accountID) ||
    !KEY_ID_PATTERN.test(input.keyID) ||
    input.key.byteLength < 32 ||
    !NONCE_PATTERN.test(input.nonce) ||
    !Number.isSafeInteger(input.issuedAtSeconds) ||
    !Number.isSafeInteger(input.expiresAtSeconds) ||
    input.expiresAtSeconds <= input.issuedAtSeconds ||
    input.expiresAtSeconds - input.issuedAtSeconds > 60
  ) {
    throw unavailable();
  }
  const header = base64url(JSON.stringify({ alg: "HS256", kid: input.keyID }));
  const claims = base64url(JSON.stringify({
    account_id: input.accountID,
    route_class: AGENTS_ROUTE_CLASS,
    aud: CONTROL_AUDIENCE,
    request_digest: createHash("sha256").update("").digest("hex"),
    iat: input.issuedAtSeconds,
    exp: input.expiresAtSeconds,
    nonce: input.nonce,
  }));
  const unsigned = `${header}.${claims}`;
  const signature = createHmac("sha256", input.key).update(unsigned, "utf8").digest("base64url");
  return `${unsigned}.${signature}`;
}

export function parseAgentRecordWire(value: unknown): AgentRecordWire {
  if (!isRecord(value) || !isRecord(value.agent) || !isRecord(value.profile) ||
      !isRecord(value.binding) || !isRecord(value.home)) throw readFailed();
  const { agent, profile, binding, home } = value;
  if (
    typeof agent.id !== "string" || typeof agent.account_id !== "string" ||
    (agent.state !== "open" && agent.state !== "archived") ||
    typeof profile.name !== "string" || profile.name.trim().length === 0 ||
    typeof binding.provider !== "string" || typeof binding.requested_model !== "string" ||
    typeof home.id !== "string" || typeof home.title !== "string" ||
    (home.state !== "open" && home.state !== "archived")
  ) {
    throw readFailed();
  }
  return value as unknown as AgentRecordWire;
}

function immutableOrigin(raw: string): string {
  let url: URL;
  try { url = new URL(raw); } catch { throw unavailable(); }
  if (url.protocol !== "https:" || url.username || url.password || url.pathname !== "/" || url.search || url.hash) {
    throw unavailable();
  }
  return url.origin;
}

function validatedAccounts(input: Readonly<Record<string, string>>): Readonly<Record<string, string>> {
  const result: Record<string, string> = Object.create(null) as Record<string, string>;
  for (const [email, accountID] of Object.entries(input)) {
    const normalized = email.trim().toLowerCase();
    if (normalized !== email || normalized.length === 0 || !UUID_PATTERN.test(accountID)) throw unavailable();
    result[normalized] = accountID.toLowerCase();
  }
  return result;
}

function decodeBase64url(value: string): Uint8Array {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) throw unavailable();
  const decoded = Buffer.from(value, "base64url");
  if (decoded.toString("base64url") !== value) throw unavailable();
  return new Uint8Array(decoded);
}

function base64url(value: string): string { return Buffer.from(value, "utf8").toString("base64url"); }

function optionalTTL(value: string | undefined): number {
  if (!value) return 30;
  const ttl = Number(value);
  if (!Number.isSafeInteger(ttl) || ttl < 1 || ttl > 60) throw unavailable();
  return ttl;
}

function recordOfStrings(value: Record<string, unknown>): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, item] of Object.entries(value)) {
    if (typeof item !== "string") throw unavailable();
    result[key] = item;
  }
  return result;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function unavailable(): Error { return new Error("fort-control Agent client unavailable"); }
function readFailed(): Error { return new Error("fort-control Agent read failed"); }
