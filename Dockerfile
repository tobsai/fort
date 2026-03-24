# Stage 1: Build
FROM node:20-bookworm AS builder

WORKDIR /app

# Copy everything (respecting .dockerignore)
COPY . .

# Install dependencies
RUN npm ci

# Build in order: core first (produces d.ts), then cli, then dashboard
RUN npm run build --workspace=packages/core && \
    npm run build --workspace=packages/cli && \
    npm run build --workspace=packages/dashboard

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
