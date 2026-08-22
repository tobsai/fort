import "server-only";

import { createHash, createHmac, randomBytes } from "node:crypto";

import type { OwnerSession } from "@/lib/v2-events";

const CONTROL_AUDIENCE = "fort-control";
const SERVICE_ASSERTION_HEADER = "X-Fort-Service-Assertion";
const MAX_FUNCTION_BODY_BYTES = 4 * 1024 * 1024;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const KEY_ID_PATTERN = /^[A-Za-z0-9._-]{1,128}$/;
const ROUTE_CLASS_PATTERN = /^[a-z0-9][a-z0-9._:-]{0,127}$/;
const NONCE_PATTERN = /^[A-Za-z0-9_-]{32,256}$/;
const ERROR_CODE_PATTERN = /^[a-z][a-z0-9_]{0,63}$/;
const EXPOSED_CONTROL_STATUSES = new Set([400, 404, 409, 413, 503]);

export class FortControlResponseError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
  ) {
    super(`fort-control response: ${code}`);
    this.name = "FortControlResponseError";
  }
}

export interface FortControlServiceClientOptions {
  origin: string;
  accountByEmail: Readonly<Record<string, string>>;
  key: Uint8Array;
  keyID: string;
  ttlSeconds: number;
  nowSeconds?: () => number;
  nonce?: () => string;
  fetch?: (input: string | URL | Request, init?: RequestInit) => Promise<Response>;
}

export interface FortControlRequest {
  owner: OwnerSession;
  path: string;
  routeClass: string;
  method: "GET" | "POST" | "PATCH";
  body?: string;
  signal?: AbortSignal;
}

export interface FortControlServiceClient {
  request(input: FortControlRequest): Promise<unknown>;
}

type Environment = Record<string, string | undefined>;

export function createFortControlServiceClient(
  options: FortControlServiceClientOptions,
): FortControlServiceClient {
  const origin = immutableOrigin(options.origin);
  const accounts = validatedAccounts(options.accountByEmail);
  if (!KEY_ID_PATTERN.test(options.keyID) || options.key.byteLength < 32) throw unavailable();
  if (!Number.isSafeInteger(options.ttlSeconds) || options.ttlSeconds < 1 || options.ttlSeconds > 60) {
    throw unavailable();
  }
  const key = new Uint8Array(options.key);
  const fetcher = options.fetch ?? fetch;
  const nowSeconds = options.nowSeconds ?? (() => Math.floor(Date.now() / 1_000));
  const makeNonce = options.nonce ?? (() => randomBytes(24).toString("base64url"));

  return {
    async request(input) {
      const email = input.owner.normalizedEmail.trim().toLowerCase();
      const accountID = email === input.owner.normalizedEmail ? accounts[email] : undefined;
      if (!accountID || !ROUTE_CLASS_PATTERN.test(input.routeClass)) throw unavailable();
      const url = trustedControlURL(origin, input.path);
      const body = input.body ?? "";
      if (input.method === "GET" && input.body !== undefined) throw requestFailed();
      if (new TextEncoder().encode(body).byteLength > MAX_FUNCTION_BODY_BYTES) throw requestFailed();

      const issuedAtSeconds = nowSeconds();
      const token = issueAssertion({
        accountID,
        routeClass: input.routeClass,
        body,
        issuedAtSeconds,
        expiresAtSeconds: issuedAtSeconds + options.ttlSeconds,
        key,
        keyID: options.keyID,
        nonce: makeNonce(),
      });
      const headers = new Headers({ [SERVICE_ASSERTION_HEADER]: token });
      if (input.body !== undefined) headers.set("content-type", "application/json");

      let response: Response;
      try {
        response = await fetcher(url, {
          method: input.method,
          headers,
          body: input.body,
          cache: "no-store",
          signal: input.signal,
        });
      } catch {
        throw requestFailed();
      }
      let responseText: string;
      try {
        responseText = await readBoundedText(response);
      } catch {
        throw requestFailed();
      }
      if (!response.ok) {
        const semanticError = parseSemanticError(response.status, responseText);
        if (semanticError) throw semanticError;
        throw requestFailed();
      }
      try {
        return JSON.parse(responseText) as unknown;
      } catch {
        throw requestFailed();
      }
    },
  };
}

export function createFortControlServiceClientFromEnvironment(
  environment: Environment = process.env,
): FortControlServiceClient {
  try {
    const rawAccounts = environment.FORT_CONTROL_ACCOUNT_MAP;
    const rawKey = environment.FORT_CONTROL_ASSERTION_KEY_B64URL;
    if (!rawAccounts || !rawKey) throw unavailable();
    const parsedAccounts = JSON.parse(rawAccounts) as unknown;
    if (!isRecord(parsedAccounts)) throw unavailable();
    return createFortControlServiceClient({
      origin: environment.FORT_CONTROL_ORIGIN ?? "",
      accountByEmail: stringRecord(parsedAccounts),
      key: decodeBase64url(rawKey),
      keyID: environment.FORT_CONTROL_ASSERTION_KID ?? "",
      ttlSeconds: optionalTTL(environment.FORT_CONTROL_ASSERTION_TTL_SECONDS),
    });
  } catch {
    return { async request() { throw unavailable(); } };
  }
}

function issueAssertion(input: {
  accountID: string;
  routeClass: string;
  body: string;
  issuedAtSeconds: number;
  expiresAtSeconds: number;
  key: Uint8Array;
  keyID: string;
  nonce: string;
}): string {
  if (
    !UUID_PATTERN.test(input.accountID) || !ROUTE_CLASS_PATTERN.test(input.routeClass) ||
    !KEY_ID_PATTERN.test(input.keyID) || input.key.byteLength < 32 || !NONCE_PATTERN.test(input.nonce) ||
    !Number.isSafeInteger(input.issuedAtSeconds) || !Number.isSafeInteger(input.expiresAtSeconds) ||
    input.expiresAtSeconds <= input.issuedAtSeconds || input.expiresAtSeconds - input.issuedAtSeconds > 60
  ) throw unavailable();
  const header = base64url(JSON.stringify({ alg: "HS256", kid: input.keyID }));
  const claims = base64url(JSON.stringify({
    account_id: input.accountID,
    route_class: input.routeClass,
    aud: CONTROL_AUDIENCE,
    request_digest: createHash("sha256").update(input.body, "utf8").digest("hex"),
    iat: input.issuedAtSeconds,
    exp: input.expiresAtSeconds,
    nonce: input.nonce,
  }));
  const unsigned = `${header}.${claims}`;
  const signature = createHmac("sha256", input.key).update(unsigned, "utf8").digest("base64url");
  return `${unsigned}.${signature}`;
}

function trustedControlURL(origin: string, path: string): string {
  if (!path.startsWith("/api/v2/") || /[\\\r\n\0]/.test(path)) throw unavailable();
  const rawPath = path.slice(0, path.search(/[?#]/) < 0 ? path.length : path.search(/[?#]/));
  for (const rawComponent of rawPath.slice(1).split("/")) {
    let component: string;
    try { component = decodeURIComponent(rawComponent); } catch { throw unavailable(); }
    if (!component || component === "." || component === ".." ||
        new TextEncoder().encode(component).byteLength > 512 || /[\/\\\r\n\0]/.test(component)) {
      throw unavailable();
    }
  }
  let url: URL;
  try { url = new URL(path, origin); } catch { throw unavailable(); }
  if (url.origin !== origin || url.hash || !url.pathname.startsWith("/api/v2/")) throw unavailable();
  return url.toString();
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
    if (email !== normalized || !normalized || !UUID_PATTERN.test(accountID)) throw unavailable();
    result[normalized] = accountID.toLowerCase();
  }
  return result;
}

async function readBoundedText(response: Response): Promise<string> {
  const declaredLength = response.headers.get("content-length");
  if (declaredLength !== null && (!/^\d+$/.test(declaredLength) || Number(declaredLength) > MAX_FUNCTION_BODY_BYTES)) {
    throw requestFailed();
  }
  if (!response.body) throw requestFailed();
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_FUNCTION_BODY_BYTES) {
      await reader.cancel();
      throw requestFailed();
    }
    chunks.push(value);
  }
  const joined = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    joined.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(joined);
}

function decodeBase64url(value: string): Uint8Array {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) throw unavailable();
  const decoded = Buffer.from(value, "base64url");
  if (decoded.toString("base64url") !== value) throw unavailable();
  return new Uint8Array(decoded);
}

function optionalTTL(value: string | undefined): number {
  if (!value) return 30;
  const ttl = Number(value);
  if (!Number.isSafeInteger(ttl) || ttl < 1 || ttl > 60) throw unavailable();
  return ttl;
}

function stringRecord(value: Record<string, unknown>): Record<string, string> {
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

function base64url(value: string): string { return Buffer.from(value, "utf8").toString("base64url"); }
function parseSemanticError(status: number, body: string): FortControlResponseError | null {
  if (!EXPOSED_CONTROL_STATUSES.has(status)) return null;
  let payload: unknown;
  try { payload = JSON.parse(body) as unknown; } catch { return null; }
  if (!isRecord(payload) || typeof payload.code !== "string" || !ERROR_CODE_PATTERN.test(payload.code)) return null;
  return new FortControlResponseError(status, payload.code);
}
function unavailable(): Error { return new Error("fort-control service client unavailable"); }
function requestFailed(): Error { return new Error("fort-control request failed"); }
