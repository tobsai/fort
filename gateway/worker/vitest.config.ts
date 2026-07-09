import { defineWorkersConfig } from "@cloudflare/vitest-pool-workers/config";

// Runs the tests inside workerd via miniflare, with the real Durable Object
// bindings + migrations from wrangler.toml.
//   • Each test FILE gets its own isolated Worker (singleWorker: false), so the
//     shared RegistryDO singleton is never carried across a module-reload
//     boundary (which throws "…changed, invalidating this Durable Object"). The
//     E2E relay's daemon WebSocket + proxy POSTs all live in one file, so they
//     still share that file's runtime.
//   • isolatedStorage is OFF: a Durable Object holding a hibernatable WebSocket
//     can't be torn down between tests cleanly (a documented pool limitation),
//     so tests instead each use their own freshly-joined machine id.
export default defineWorkersConfig({
  test: {
    poolOptions: {
      workers: {
        singleWorker: false,
        isolatedStorage: false,
        wrangler: { configPath: "./wrangler.toml" },
        miniflare: {
          bindings: {
            GATEWAY_SECRET: "test-secret",
            CODE_TTL_SECONDS: "900",
          },
        },
      },
    },
  },
});
