# Stage 1: Build
FROM node:20-bookworm AS builder

WORKDIR /app

# Copy source (node_modules and dist excluded via .dockerignore)
COPY . .

# Install dependencies
RUN npm ci

# Remove any stale incremental build state (tsbuildinfo files can prevent emit)
RUN find . -name "*.tsbuildinfo" -not -path "*/node_modules/*" -delete

# Build step 1: core package
RUN ./node_modules/.bin/tsc -p packages/core/tsconfig.json
RUN test -f packages/core/dist/index.d.ts || (echo "ERROR: core dist/index.d.ts missing" && exit 1)

# Build step 2: cli package
RUN ./node_modules/.bin/tsc -p packages/cli/tsconfig.json
RUN test -f packages/cli/dist/index.js || (echo "ERROR: cli dist/index.js missing" && exit 1)

# Build step 3: dashboard (vite)
RUN npm run build --workspace=packages/dashboard

# Stage 2: Runtime
FROM node:20-bookworm-slim AS runtime

WORKDIR /app
ENV NODE_ENV=production

COPY package*.json ./
COPY packages/core/package*.json ./packages/core/
COPY packages/cli/package*.json ./packages/cli/
COPY packages/dashboard/package*.json ./packages/dashboard/
RUN npm ci --omit=dev

COPY --from=builder /app/packages/core/dist ./packages/core/dist
COPY --from=builder /app/packages/cli/dist ./packages/cli/dist
COPY --from=builder /app/packages/dashboard/dist ./packages/dashboard/dist

EXPOSE ${PORT:-4077}

CMD ["sh", "-c", "node packages/cli/dist/index.js portal --port ${PORT:-4077} --no-open"]
