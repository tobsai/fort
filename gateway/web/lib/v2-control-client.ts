import "server-only";

import { createHash, createHmac, randomBytes } from "node:crypto";

import {
  MAX_CURSOR_PAGE_BYTES,
  type CursorPage,
  type CursorPageClient,
  type DurableEvent,
} from "@/lib/v2-events";

const EVENTS_CURSOR_PATH = "/api/v2/events/cursor";
const EVENTS_ROUTE_CLASS = "owner.events.read";
const CONTROL_AUDIENCE = "fort-control";
const SERVICE_ASSERTION_HEADER = "X-Fort-Service-Assertion";
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const KEY_ID_PATTERN = /^[A-Za-z0-9._-]{1,128}$/;
const NONCE_PATTERN = /^[A-Za-z0-9_-]{32,256}$/;

interface SignedCursorClientOptions {
  origin: string;
  accountByEmail: Readonly<Record<string, string>>;
  key: Uint8Array;
  keyID: string;
  ttlSeconds: number;
  nowSeconds?: () => number;
  nonce?: () => string;
  fetch?: (input: string | URL | Request, init?: RequestInit) => Promise<Response>;
}

interface AssertionInput {
  accountID: string;
  body: string;
  issuedAtSeconds: number;
  expiresAtSeconds: number;
  key: Uint8Array;
  keyID: string;
  nonce: string;
}

type Environment = Record<string, string | undefined>;

export function issueEventsReadAssertion(input: AssertionInput): string {
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

  const digest = createHash("sha256").update(input.body, "utf8").digest("hex");
  const header = base64url(JSON.stringify({ alg: "HS256", kid: input.keyID }));
  const claims = base64url(
    JSON.stringify({
      account_id: input.accountID,
      route_class: EVENTS_ROUTE_CLASS,
      aud: CONTROL_AUDIENCE,
      request_digest: digest,
      iat: input.issuedAtSeconds,
      exp: input.expiresAtSeconds,
      nonce: input.nonce,
    }),
  );
  const unsigned = `${header}.${claims}`;
  const signature = createHmac("sha256", input.key).update(unsigned, "utf8").digest("base64url");
  return `${unsigned}.${signature}`;
}

export function createSignedFortControlCursorClient(options: SignedCursorClientOptions): CursorPageClient {
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
    async readPage({ owner, afterCursor, signal }): Promise<CursorPage> {
      const normalizedEmail = owner.normalizedEmail.trim().toLowerCase();
      const accountID = normalizedEmail === owner.normalizedEmail ? accounts[normalizedEmail] : undefined;
      if (!accountID) throw unavailable();

      const body = JSON.stringify({ after_cursor: afterCursor });
      if (new TextEncoder().encode(body).byteLength > MAX_CURSOR_PAGE_BYTES) throw readFailed();
      const issuedAtSeconds = nowSeconds();
      const token = issueEventsReadAssertion({
        accountID,
        body,
        issuedAtSeconds,
        expiresAtSeconds: issuedAtSeconds + options.ttlSeconds,
        key,
        keyID: options.keyID,
        nonce: makeNonce(),
      });

      let response: Response;
      try {
        response = await fetcher(`${origin}${EVENTS_CURSOR_PATH}`, {
          method: "POST",
          headers: {
            "content-type": "application/json",
            [SERVICE_ASSERTION_HEADER]: token,
          },
          body,
          cache: "no-store",
          signal,
        });
      } catch {
        throw readFailed();
      }
      if (!response.ok) throw readFailed();

      try {
        return parseCursorPage(await readBoundedJSON(response));
      } catch {
        throw readFailed();
      }
    },
  };
}

export function createSignedFortControlCursorClientFromEnvironment(
  environment: Environment = process.env,
): CursorPageClient {
  try {
    const rawAccounts = environment.FORT_CONTROL_ACCOUNT_MAP;
    const rawKey = environment.FORT_CONTROL_ASSERTION_KEY_B64URL;
    if (!rawAccounts || !rawKey) throw unavailable();
    const accountByEmail = JSON.parse(rawAccounts) as unknown;
    if (!isRecord(accountByEmail)) throw unavailable();

    return createSignedFortControlCursorClient({
      origin: environment.FORT_CONTROL_ORIGIN ?? "",
      accountByEmail: recordOfStrings(accountByEmail),
      key: decodeBase64url(rawKey),
      keyID: environment.FORT_CONTROL_ASSERTION_KID ?? "",
      ttlSeconds: optionalTTL(environment.FORT_CONTROL_ASSERTION_TTL_SECONDS),
    });
  } catch {
    return {
      async readPage() {
        throw unavailable();
      },
    };
  }
}

function immutableOrigin(raw: string): string {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw unavailable();
  }
  if (
    url.protocol !== "https:" ||
    url.username !== "" ||
    url.password !== "" ||
    url.pathname !== "/" ||
    url.search !== "" ||
    url.hash !== ""
  ) {
    throw unavailable();
  }
  return url.origin;
}

function validatedAccounts(input: Readonly<Record<string, string>>): Readonly<Record<string, string>> {
  const accounts: Record<string, string> = Object.create(null) as Record<string, string>;
  for (const [email, accountID] of Object.entries(input)) {
    const normalizedEmail = email.trim().toLowerCase();
    if (email !== normalizedEmail || normalizedEmail.length === 0 || !UUID_PATTERN.test(accountID)) {
      throw unavailable();
    }
    accounts[normalizedEmail] = accountID.toLowerCase();
  }
  return accounts;
}

async function readBoundedJSON(response: Response): Promise<unknown> {
  const contentLength = response.headers.get("content-length");
  if (contentLength !== null) {
    if (!/^\d+$/.test(contentLength) || Number(contentLength) > MAX_CURSOR_PAGE_BYTES) throw readFailed();
  }
  if (!response.body) throw readFailed();

  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_CURSOR_PAGE_BYTES) {
      await reader.cancel();
      throw readFailed();
    }
    chunks.push(value);
  }

  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  return JSON.parse(text) as unknown;
}

function parseCursorPage(value: unknown): CursorPage {
  if (!isRecord(value) || !Array.isArray(value.events) || typeof value.next_cursor !== "string") {
    throw readFailed();
  }
  const events = value.events.map(parseEvent);
  if (!validWireCursor(value.next_cursor)) throw readFailed();
  const cursors = new Set(events.map((event) => event.cursor));
  if (cursors.size !== events.length) throw readFailed();
  if (events.length > 0 && events[events.length - 1]?.cursor !== value.next_cursor) throw readFailed();
  return { events, nextCursor: value.next_cursor };
}

function parseEvent(value: unknown): DurableEvent {
  if (!isRecord(value) || typeof value.cursor !== "string" || typeof value.kind !== "string") {
    throw readFailed();
  }
  if (!validWireCursor(value.cursor) || value.kind.length === 0 || !("data" in value)) throw readFailed();
  return { cursor: value.cursor, kind: value.kind, data: value.data };
}

function validWireCursor(value: string): boolean {
  return value.length > 0 && value.length <= 1024 && !/[\r\n\0]/.test(value);
}

function decodeBase64url(value: string): Uint8Array {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) throw unavailable();
  const decoded = Buffer.from(value, "base64url");
  if (decoded.toString("base64url") !== value) throw unavailable();
  return new Uint8Array(decoded);
}

function base64url(value: string): string {
  return Buffer.from(value, "utf8").toString("base64url");
}

function optionalTTL(value: string | undefined): number {
  if (value === undefined || value === "") return 30;
  const ttl = Number(value);
  if (!Number.isSafeInteger(ttl) || ttl < 1 || ttl > 60) throw unavailable();
  return ttl;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function recordOfStrings(value: Record<string, unknown>): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, item] of Object.entries(value)) {
    if (typeof item !== "string") throw unavailable();
    result[key] = item;
  }
  return result;
}

function unavailable(): Error {
  return new Error("fort-control cursor client unavailable");
}

function readFailed(): Error {
  return new Error("fort-control cursor read failed");
}
