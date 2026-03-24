# Stage 1: Build
FROM node:20-bookworm AS builder

WORKDIR /app

# Copy everything (respecting .dockerignore)
COPY . .

# Install dependencies (creates workspace symlinks)
RUN npm ci

# Build via project references — builds core first, then cli
RUN npx tsc --build tsconfig.json

# Verify declarations were emitted (fast fail if something went wrong)
RUN test -f packages/core/dist/index.d.ts || (echo "ERROR: core declarations not emitted" && exit 1)

# Build dashboard (vite, separate from tsc project references)
RUN npm run build --workspace=packages/dashboard

# Stage 2: Runtime
FROM node:20-bookworm-slim AS runtime

WORKDIR /app
ENV NODE_ENV=production

# Copy package files and install prod deps only
COPY package*.json ./
COPY packages/core/package*.json ./packages/core/
COPY packages/cli/package*.json ./packages/cli/
COPY packages/dashboard/package*.json ./packages/dashboard/
RUN npm ci --omit=dev

# Copy built artifacts
COPY --from=builder /app/packages/core/dist ./packages/core/dist
COPY --from=builder /app/packages/cli/dist ./packages/cli/dist
COPY --from=builder /app/packages/dashboard/dist ./packages/dashboard/dist

EXPOSE ${PORT:-4077}

CMD ["sh", "-c", "node packages/cli/dist/index.js portal --port ${PORT:-4077} --no-open"]
