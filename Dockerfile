# Stage 1: Build
FROM node:20-bookworm AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./
COPY packages/core/package*.json ./packages/core/
COPY packages/cli/package*.json ./packages/cli/
COPY packages/dashboard/package*.json ./packages/dashboard/

# Install all dependencies (including devDeps for build, native modules compiled here)
RUN npm ci

# Copy source
COPY . .

# Build packages in dependency order (core → cli → dashboard)
RUN npm run build --workspace=packages/core && \
    npm run build --workspace=packages/cli && \
    npm run build --workspace=packages/dashboard

# Stage 2: Runtime
FROM node:20-bookworm-slim AS runtime

WORKDIR /app

ENV NODE_ENV=production

# Copy built artifacts and node_modules from builder
COPY --from=builder /app/package*.json ./
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/packages/core/package*.json ./packages/core/
COPY --from=builder /app/packages/core/dist ./packages/core/dist
COPY --from=builder /app/packages/core/node_modules ./packages/core/node_modules
COPY --from=builder /app/packages/cli/package*.json ./packages/cli/
COPY --from=builder /app/packages/cli/dist ./packages/cli/dist
COPY --from=builder /app/packages/cli/node_modules ./packages/cli/node_modules
COPY --from=builder /app/packages/dashboard/package*.json ./packages/dashboard/
COPY --from=builder /app/packages/dashboard/dist ./packages/dashboard/dist

EXPOSE ${PORT:-4077}

CMD ["sh", "-c", "node packages/cli/dist/index.js portal --port ${PORT:-4077} --no-open"]
