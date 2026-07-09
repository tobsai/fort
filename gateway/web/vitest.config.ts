import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

const root = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  resolve: {
    alias: {
      // `@/...` -> project root (mirrors tsconfig paths).
      "@": root.replace(/\/$/, ""),
      // `server-only` is a Next runtime guard with no test-time behavior; the
      // proxy-route test mocks the modules that import it, but alias it to a
      // stub so nothing throws if an import chain pulls it in.
      "server-only": root + "test/stubs/server-only.ts",
    },
  },
  test: {
    environment: "node",
    include: ["test/**/*.test.ts"],
  },
});
