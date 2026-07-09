// Binds the `cloudflare:test` env to the Worker's Env shape so tests get typed
// bindings (REGISTRY, TUNNEL, GATEWAY_SECRET, ...).
import type { Env } from "../src/types";

declare module "cloudflare:test" {
  interface ProvidedEnv extends Env {}
}
