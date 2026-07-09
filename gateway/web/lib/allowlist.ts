// allowlist: the single source of truth for who may sign in. A personal
// gateway (spec 028, D1) — one owner, an email allowlist, no accounts model.
// Kept pure and dependency-free so the Auth.js signIn callback and its unit
// test share the exact same decision.

/** parseAllowlist splits FORT_ALLOWLIST into normalized, lowercased emails. */
export function parseAllowlist(raw: string | undefined): string[] {
  return (raw ?? "")
    .split(",")
    .map((e) => e.trim().toLowerCase())
    .filter((e) => e.length > 0);
}

/**
 * isAllowed reports whether `email` is on the allowlist. An empty or missing
 * email is never allowed; an empty allowlist allows no one (fail closed).
 */
export function isAllowed(email: string | null | undefined, raw: string | undefined): boolean {
  if (!email) return false;
  return parseAllowlist(raw).includes(email.trim().toLowerCase());
}
